package main

// (c) 2015-2017 Valentin Rothberg <valentinrothberg@gmail.com>
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
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jessevdk/go-flags"
	"github.com/sirupsen/logrus"
	"github.com/vrothberg/vgrep/ansi"
	"github.com/vrothberg/vgrep/colwriter"
)

// cliArgs passed to go-flags
type cliArgs struct {
	Debug       bool   `short:"d" long:"debug" description:"Verbose debug logging"`
	Interactive bool   `short:"i" long:"interactive" description:"Enter interactive shell"`
	NoGit       bool   `long:"no-git" description:"Use grep instead of git-grep"`
	NoHeader    bool   `long:"no-header" description:"Do not print pretty headers"`
	NoLess      bool   `long:"no-less" description:"Use stdout instead of less"`
	Show        string `short:"s" long:"show" description:"Show specified matches or open shell" value-name:"SELECTORS"`
	Version     bool   `short:"v" long:"version" description:"Print version number"`
}

// global variables
var (
	Options cliArgs
	Matches [][]string
	Log     = logrus.New()
	version string // set in the Makefile
)

func main() {
	var err error

	// Unkown flags will be ignored and stored in args to further pass them
	// to (git) grep.
	parser := flags.NewParser(&Options, flags.Default|flags.IgnoreUnknown)
	args, err := parser.ParseArgs(os.Args[1:])
	if err != nil {
		os.Exit(1)
	}

	if Options.Version {
		fmt.Println(version)
		os.Exit(0)
	}

	if Options.Debug {
		Log.SetLevel(logrus.DebugLevel)
		Log.Debug("log level set to debug")
	}

	Log.Debugf("passed args: %s", args)

	// Load the cache if there's no new querry, otherwise execute a new one.
	if len(args) == 0 {
		err = loadCache()
		if err != nil {
			fmt.Fprintf(os.Stderr, "No cache: %v\n", err)
			os.Exit(1)
		}
	} else {
		grep(args)
		cacheWrite() // this runs in the background
	}

	if len(Matches) == 0 {
		os.Exit(0) // nothing to do anymore
	}

	// Execute the specified command and/or jump directly into the
	// interactive sheel.
	if Options.Show != "" || Options.Interactive {
		commandParse(Options.Show)
		os.Exit(0)
	}

	// Last resort, print all matches.
	if len(Matches) > 0 {
		commandPrintMatches([]int{})
	}
}

// runCommand executes the program specified in args and returns the stdout as
// a line-separated []string.
func runCommand(args []string, env string) ([]string, error) {
	var cmd *exec.Cmd
	var sout, serr bytes.Buffer

	Log.Debugf("runCommand(args=%s, env=%s)", args, env)

	cmd = exec.Command(args[0], args[1:]...)
	cmd.Stdout = &sout
	cmd.Stderr = &serr
	cmd.Env = []string{env}

	err := cmd.Run()
	if err != nil {
		Log.Debugf("error running command: %s", err)
		errStr := err.Error()
		if errStr == "exit status 1" {
			Log.Debug("ignoring error (no matches found)")
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
func insideGitTree() bool {
	cmd := []string{"git", "rev-parse", "--is-inside-work-tree"}
	out, _ := runCommand(cmd, "")
	inside := false

	if len(out) > 0 && out[0] == "true" {
		inside = true
	}

	Log.Debugf("insideGitTree() -> %v", inside)
	return inside
}

// grep (git) greps with the specified args and stores the results in Matches.
func grep(args []string) {
	var cmd []string
	var usegit bool
	var env string

	Log.Debugf("grep(args=%s)", args)

	usegit = insideGitTree() && !Options.NoGit

	if usegit {
		env = "HOME="
		cmd = []string{
			"git", "-c", "color.grep.match=red bold",
			"grep", "-z", "-In", "--color=always",
		}
		cmd = append(cmd, args...)
	} else {
		env = "GREP_COLORS='ms=01;31:mc=:sl=:cx=:fn=:ln=:se=:bn='"
		cmd = []string{"grep", "-ZIn", "--color=always"}
		cmd = append(cmd, args...)
		cmd = append(cmd, "-r", ".")
	}

	output, err := runCommand(cmd, env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Searching symbols failed: %v\n", err)
		os.Exit(1)
	}
	Matches = make([][]string, len(output))

	for i, m := range output {
		file, line, content := splitMatch(m, usegit)
		Matches[i] = make([]string, 4)
		Matches[i][0] = strconv.Itoa(i)
		Matches[i][1] = file
		Matches[i][2] = line
		Matches[i][3] = content
	}

	Log.Debugf("Found %d matches", len(Matches))
}

// splitMatch splits match into its file, line and content.  The format of
// match varies depending if it has been produced by grep or git-grep.
func splitMatch(match string, gitgrep bool) (file, line, content string) {
	spl := bytes.SplitN([]byte(match), []byte{0}, 3)
	if gitgrep {
		return string(spl[0]), string(spl[1]), string(spl[2])
	}
	// the 2nd separator of grep is ":"
	splline := bytes.SplitN(spl[1], []byte(":"), 2)
	return string(spl[0]), string(splline[0]), string(splline[1])
}

// getEditor returns the EDITOR environment variable (default="vim").
func getEditor() string {
	editor := os.Getenv("EDITOR")
	if len(editor) == 0 {
		editor = "vim"
	}
	return editor
}

// getEditorLineFlag returns the EDITORLINEFLAG environment variable (default="+").
func getEditorLineFlag() string {
	editor := os.Getenv("EDITORLINEFLAG")
	if len(editor) == 0 {
		editor = "+"
	}
	return editor
}

// cachePath returns the path to the user-specific vgrep cache.
func cachePath() string {
	return os.Getenv("HOME") + "/.cache/vgrep-go"
}

// cacheWrite uses cacheWriterHelper to write to the user-specific vgrep cache.
func cacheWrite() {
	go func() {
		if err := cacheWriterHelper(); err != nil {
			Log.Debugf("error writing cache: %v", err)
		}
	}()
}

// cacheWriterHelper writes to the user-specific vgrep cache.
func cacheWriterHelper() error {
	Log.Debug("cacheWriterHelper(): start")
	defer Log.Debug("cacheWriterHelper(): end")

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting working dir: %v", err)
	}

	file, err := os.OpenFile(cachePath(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}

	out := append(Matches, []string{workDir})

	b, err := json.Marshal(out)
	if err != nil {
		return err
	}

	if _, err := file.Write(b); err != nil {
		return err
	}

	return file.Close()
}

// loadsCache loads the user-specific vgrep cache.
func loadCache() error {
	Log.Debug("loadCache(): start")
	defer Log.Debug("loadCache(): end")

	file, err := ioutil.ReadFile(cachePath())
	if err != nil {
		return err
	}
	if err := json.Unmarshal(file, &Matches); err != nil {
		os.Remove(cachePath())
		return err
	}

	if length := len(Matches); length > 0 {
		oldWorkDir := Matches[length-1][0]
		Matches = Matches[:len(Matches)-1]
		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("error getting working dir: %v", err)
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

// commandParse starts and dispatches user-specific vgrep commands.  If input
// matches a vgrep selector commanShow will be executed.  It will promt the
// user for commands we're running in interactive mode.
func commandParse(input string) {
	Log.Debugf("commandParse(input=%s)", input)

	if indices, err := parseSelectors(input); err == nil && len(indices) > 0 {
		input = "s "
		for _, i := range indices {
			input = fmt.Sprintf("%s%d,", input, i)
		}
	}

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
		Log.Debugf("User input: %s", usrInp)
		return usrInp
	}

	for {
		quit := dispatchCommand(input)
		if quit || !Options.Interactive {
			return
		}
		input = nextInput()
	}
}

// checkIndices is a helper function to fill indices in case it's an empty
// array and does some range checks otherwise.
func checkIndices(indices []int) ([]int, error) {
	if len(indices) == 0 {
		indices = make([]int, len(Matches))
		for i := range Matches {
			indices[i] = i
		}
		return indices, nil
	}
	for _, idx := range indices {
		if idx < 0 || idx > len(Matches)-1 {
			return nil, fmt.Errorf("Index %d out of range (%d, %d)", idx, 0, len(Matches)-1)
		}
	}
	return indices, nil
}

// dispatchCommand parses and dispatches the specified vgrep command in input.
// The return value indicates if dispatching of commands should be stopped.
func dispatchCommand(input string) bool {
	if len(input) == 0 {
		return false
	}
	cmdRgx := regexp.MustCompile("^([a-z?]{1,})([\\d]+){0,1}([\\d , -]+){0,1}$")

	if !cmdRgx.MatchString(input) {
		fmt.Printf("\"%s\" doesn't match format \"command[context lines] [selectors]\"\n", input)
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
			fmt.Printf("Cannot convert specified context lines '%d': %v", context, err)
		}
	}

	indices, err := parseSelectors(selectors)
	if err != nil {
		fmt.Println(err)
		return false
	}

	if command == "?" {
		return commandPrintHelp()
	}

	if command == "c" || command == "context" {
		if context == -1 {
			context = 5
		}
		return commandPrintContextLines(indices, context)
	}

	if command == "d" || command == "delete" {
		if len(indices) == 0 {
			fmt.Println("Delete requires specified selectors")
			return false
		}
		return commandDelete(indices)
	}

	if command == "f" || command == "files" {
		return commandListFiles(indices)
	}

	if command == "p" || command == "print" {
		return commandPrintMatches(indices)
	}

	if command == "q" || command == "quit" {
		return true
	}

	if command == "s" || command == "show" {
		if len(indices) == 0 {
			fmt.Println("Show requires specified selectors")
			return false
		}
		for _, idx := range indices {
			commandShow(idx)
		}
	}

	if command == "t" || command == "tree" {
		return commandListTree(indices)
	}

	fmt.Printf("Unsupported command \"%s\"\n", command)
	return false
}

// commandPrintHelp prints the help/usage message for vgrep commands on stdout.
func commandPrintHelp() bool {
	fmt.Printf("vgrep command help: command[context lines] [selectors]\n")
	fmt.Printf("         selectors: '3' (single), '1,2,6' (multi), '1-8' (range)\n")
	fmt.Printf("          commands: %srint, %show, %sontext, %sree, %selete, %siles, %suit, %s\n",
		ansi.Bold("p"), ansi.Bold("s"), ansi.Bold("c"), ansi.Bold("t"),
		ansi.Bold("d"), ansi.Bold("f"), ansi.Bold("q"), ansi.Bold("?"))
	return false
}

// commandPrintMatches prints all matches specified in indices using less(1) or
// stdout in case Options.NoLess is specified. If indices is empty all matches
// are printed.
func commandPrintMatches(indices []int) bool {
	var toPrint [][]string
	var err error

	indices, err = checkIndices(indices)
	if err != nil {
		fmt.Printf("%v\n", err)
		return false
	}

	if !Options.NoHeader {
		toPrint = append(toPrint, []string{"Index", "File", "Line", "Content"})
	}

	for _, i := range indices {
		toPrint = append(toPrint, Matches[i])
	}

	cw := colwriter.New(4)
	cw.Headers = true && !Options.NoHeader
	cw.Colors = []ansi.COLOR{ansi.YELLOW, ansi.BLUE, ansi.GREEN, ansi.DEFAULT}
	cw.Padding = []colwriter.PaddingFunc{colwriter.PadLeft, colwriter.PadRight, colwriter.PadLeft, colwriter.PadNone}
	cw.UseLess = !Options.NoLess
	cw.Trim = []bool{false, false, false, true}

	cw.Open()
	cw.Write(toPrint)
	cw.Close()

	return false
}

// getContextLines return numLines context lines before and after the match at
// the specified index including the matched line itself as []string.
func getContextLines(index int, numLines int) [][]string {
	var contextLines [][]string

	path := Matches[index][1]
	line, err := strconv.Atoi(Matches[index][2])
	if err != nil {
		Log.Warnf("Error converting '%s': %v", path, err)
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		Log.Warnf("Error opening file '%s': %v", file, err)
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	counter := 0
	for scanner.Scan() {
		counter++
		if counter == line {
			newContext := []string{strconv.Itoa(counter), Matches[index][3]}
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
func commandPrintContextLines(indices []int, numLines int) bool {
	var err error

	Log.Debugf("commandPrintContextLines(indices=[..], numlines=%d)", numLines)
	indices, err = checkIndices(indices)
	if err != nil {
		fmt.Printf("%v\n", err)
		return false
	}

	cw := colwriter.New(2)
	cw.Colors = []ansi.COLOR{ansi.YELLOW, ansi.DEFAULT}
	cw.Padding = []colwriter.PaddingFunc{colwriter.PadLeft, colwriter.PadNone}
	cw.UseLess = !Options.NoLess
	cw.Open()

	for _, idx := range indices {
		toPrint := getContextLines(idx, numLines)
		if toPrint == nil {
			continue
		}

		sep := fmt.Sprintf("%s %s %s ",
			ansi.Color("---", ansi.YELLOW, false),
			ansi.Color(strconv.Itoa(idx), ansi.YELLOW, false),
			ansi.Color(Matches[idx][1], ansi.BLUE, false))
		for i := 0; i < 80-len(ansi.RemoveANSI(sep)); i++ {
			sep += ansi.Color("---", ansi.YELLOW, false)
		}
		sep += "\n"
		cw.WriteString(sep)
		cw.Write(toPrint)
	}

	cw.Close()
	return false
}

// commandDelete deletes all indices from Matches. It updates all other indices.
func commandDelete(indices []int) bool {
	var err error

	indices, err = checkIndices(indices)
	if err != nil {
		fmt.Printf("%v\n", err)
		return false
	}

	for offset, idx := range indices {
		Log.Debugf("Deleting index '%d'", idx)
		for i := idx + 1; i < len(Matches); i++ {
			Matches[i][0] = strconv.Itoa(i - 1)
		}
		index := idx + offset
		Matches = append(Matches[:index], Matches[index+1:]...)
	}

	return false
}

// commandShow opens the environment's editor at Matches[index].
func commandShow(index int) bool {
	if _, err := checkIndices([]int{index}); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return false
	}

	editor := getEditor()
	file := Matches[index][1]
	lFlag := getEditorLineFlag() + Matches[index][2]

	Log.Debugf("opening index %d via: %s %s %s", index, editor, file, lFlag)
	cmd := exec.Command(editor, file, lFlag)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Couldn't open match: %v\n", err)
	}

	return false
}

// commandListTree prints statistics about how many matches occur in which
// directories in the search.
func commandListTree(indices []int) bool {
	var err error

	indices, err = checkIndices(indices)
	if err != nil {
		fmt.Printf("%v\n", err)
		return false
	}

	count := make(map[string]int)
	for _, idx := range indices {
		m := Matches[idx]
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
	if !Options.NoHeader {
		toPrint = append(toPrint, []string{"Matches", "Directory"})
	}

	for _, k := range sortKeys(count) {
		num := strconv.Itoa(count[k])
		toPrint = append(toPrint, []string{num, k})
	}

	cw := colwriter.New(2)
	cw.Headers = true && !Options.NoHeader
	cw.Colors = []ansi.COLOR{ansi.YELLOW, ansi.GREEN}
	cw.Padding = []colwriter.PaddingFunc{colwriter.PadLeft, colwriter.PadNone}
	cw.UseLess = !Options.NoLess

	cw.Open()
	cw.Write(toPrint)
	cw.Close()

	return false
}

// commandListFiles prints statistics about how many matches occur in which
// files in the search.
func commandListFiles(indices []int) bool {
	var err error

	if indices, err = checkIndices(indices); err != nil {
		fmt.Printf("%v\n", err)
		return false
	}

	count := make(map[string]int)
	for _, idx := range indices {
		m := Matches[idx]
		count[m[1]]++
	}

	var toPrint [][]string
	if !Options.NoHeader {
		toPrint = append(toPrint, []string{"Matches", "File"})
	}

	for _, k := range sortKeys(count) {
		num := strconv.Itoa(count[k])
		toPrint = append(toPrint, []string{num, k})
	}

	cw := colwriter.New(2)
	cw.Headers = true && !Options.NoHeader
	cw.Colors = []ansi.COLOR{ansi.YELLOW, ansi.GREEN}
	cw.Padding = []colwriter.PaddingFunc{colwriter.PadLeft, colwriter.PadNone}
	cw.UseLess = !Options.NoLess

	cw.Open()
	cw.Write(toPrint)
	cw.Close()

	return false
}

// parseSelectors parses input for vgrep selectors and returns the corresonding
// indices as a sorted []int.
func parseSelectors(input string) ([]int, error) {
	indices := []int{}
	selRgx := regexp.MustCompile("([^,]+)")

	toInt := func(idx string) (int, error) {
		idx = strings.TrimSpace(idx)
		num, err := strconv.Atoi(idx)
		if err != nil {
			return -1, fmt.Errorf("Non-numeric selector '%s'", idx)
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
			return nil, fmt.Errorf("Invalid range format '%s'", sel)
		}
	}

	sort.Ints(indices)
	return indices, nil
}
