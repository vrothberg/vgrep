package main

// (c) 2015-2019 Valentin Rothberg <valentin@rothberg.email>
//
// Licensed under the terms of the GNU GPL License version 3.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/nightlyone/lockfile"
	"github.com/sirupsen/logrus"
	"github.com/vrothberg/vgrep/internal/ansi"
	"github.com/vrothberg/vgrep/internal/colwriter"
)

// cliArgs passed to go-flags
type cliArgs struct {
	Debug       bool   `short:"d" long:"debug" description:"Verbose debug logging"`
	Interactive bool   `long:"interactive" description:"Enter interactive shell"`
	NoGit       bool   `long:"no-git" description:"Use grep instead of git-grep"`
	NoRipgrep   bool   `long:"no-ripgrep" description:"Do not use ripgrep"`
	NoHeader    bool   `long:"no-header" description:"Do not print pretty headers"`
	NoLess      bool   `long:"no-less" description:"Use stdout instead of less"`
	Show        string `short:"s" long:"show" description:"Show specified matches or open shell" value-name:"SELECTORS"`
	Version     bool   `short:"v" long:"version" description:"Print version number"`
}

// vgrep stores state and the user-specified command-line arguments.
type vgrep struct {
	cliArgs
	matches [][]string
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

// set in the Makefile
var version string

func main() {
	var (
		err error
		v   vgrep
	)

	// Unknown flags will be ignored and stored in args to further pass them
	// to (git) grep.
	parser := flags.NewParser(&v, flags.Default|flags.IgnoreUnknown)
	args, err := parser.ParseArgs(os.Args[1:])
	if err != nil {
		os.Exit(1)
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
	} else {
		v.waiter.Add(1)
		v.grep(args)
		v.cacheWrite() // this runs in the background
	}

	if len(v.matches) == 0 {
		os.Exit(0) // nothing to do anymore
	}

	// Execute the specified command and/or jump directly into the
	// interactive shell.
	if haveToRunCommand {
		v.commandParse()
		v.waiter.Wait()
		os.Exit(0)
	}

	// Last resort, print all matches.
	if len(v.matches) > 0 {
		v.commandPrintMatches([]int{})
	}

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

func (v *vgrep) getGrepType() (grepType string) {
	out, _ := v.runCommand([]string{"grep", "--version"}, "")
	versionString := out[0]
	// versionString = "grep (BSD grep) 2.5.1-FreeBSD"
	versionRegex := regexp.MustCompile(`\(([[:alpha:]]+) grep\)`)
	// versionRegex matches to ["(BSD grep)", "BSD"], return "BSD"
	grepType = versionRegex.FindStringSubmatch(versionString)[1]
	return
}

// isVscode checks if the terminal is running inside of vscode.
func isVscode() bool {
	return os.Getenv("TERM_PROGRAM") == "vscode"
}

// grep (git) greps with the specified args and stores the results in v.matches.
func (v *vgrep) grep(args []string) {
	var cmd []string
	var usegit bool
	var env string
	var greptype string // can have values , GIT, RIP, GNU, BSD

	useripgrep := v.ripgrepInstalled() && !v.NoRipgrep
	usegit = v.insideGitTree() && !v.NoGit
	if useripgrep {
		cmd = []string{
			"rg", "-0", "--colors=path:none", "--colors=line:none",
			"--color=always", "--no-heading", "--line-number",
		}
		cmd = append(cmd, args...)
		cmd = append(cmd, ".")
		greptype = RIPGrep
	} else if usegit {
		env = "HOME="
		cmd = []string{
			"git", "-c", "color.grep.match=red bold",
			"grep", "-z", "-In", "--color=always",
		}
		cmd = append(cmd, args...)
		greptype = GITGrep
	} else {
		env = "GREP_COLORS='ms=01;31:mc=:sl=:cx=:fn=:ln=:se=:bn='"
		cmd = []string{"grep", "-ZIn", "--color=always"}
		cmd = append(cmd, args...)
		cmd = append(cmd, "-r", ".")
		greptype = v.getGrepType()
	}
	output, err := v.runCommand(cmd, env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "searching symbols failed: %v\n", err)
		os.Exit(1)
	}
	v.matches = make([][]string, len(output))
	for i, m := range output {
		file, line, content := v.splitMatch(m, greptype)
		v.matches[i] = make([]string, 4)
		v.matches[i][0] = strconv.Itoa(i)
		v.matches[i][1] = file
		v.matches[i][2] = line
		v.matches[i][3] = content
	}

	logrus.Debugf("found %d matches", len(v.matches))
}

// splitMatch splits match into its file, line and content.  The format of
// match varies depending if it has been produced by grep or git-grep.
func (v *vgrep) splitMatch(match string, greptype string) (file, line, content string) {
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
	spl := bytes.SplitN([]byte(match), separator, 3)
	switch greptype {
	case BSDGrep, GITGrep:
		file, line, content = string(spl[0]), string(spl[1]), string(spl[2])
	case GNUGrep, RIPGrep:
		splline := bytes.SplitN(spl[1], []byte(":"), 2)
		file, line, content = string(spl[0]), string(splline[0]), string(splline[1])
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
		oldWorkDir := v.matches[length-1][0]
		v.matches = v.matches[:len(v.matches)-1]
		workDir, err := resolvedWorkdir()
		if err != nil {
			return err
		}
		if workDir != oldWorkDir {
			return fmt.Errorf("please cd into %s to use old cache", oldWorkDir)
		}
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

// commandParse starts and dispatches user-specific vgrep commands.  If the
// user input matches a vgrep selector commandShow will be executed. It will
// prompt the user for commands if we're running in interactive mode.
func (v *vgrep) commandParse() {
	nextInput := func() string {
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Print(ansi.Bold("Enter a vgrep command: "))
		if !scanner.Scan() {
			// Either we hit an error or EOF (ctrl+d)
			if err := scanner.Err(); err != nil {
				fmt.Fprintf(os.Stderr, "error parsing user input: %v", err)
			}
			fmt.Println()
			os.Exit(1)
		}
		usrInp := scanner.Text()
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

	// normalize selector-only inputs (e.g., "1,2,3,5-10") to the show cmd
	numRgx := regexp.MustCompile(`^([\d]+){0,1}([\d , -]+){0,1}$`)
	if numRgx.MatchString(input) {
		input = "s " + input
	}

	cmdRgx := regexp.MustCompile(`^([a-z?]{1,})([\d]+){0,1}([\d , -]+){0,1}$`)
	if !cmdRgx.MatchString(input) {
		fmt.Printf("%q doesn't match format %q\n", input, "command[context lines] [selectors]")
		return false
	}

	var command, selectors string
	var context int

	cmdArray := cmdRgx.FindStringSubmatch(input)
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

	if command == "?" {
		return v.commandPrintHelp()
	}

	if command == "c" || command == "context" {
		if context == -1 {
			context = 5
		}
		return v.commandPrintContextLines(indices, context)
	}

	if command == "d" || command == "delete" {
		if len(indices) == 0 {
			fmt.Println("delete requires specified selectors")
			return false
		}
		return v.commandDelete(indices)
	}

	if command == "f" || command == "files" {
		return v.commandListFiles(indices)
	}

	if command == "p" || command == "print" {
		return v.commandPrintMatches(indices)
	}

	if command == "q" || command == "quit" {
		return true
	}

	if command == "s" || command == "show" {
		if len(indices) == 0 {
			fmt.Println("show requires specified selectors")
		} else {
			for _, idx := range indices {
				v.commandShow(idx)
			}
		}
		return false
	}

	if command == "t" || command == "tree" {
		return v.commandListTree(indices)
	}

	fmt.Printf("unsupported command %q\n", command)
	return false
}

// commandPrintHelp prints the help/usage message for vgrep commands on stdout.
func (v *vgrep) commandPrintHelp() bool {
	fmt.Printf("vgrep command help: command[context lines] [selectors]\n")
	fmt.Printf("         selectors: '3' (single), '1,2,6' (multi), '1-8' (range)\n")
	fmt.Printf("          commands: %srint, %show, %sontext, %sree, %selete, %siles, %suit, %s\n",
		ansi.Bold("p"), ansi.Bold("s"), ansi.Bold("c"), ansi.Bold("t"),
		ansi.Bold("d"), ansi.Bold("f"), ansi.Bold("q"), ansi.Bold("?"))
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

	if !v.NoHeader {
		toPrint = append(toPrint, []string{"Index", "File", "Line", "Content"})
	}

	isVscode := isVscode()
	for _, i := range indices {
		if isVscode {
			// If we're running inside a vscode terminal, append the line to the
			// file path, so we can quick jump to the specific location.  Note
			// that dancing around with the indexes below is intentional - ugly
			// but fast.
			toPrint = append(toPrint, []string{v.matches[i][0], v.matches[i][1] + ":" + v.matches[i][2], v.matches[i][2], v.matches[i][3]})
		} else {
			toPrint = append(toPrint, v.matches[i])
		}
	}

	cw := colwriter.New(4)
	cw.Headers = true && !v.NoHeader
	cw.Colors = []ansi.COLOR{ansi.MAGENTA, ansi.BLUE, ansi.GREEN, ansi.DEFAULT}
	cw.Padding = []colwriter.PaddingFunc{colwriter.PadLeft, colwriter.PadRight, colwriter.PadLeft, colwriter.PadNone}
	cw.UseLess = !v.NoLess
	cw.Trim = []bool{false, false, false, true}

	cw.Open()
	cw.Write(toPrint)
	cw.Close()

	return false
}

// getContextLines return numLines context lines before and after the match at
// the specified index including the matched line itself as []string.
func (v *vgrep) getContextLines(index int, numLines int) [][]string {
	var contextLines [][]string

	path := v.matches[index][1]
	line, err := strconv.Atoi(v.matches[index][2])
	if err != nil {
		logrus.Warnf("error converting %q: %v", path, err)
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
		index := idx + offset
		v.matches = append(v.matches[:index], v.matches[index+1:]...)
	}

	return false
}

// commandShow opens the environment's editor at v.matches[index].
func (v *vgrep) commandShow(index int) bool {
	if _, err := v.checkIndices([]int{index}); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return false
	}

	editor := v.getEditor()
	file := v.matches[index][1]
	lFlag := v.getEditorLineFlag() + v.matches[index][2]

	logrus.Debugf("opening index %d via: %s %s %s", index, editor, file, lFlag)
	cmd := exec.Command(editor, file, lFlag)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("douldn't open index %d: %v\n", index, err)
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
