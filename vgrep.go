package main

// (c) 2015-2022 Valentin Rothberg <valentin@rothberg.email>
//
// Licensed under the terms of the GNU GPL License version 3.

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jessevdk/go-flags"
	jsoniter "github.com/json-iterator/go"
	"github.com/mattn/go-shellwords"
	"github.com/nightlyone/lockfile"
	"github.com/peterh/liner"
	"github.com/sirupsen/logrus"
	"github.com/vrothberg/vgrep/internal/ansi"
	"github.com/vrothberg/vgrep/internal/colwriter"
	"golang.org/x/term"
)

// Noticeably faster than the standard lib and battle tested.
var json = jsoniter.ConfigCompatibleWithStandardLibrary

// cliArgs passed to go-flags
type cliArgs struct {
	Debug         bool   `short:"d" long:"debug" description:"Verbose debug logging"`
	FilesOnly     bool   `short:"l" long:"files-with-matches" description:"Print matching files only"`
	Interactive   bool   `long:"interactive" description:"Enter interactive shell"`
	MemoryProfile string `long:"memory-profile" description:"Write a memory profile to the specified path"`
	NoGit         bool   `long:"no-git" description:"Use grep instead of git-grep"`
	NoRipgrep     bool   `long:"no-ripgrep" description:"Do not use ripgrep"`
	NoHeader      bool   `long:"no-header" description:"Do not print pretty headers"`
	NoLess        bool   `long:"no-less" description:"Use stdout instead of less"`
	Show          string `short:"s" long:"show" description:"Show specified matches or open shell" value-name:"SELECTORS"`
	Version       bool   `short:"v" long:"version" description:"Print version number"`
}

// vgrep stores state and the user-specified command-line arguments.
type vgrep struct {
	cliArgs
	matches [][]string
	workDir string
	lock    lockfile.Lockfile
	waiter  sync.WaitGroup
}

// the type of underlying grep program
const (
	BSDGrep = "BSD"
	GNUGrep = "GNU"
	GITGrep = "GIT"
	RIPGrep = "RIP"
)

var (
	// set in the Makefile
	version string

	commands = [...]string{"print", "show", "context", "tree", "delete",
		"keep", "refine", "files", "grep", "quit", "?"}
)

func main() {
	var (
		err error
		v   vgrep
	)

	// vgrep must not be terminated with SIGINT since less pager must be
	// terminated before vgrep. Ignore SIGINT.
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)
	go func() {
		for range sc {
		}
	}()

	// Unknown flags will be ignored and stored in args to further pass them
	// to (git) grep.
	parser := flags.NewParser(&v, flags.Default|flags.IgnoreUnknown)
	args, err := parser.ParseArgs(os.Args[1:])
	if err != nil {
		// Don't print the error to make sure the help message is printed once.
		// In other words, let's rely on the parser to print errors.
		os.Exit(1)
	}

	if v.MemoryProfile != "" {
		// Same value as the default in github.com/pkg/profile.
		runtime.MemProfileRate = 4096
		if rate := os.Getenv("MemProfileRate"); rate != "" {
			r, err := strconv.Atoi(rate)
			if err != nil {
				logrus.Errorf("%v", err)
				os.Exit(1)
			}
			runtime.MemProfileRate = r
		}
		defer func() { // Write the profile at the end
			f, err := os.Create(v.MemoryProfile)
			if err != nil {
				logrus.Errorf("Creating memory profile: %v", err)
				return
			}
			defer f.Close()
			runtime.GC() // get up-to-date GC statistics
			if err := pprof.WriteHeapProfile(f); err != nil {
				logrus.Errorf("Writing memory profile: %v", err)
				return
			}
		}()
	}

	if v.Version {
		fmt.Println(version)
		os.Exit(0)
	}

	if v.Debug {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.Debug("log level set to debug")
	}

	logrus.Debugf("passed args: %s", args)

	// Load the cache if there's no new query, otherwise execute a new one.
	err = v.makeLockFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating lock file: %v\n", err)
		os.Exit(1)
	}
	v.workDir, err = resolvedWorkdir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error resolving working directory: %v\n", err)
		os.Exit(1)
	}

	haveToRunCommand := v.Show != "" || v.Interactive

	// append additional args to the show command
	if v.Show != "" && len(args) > 0 {
		v.Show = fmt.Sprintf("%s %s", v.Show, strings.Join(args, ""))
	}

	if haveToRunCommand || len(args) == 0 {
		err = v.loadCache()
		if err != nil {
			if os.IsNotExist(err) {
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "error loading cache: %v\n", err)
			os.Exit(1)
		}

		if len(v.matches) == 0 {
			os.Exit(1)
		}

		if haveToRunCommand {
			v.commandParse()
		} else {
			v.commandPrintMatches([]int{})
		}
		v.waiter.Wait()
		os.Exit(0)
	}

	v.waiter.Add(1)
	v.grep(args)
	v.cacheWrite() // this runs in the background

	if len(v.matches) == 0 {
		v.waiter.Wait()
		os.Exit(1)
	}

	// Last resort, print all matches.
	v.commandPrintMatches([]int{})
	v.waiter.Wait()
}

// runCommand executes the program specified in args and returns the stdout as
// a line-separated []string.
func (v *vgrep) runCommand(args []string, env string) ([]string, error) {
	var cmd *exec.Cmd
	var sout, serr bytes.Buffer

	logrus.Debugf("runCommand(args=%s, env=%s)", args, env)

	cmd = exec.Command(args[0], args[1:]...)
	cmd.Stdout = &sout
	cmd.Stderr = &serr
	cmd.Env = []string{env}

	err := cmd.Run()
	if err != nil {
		logrus.Debugf("error running command: %v", err)
		errStr := err.Error()
		if errStr == "exit status 1" {
			logrus.Debug("ignoring error (no matches found)")
			err = nil
		} else {
			spl := strings.Split(serr.String(), "\n")
			err = fmt.Errorf("%s [%s]", spl[0], args[0])
		}
	}

	slice := strings.Split(sout.String(), "\n")
	return slice[:len(slice)-1], err
}

// insideGitTree returns true if the current working directory is inside a git
// tree.
func (v *vgrep) insideGitTree() bool {
	cmd := []string{"git", "rev-parse", "--is-inside-work-tree"}
	out, _ := v.runCommand(cmd, "")
	inside := false

	if len(out) > 0 && out[0] == "true" {
		inside = true
	}

	logrus.Debugf("insideGitTree() -> %v", inside)
	return inside
}

// ripgrepInstalled returns true if ripgrep is installed
func (v *vgrep) ripgrepInstalled() bool {
	out, err := exec.LookPath("rg")
	if err != nil {
		logrus.Debug("error checking if ripgrep is installed")
	}
	installed := false

	if len(out) > 0 {
		installed = true
	}

	logrus.Debugf("ripgrepInstalled() -> %v", installed)
	return installed
}

func (v *vgrep) getGrepType() string {
	out, _ := v.runCommand([]string{"grep", "--version"}, "")
	if len(out) == 0 {
		return ""
	}
	versionString := out[0]
	// versionString = "grep (BSD grep) 2.5.1-FreeBSD"
	// versionString = "grep (BSD grep, GNU compatible) 2.6.0-FreeBSD"
	versionRegex := regexp.MustCompile(`\(([[:alpha:]]+) grep`)
	// versionRegex matches to ["(BSD grep)", "BSD"], return "BSD"
	submatch := versionRegex.FindStringSubmatch(versionString)
	if len(submatch) < 2 {
		return ""
	}
	return submatch[1]
}

// isVscode checks if the terminal is running inside of vscode.
func isVscode() bool {
	return os.Getenv("TERM_PROGRAM") == "vscode"
}

// isGoland checks if the terminal is running inside of goland or possible other JetBrains IDEs.
func isGoland() bool {
	return strings.Contains(os.Getenv("TERMINAL_EMULATOR"), "JetBrains")
}

// grep (git) greps with the specified args and stores the results in v.matches.
func (v *vgrep) grep(args []string) {
	var cmd []string
	var env string
	var greptype string // can have values , GIT, RIP, GNU, BSD

	if v.ripgrepInstalled() && !v.NoRipgrep {
		cmd = []string{
			"rg", "-0", "--colors=path:none", "--colors=line:none",
			"--color=always", "--no-heading", "--line-number",
			"--with-filename",
		}
		cmd = append(cmd, args...)
		greptype = RIPGrep
		config := os.Getenv("RIPGREP_CONFIG_PATH")
		if len(config) != 0 {
			env = "RIPGREP_CONFIG_PATH=" + config
		}
	} else if v.insideGitTree() && !v.NoGit {
		env = "HOME="
		cmd = []string{
			"git", "-c", "color.grep.match=red bold",
			"grep", "-z", "-In", "--color=auto",
		}
		cmd = append(cmd, args...)
		greptype = GITGrep
	} else {
		env = "GREP_COLORS='ms=01;31:mc=:sl=:cx=:fn=:ln=:se=:bn='"
		cmd = []string{"grep", "-ZHInr", "--color=always"}
		cmd = append(cmd, args...)
		greptype = v.getGrepType()
	}
	output, err := v.runCommand(cmd, env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "searching symbols failed: %v\n", err)
		os.Exit(1)
	}
	v.matches = make([][]string, len(output))
	i := 0
	for _, m := range output {
		file, line, content, err := v.splitMatch(m, greptype)
		if err != nil {
			logrus.Debugf("skipping line %q (parse error: %v)", m, err)
			continue
		}
		v.matches[i] = make([]string, 4)
		v.matches[i][0] = strconv.Itoa(i)
		v.matches[i][1] = file
		v.matches[i][2] = line
		v.matches[i][3] = content
		i++
	}

	logrus.Debugf("found %d matches", len(v.matches))
}

// splitMatch splits match into its file, line and content.  The format of
// match varies depending if it has been produced by grep or git-grep.
func (v *vgrep) splitMatch(match string, greptype string) (file, line, content string, err error) {
	if greptype == RIPGrep {
		// remove default color ansi escape codes from ripgrep's output
		match = strings.Replace(match, "\x1b[0m", "", 4)
	}
	var separator []byte
	switch greptype {
	case BSDGrep:
		separator = []byte(":")
	case GITGrep, GNUGrep, RIPGrep:
		separator = []byte{0}
	}
	switch greptype {
	case BSDGrep, GITGrep:
		spl := bytes.SplitN([]byte(match), separator, 3)
		if len(spl) < 3 {
			err = fmt.Errorf("expected %d but split into %d items (%v)", 3, len(spl), separator)
			return
		}
		file, line, content = string(spl[0]), string(spl[1]), string(spl[2])
	case GNUGrep, RIPGrep:
		spl := bytes.SplitN([]byte(match), separator, 2)
		if len(spl) < 2 {
			err = fmt.Errorf("expected %d but split into %d items (%v)", 2, len(spl), separator)
			return
		}
		splline := bytes.SplitN(spl[1], []byte(":"), 2)
		if len(splline) != 2 {
			// Fall back to "-" which is used when displaying
			// context lines.
			splline = bytes.SplitN(spl[1], []byte("-"), 2)
		}
		if len(splline) == 2 {
			file, line, content = string(spl[0]), string(splline[0]), string(splline[1])
			return
		}
		err = fmt.Errorf("unexpected input")
		return
	default:
		err = fmt.Errorf("unknown grep type %q", greptype)
		return
	}
	return
}

// getEditor returns the EDITOR environment variable (default="vim").
func (v *vgrep) getEditor() string {
	editor := os.Getenv("EDITOR")
	if len(editor) == 0 {
		editor = "vim"
	}
	return editor
}

// getEditorLineFlag returns the EDITORLINEFLAG environment variable (default="+").
func (v *vgrep) getEditorLineFlag() string {
	editor := os.Getenv("EDITORLINEFLAG")
	if len(editor) == 0 {
		editor = "+"
	}
	return editor
}

// Create the lock file to guard against concurrent processes
func (v *vgrep) makeLockFile() error {

	var err error
	var lockdir string

	if runtime.GOOS == "windows" {
		lockdir = filepath.Join(os.Getenv("LOCALAPPDATA"), "vgrep")
	} else {
		lockdir = filepath.Join(os.Getenv("HOME"), ".local/share/vgrep")
	}
	exists := true

	if _, err := os.Stat(lockdir); err != nil {
		if os.IsNotExist(err) {
			exists = false
		} else {
			return err
		}
	}

	if !exists {
		if err := os.MkdirAll(lockdir, 0700); err != nil {
			return err
		}
	}
	v.lock, err = lockfile.New(filepath.Join(lockdir, "cache-lock"))
	return err
}

// Try to acquire the lock file for the cache
func (v *vgrep) acquireLock() error {
	for err := v.lock.TryLock(); err != nil; err = v.lock.TryLock() {
		// If the lock is busy, wait for it, otherwise error out
		if err != lockfile.ErrBusy {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}

	return nil
}

// cachePath returns the path to the user-specific vgrep cache.
func (v *vgrep) cachePath() (string, error) {

	var cache string

	if runtime.GOOS == "windows" {
		cache = filepath.Join(os.Getenv("LOCALAPPDATA"), "vgrep-cache/")
	} else {
		cache = filepath.Join(os.Getenv("HOME"), ".cache/")
	}
	exists := true

	if _, err := os.Stat(cache); err != nil {
		if os.IsNotExist(err) {
			exists = false
		} else {
			return "", err
		}
	}

	if !exists {
		if err := os.Mkdir(cache, 0700); err != nil {
			return "", err
		}
	}

	return filepath.Join(cache, "vgrep-go"), nil
}

// cacheWrite uses cacheWriterHelper to write to the user-specific vgrep cache.
func (v *vgrep) cacheWrite() {
	go func() {
		defer v.waiter.Done()
		if err := v.cacheWriterHelper(); err != nil {
			logrus.Debugf("error writing cache: %v", err)
		}
	}()
}

// cacheWriterHelper writes to the user-specific vgrep cache.
func (v *vgrep) cacheWriterHelper() error {
	logrus.Debug("cacheWriterHelper(): start")
	defer logrus.Debug("cacheWriterHelper(): end")

	workDir, err := resolvedWorkdir()
	if err != nil {
		return err
	}

	cache, err := v.cachePath()
	if err != nil {
		return fmt.Errorf("error getting cache path: %v", err)
	}

	if err := v.acquireLock(); err != nil {
		return fmt.Errorf("error acquiring lock file: %v", err)
	}
	defer func() {
		if err := v.lock.Unlock(); err != nil {
			panic(fmt.Sprintf("Error releasing lock file: %v", err))
		}
	}()

	file, err := os.OpenFile(cache, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	out := append(v.matches, []string{workDir})

	b, err := json.Marshal(out)
	if err != nil {
		return err
	}

	if _, err := file.Write(b); err != nil {
		return err
	}

	if err := file.Sync(); err != nil {
		return err
	}

	return file.Close()
}

// resolvedWorkdir returns the path to current working directory (fully evaluated in case it's a symlink).
func resolvedWorkdir() (string, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("error getting working dir: %v", err)
	}
	return filepath.EvalSymlinks(workDir)
}

// loadCache loads the user-specific vgrep cache.
func (v *vgrep) loadCache() error {
	logrus.Debug("loadCache(): start")
	defer logrus.Debug("loadCache(): end")

	cache, err := v.cachePath()
	if err != nil {
		return fmt.Errorf("error getting cache path: %v", err)
	}

	if err := v.acquireLock(); err != nil {
		return err
	}
	defer func() {
		if err := v.lock.Unlock(); err != nil {
			panic(fmt.Sprintf("Error releasing lock file: %v", err))
		}
	}()

	file, err := ioutil.ReadFile(cache)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(file, &v.matches); err != nil {
		// if there's an error unmarshalling it, remove the cache file
		os.Remove(cache)
		return err
	}

	if length := len(v.matches); length > 0 {
		v.workDir = v.matches[length-1][0]
		v.matches = v.matches[:len(v.matches)-1]
	}

	return nil
}

// sortKeys returns a sorted []string of m's keys.
func sortKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// shellCompleter is a completion function for the interactive shell's prompt.
func shellCompleter(line string) (c []string) {
	args, err := shellwords.Parse(line)
	if err != nil {
		return
	}

	if len(args) < 2 {
		for _, cmd := range commands {
			if strings.HasPrefix(cmd, line) {
				c = append(c, cmd)
			}
		}
		return
	}

	arg := args[len(args)-1]
	switch args[0] {
	case "g", "grep":
		if len(args) < 3 {
			return
		}
		dir := filepath.Dir(arg)
		base := filepath.Base(arg)
		if arg[len(arg)-1] == '/' {
			dir = arg
			base = ""
		}
		files, err := ioutil.ReadDir(dir)
		if err != nil {
			return
		}
		for _, f := range files {
			if strings.HasPrefix(f.Name(), base) {
				comp := line + f.Name()[len(base):]
				c = append(c, comp)
			}
		}
	default:
		return
	}
	return
}

// commandParse starts and dispatches user-specific vgrep commands.  If the
// user input matches a vgrep selector commandShow will be executed. It will
// prompt the user for commands if we're running in interactive mode.
func (v *vgrep) commandParse() {
	line := liner.NewLiner()
	defer line.Close()
	line.SetCtrlCAborts(true)
	line.SetCompleter(shellCompleter)

	nextInput := func() string {
		usrInp, err := line.Prompt("Enter a vgrep command: ")
		if err != nil {
			// Either we hit an error or EOF (ctrl+d)
			line.Close()
			fmt.Fprintf(os.Stderr, "error parsing user input: %v\n", err)
			os.Exit(1)
		}
		line.AppendHistory(usrInp)
		logrus.Debugf("user input: %q", usrInp)
		return usrInp
	}

	input := v.Show
	quit := false
	if input != "" {
		quit = v.dispatchCommand(input)
	}
	for !quit && v.Interactive {
		input = nextInput()
		quit = v.dispatchCommand(input)
	}
}

// v.checkIndices is a helper function to fill indices in case it's an empty
// array and does some range checks otherwise.
func (v *vgrep) checkIndices(indices []int) ([]int, error) {
	if len(indices) == 0 {
		indices = make([]int, len(v.matches))
		for i := range v.matches {
			indices[i] = i
		}
		return indices, nil
	}
	for _, idx := range indices {
		if idx < 0 || idx > len(v.matches)-1 {
			return nil, fmt.Errorf("index %d out of range (%d, %d)", idx, 0, len(v.matches)-1)
		}
	}
	return indices, nil
}

// dispatchCommand parses and dispatches the specified vgrep command in input.
// The return value indicates if dispatching of commands should be stopped.
func (v *vgrep) dispatchCommand(input string) bool {
	logrus.Debugf("dispatchCommand(%s)", input)
	if len(input) == 0 {
		return v.commandPrintHelp()
	}

	// Most commands accept only a set of selectors on indices as arguments.
	// Before we try to parse arguments as selectors, deal with the few
	// commands that take random strings as arguments.

	cmdArray := strings.SplitN(input, " ", 2)

	if cmdArray[0] == "r" || cmdArray[0] == "refine" {
		if len(cmdArray) != 2 {
			fmt.Println("refine expects a regexp argument")
			return false
		}
		return v.commandRefine(cmdArray[1])
	}

	if cmdArray[0] == "g" || cmdArray[0] == "grep" {
		if len(cmdArray) < 2 {
			fmt.Println("grep expects at least a pattern")
			return false
		}
		return v.commandGrep(cmdArray[1])
	}

	// normalize selector-only inputs (e.g., "1,2,3,5-10") to the show cmd
	selectorRegexp := `(\s*all|[\d , -]+){0,1}`
	numRgx := regexp.MustCompile(`^` + selectorRegexp + `$`)
	if numRgx.MatchString(input) {
		input = "s " + input
	}

	cmdRgx := regexp.MustCompile(`^([a-z?]{1,})([\d]+){0,1}` + selectorRegexp + `$`)
	if !cmdRgx.MatchString(input) {
		fmt.Printf("%q doesn't match format %q\n", input, "command[context lines] [selectors]")
		return false
	}

	var command, selectors string
	var context int

	cmdArray = cmdRgx.FindStringSubmatch(input)
	command = cmdArray[1]
	selectors = cmdArray[3]
	context = -1

	if len(cmdArray[2]) > 0 {
		var err error
		context, err = strconv.Atoi(cmdArray[2])
		if err != nil {
			fmt.Printf("cannot convert specified context lines %q: %v", cmdArray[2], err)
			return false
		}
	}

	indices, err := v.parseSelectors(selectors)
	if err != nil {
		fmt.Println(err)
		return false
	}

	switch command {
	case "?":
		return v.commandPrintHelp()

	case "c", "context":
		if context == -1 {
			context = 5
		}
		return v.commandPrintContextLines(indices, context)

	case "d", "delete":
		if len(indices) == 0 {
			fmt.Println("delete requires specified selectors")
			return false
		}
		return v.commandDelete(indices)

	case "k", "keep":
		if len(indices) == 0 {
			fmt.Println("keep requires specified selectors")
			return false
		}
		return v.commandKeep(indices)

	case "f", "files":
		return v.commandListFiles(indices)

	case "p", "print":
		return v.commandPrintMatches(indices)

	case "q", "quit":
		return true

	case "s", "show":
		if len(indices) == 0 {
			fmt.Println("show requires specified selectors")
		} else {
			for _, idx := range indices {
				v.commandShow(idx)
			}
		}
		return false

	case "t", "tree":
		return v.commandListTree(indices)

	default:
		fmt.Printf("unsupported command %q\n", command)
		return false
	}
}

// commandPrintHelp prints the help/usage message for vgrep commands on stdout.
func (v *vgrep) commandPrintHelp() bool {
	// Join command names, but write first letter in bold.
	commandList := ansi.Bold(string(commands[0][0])) + commands[0][1:]
	for _, c := range commands[1:] {
		commandList += ", " + ansi.Bold(string(c[0])) + c[1:]
	}

	fmt.Printf("vgrep command help: command[context lines] [selectors]\n")
	fmt.Printf("         selectors: '3' (single), '1,2,6' (multi), '1-8' (range), 'all'\n")
	fmt.Printf("          commands: %s\n", commandList)
	return false
}

// commandPrintMatches prints all matches specified in indices using less(1) or
// stdout in case v.NoLess is specified. If indices is empty all matches
// are printed.
func (v *vgrep) commandPrintMatches(indices []int) bool {
	var toPrint [][]string
	var err error

	indices, err = v.checkIndices(indices)
	if err != nil {
		fmt.Printf("%v\n", err)
		return false
	}

	if v.FilesOnly {
		visited := make(map[string]bool)
		for _, i := range indices {
			file := v.matches[i][1]
			if _, exists := visited[file]; exists {
				continue
			}
			visited[file] = true
			fmt.Println(file)
		}
		return false
	}

	if !v.NoHeader {
		toPrint = append(toPrint, []string{"Index", "File", "Line", "Content"})
	}

	inIDE := isVscode() || isGoland()
	for _, i := range indices {
		switch {
		case inIDE:
			// If we're running inside an IDE's terminal, append
			// the line to the file path, so we can quick jump to
			// the specific location.  Note that dancing around
			// with the indexes below is intentional - ugly but
			// fast.
			toPrint = append(toPrint, []string{v.matches[i][0], v.matches[i][1] + ":" + v.matches[i][2], v.matches[i][2], v.matches[i][3]})
		default:
			toPrint = append(toPrint, v.matches[i])
		}
	}

	useLess := !v.NoLess
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		useLess = false
	}

	cw := colwriter.New(4)
	cw.Headers = true && !v.NoHeader
	cw.Colors = []ansi.COLOR{ansi.MAGENTA, ansi.BLUE, ansi.GREEN, ansi.DEFAULT}
	cw.Padding = []colwriter.PaddingFunc{colwriter.PadLeft, colwriter.PadRight, colwriter.PadLeft, colwriter.PadNone}
	cw.UseLess = useLess
	cw.Trim = []bool{false, false, false, true}

	cw.Open()
	cw.Write(toPrint)
	cw.Close()

	return false
}

// fileLocation returns the path and line number of the matches at the
// specified index.
func (v *vgrep) fileLocation(index int) (string, int, error) {
	p := v.matches[index][1]
	// If it's not an absolute path, join it with the workDir.
	// This allows for using vgrep from another working dir
	// than where the initial query was done.
	if !path.IsAbs(p) {
		p = path.Join(v.workDir, p)
	}

	line, err := strconv.Atoi(v.matches[index][2])
	if err != nil {
		return "", 0, err
	}
	return p, line, nil
}

// getContextLines return numLines context lines before and after the match at
// the specified index including the matched line itself as []string.
func (v *vgrep) getContextLines(index int, numLines int) [][]string {
	var contextLines [][]string

	path, line, err := v.fileLocation(index)
	if err != nil {
		logrus.Warn(err.Error())
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		logrus.Warnf("error opening file %q: %v", path, err)
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	counter := 0
	for scanner.Scan() {
		counter++
		if counter == line {
			newContext := []string{strconv.Itoa(counter), v.matches[index][3]}
			contextLines = append(contextLines, newContext)
		} else if (counter >= line-numLines) && (counter <= line+numLines) {
			newContext := []string{strconv.Itoa(counter), scanner.Text()}
			contextLines = append(contextLines, newContext)
		}
		if counter > line+numLines {
			break
		}
	}

	return contextLines
}

// commandPrintContextLines prints at most numLines context lines before and
// after each match specified in indices.
func (v *vgrep) commandPrintContextLines(indices []int, numLines int) bool {
	var err error

	logrus.Debugf("commandPrintContextLines(indices=[..], numlines=%d)", numLines)
	indices, err = v.checkIndices(indices)
	if err != nil {
		fmt.Printf("%v\n", err)
		return false
	}

	cw := colwriter.New(2)
	cw.Colors = []ansi.COLOR{ansi.MAGENTA, ansi.DEFAULT}
	cw.Padding = []colwriter.PaddingFunc{colwriter.PadLeft, colwriter.PadNone}
	cw.UseLess = !v.NoLess
	cw.Open()

	for _, idx := range indices {
		toPrint := v.getContextLines(idx, numLines)
		if toPrint == nil {
			continue
		}

		sep := fmt.Sprintf("%s %s %s ",
			ansi.Color("---", ansi.MAGENTA, false),
			ansi.Color(strconv.Itoa(idx), ansi.MAGENTA, false),
			ansi.Color(v.matches[idx][1], ansi.BLUE, false))
		for i := 0; i < 80-len(ansi.RemoveANSI(sep)); i++ {
			sep += ansi.Color("---", ansi.MAGENTA, false)
		}
		sep += "\n"
		cw.WriteString(sep)
		cw.Write(toPrint)
	}

	cw.Close()
	return false
}

// commandDelete deletes all indices from v.matches. It updates all other indices.
func (v *vgrep) commandDelete(indices []int) bool {
	var err error

	indices, err = v.checkIndices(indices)
	if err != nil {
		fmt.Printf("%v\n", err)
		return false
	}

	for offset, idx := range indices {
		logrus.Debugf("deleting index %d", idx)
		for i := idx + 1; i < len(v.matches); i++ {
			v.matches[i][0] = strconv.Itoa(i - 1)
		}
		index := idx - offset
		v.matches = append(v.matches[:index], v.matches[index+1:]...)
	}

	return false
}

// commandKeep does the opposite of commandDelete and keeps only provided
// indices in v.matches.
func (v *vgrep) commandKeep(indices []int) bool {
	var toDelete []int
	var last int

	for _, idx := range indices {
		for i := last; i < idx; i++ {
			toDelete = append(toDelete, i)
		}
		last = idx + 1
	}
	for i := last; i < len(v.matches); i++ {
		toDelete = append(toDelete, i)
	}
	return v.commandDelete(toDelete)
}

// commandRefine deletes all results that do not match the provided pattern
// (regexp) from the list.
func (v *vgrep) commandRefine(expr string) bool {
	pattern, err := regexp.Compile(expr)
	if err != nil {
		fmt.Printf("failed to compile '%s' as a regexp\n", expr)
		return false
	}

	var toDelete []int
	for offset, grepMatch := range v.matches {
		if !pattern.Match([]byte(ansi.RemoveANSI(grepMatch[3]))) {
			toDelete = append(toDelete, offset)
		}
	}
	return v.commandDelete(toDelete)
}

func (v *vgrep) commandGrep(expr string) bool {
	args, err := shellwords.Parse(expr)
	if err != nil {
		fmt.Printf("failed to parse '%s': %s (missing quotes?)\n", expr, err)
		return false
	}

	logrus.Debugf("new grep from interactive shell, passed args: %s", args)
	v.waiter.Add(1)
	v.grep(args)
	v.cacheWrite()

	if len(v.matches) > 0 {
		v.commandPrintMatches([]int{})
	}

	v.waiter.Wait()
	return false
}

// commandShow opens the environment's editor at v.matches[index].
func (v *vgrep) commandShow(index int) bool {
	if _, err := v.checkIndices([]int{index}); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return false
	}

	editor := v.getEditor()
	path, line, err := v.fileLocation(index)
	if err != nil {
		logrus.Warn(err.Error())
		return false
	}

	lFlag := fmt.Sprintf("%s%d", v.getEditorLineFlag(), line)

	logrus.Debugf("opening index %d via: %s %s %s", index, editor, path, lFlag)

	var cmd *exec.Cmd
	_, file := filepath.Split(editor)
	switch file {
	case "emacs", "emacsclient":
		// emacs expects the line before the file
		cmd = exec.Command(editor, lFlag, path)
	default:
		// default to adding the line after the file
		cmd = exec.Command(editor, path, lFlag)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("couldn't open index %d: %v\n", index, err)
	}

	return false
}

// commandListTree prints statistics about how many matches occur in which
// directories in the search.
func (v *vgrep) commandListTree(indices []int) bool {
	var err error

	indices, err = v.checkIndices(indices)
	if err != nil {
		fmt.Printf("%v\n", err)
		return false
	}

	count := make(map[string]int)
	for _, idx := range indices {
		m := v.matches[idx]
		split := strings.Split(m[1], "/")
		if len(split) == 1 {
			count["."]++
			continue
		}
		for i := range split {
			path := strings.Join(split[:i], "/")
			count[path]++
		}
	}

	var toPrint [][]string
	if !v.NoHeader {
		toPrint = append(toPrint, []string{"Matches", "Directory"})
	}

	for _, k := range sortKeys(count) {
		num := strconv.Itoa(count[k])
		toPrint = append(toPrint, []string{num, k})
	}

	cw := colwriter.New(2)
	cw.Headers = true && !v.NoHeader
	cw.Colors = []ansi.COLOR{ansi.MAGENTA, ansi.GREEN}
	cw.Padding = []colwriter.PaddingFunc{colwriter.PadLeft, colwriter.PadNone}
	cw.UseLess = !v.NoLess

	cw.Open()
	cw.Write(toPrint)
	cw.Close()

	return false
}

// commandListFiles prints statistics about how many matches occur in which
// files in the search.
func (v *vgrep) commandListFiles(indices []int) bool {
	var err error

	if indices, err = v.checkIndices(indices); err != nil {
		fmt.Printf("%v\n", err)
		return false
	}

	count := make(map[string]int)
	for _, idx := range indices {
		m := v.matches[idx]
		count[m[1]]++
	}

	var toPrint [][]string
	if !v.NoHeader {
		toPrint = append(toPrint, []string{"Matches", "File"})
	}

	for _, k := range sortKeys(count) {
		num := strconv.Itoa(count[k])
		toPrint = append(toPrint, []string{num, k})
	}

	cw := colwriter.New(2)
	cw.Headers = true && !v.NoHeader
	cw.Colors = []ansi.COLOR{ansi.MAGENTA, ansi.GREEN}
	cw.Padding = []colwriter.PaddingFunc{colwriter.PadLeft, colwriter.PadNone}
	cw.UseLess = !v.NoLess

	cw.Open()
	cw.Write(toPrint)
	cw.Close()

	return false
}

// parseSelectors parses input for vgrep selectors and returns the corresponding
// indices as a sorted []int.
func (v *vgrep) parseSelectors(input string) ([]int, error) {
	indices := []int{}

	if strings.TrimSpace(input) == "all" {
		for i := 0; i < len(v.matches); i++ {
			indices = append(indices, i)
		}
		return indices, nil
	}

	selRgx := regexp.MustCompile("([^,]+)")

	toInt := func(idx string) (int, error) {
		idx = strings.TrimSpace(idx)
		num, err := strconv.Atoi(idx)
		if err != nil {
			return -1, fmt.Errorf("non-numeric selector %q", idx)
		}
		return num, nil
	}

	addIndex := func(idx int) {
		for _, x := range indices {
			if x == idx {
				return
			}
		}
		indices = append(indices, idx)
	}

	for _, sel := range selRgx.FindAllString(input, -1) {
		rng := strings.Split(sel, "-")
		if len(rng) == 1 {
			num, err := toInt(rng[0])
			if err != nil {
				return nil, err
			}
			addIndex(num)
		} else if len(rng) == 2 {
			from, err := toInt(rng[0])
			if err != nil {
				return nil, err
			}
			to, err := toInt(rng[1])
			if err != nil {
				return nil, err
			}
			if from > to {
				cpy := from
				from = to
				to = cpy
			}
			for i := from; i <= to; i++ {
				addIndex(i)
			}
		} else {
			return nil, fmt.Errorf("invalid range format %q", sel)
		}
	}

	sort.Ints(indices)
	return indices, nil
}
