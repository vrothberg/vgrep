// Package colwriter allows to print text in a pretty columnized format.  It is
// similar to the "text/tabwriter" package but it allows to configure each
// column independently with different attributes such as padding or color.
// Notice, that this package's main purpose is to serve the needs of vgrep, but
// it may be generalized a bit more in the future.
//
// (c) 2017 Valentin Rothberg <valentinrothberg@gmail.com>
//
// Licensed under the terms of the GNU GPL License version 3.
package colwriter

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/vrothberg/vgrep/internal/ansi"
)

// PaddingFunc is a prototype for padding to make the padding configurable for
// each column.
type PaddingFunc func(string, int, string) string

// ColWriter only exposes function interfaces and no internal data.
type ColWriter struct {
	Size    []int          // size of each column
	Colors  []ansi.COLOR   // column-specific colors
	Padding []PaddingFunc  // left, right, none
	Headers bool           // text in first row will be underlined
	UseLess bool           // use less(1) instead of os.Stdout
	Trim    []bool         // trim space of column
	writer  *bufio.Writer  // in case we use less(1)
	pipe    io.WriteCloser // in case we use less(1)
	cmd     *exec.Cmd      // required for cmd.Wait() for less(1)
	opened  bool           // indicates if ColWriter is opened/closed
}

// New returns a default ColWriter of size numColumns.
func New(numColumns int) *ColWriter {
	cw := &ColWriter{
		Size:    make([]int, numColumns),
		Colors:  make([]ansi.COLOR, numColumns),
		Padding: make([]PaddingFunc, numColumns),
		Headers: false,
		UseLess: false,
		Trim:    make([]bool, numColumns),
		writer:  bufio.NewWriter(os.Stdout),
		opened:  false,
	}
	for i := 0; i < numColumns; i++ {
		cw.Size[i] = 0
		cw.Colors[i] = ansi.DEFAULT
		cw.Padding[i] = PadRight
	}
	cw.Padding[numColumns-1] = PadNone
	return cw
}

// ComputeSize computes the maximum string length for each column based on rows.
func (cw *ColWriter) ComputeSize(rows [][]string) {
	max := func(a, b int) int {
		if a > b {
			return a
		}
		return b
	}
	for _, row := range rows {
		for i, col := range row {
			cw.Size[i] = max(cw.Size[i], len(col))
		}
	}
}

// Open opens cw based on its configuration.
func (cw *ColWriter) Open() {
	if cw.UseLess {
		var err error
		cw.cmd = exec.Command("less", "-FRXS")
		cw.cmd.Stdout = os.Stdout
		cw.cmd.Stderr = os.Stderr
		cw.pipe, err = cw.cmd.StdinPipe()
		if err != nil {
			panic(fmt.Sprintf("Could not execute less:%s\n", err))
		}
		cw.writer = bufio.NewWriter(cw.pipe)
		err = cw.cmd.Start()
		if err != nil {
			panic(fmt.Sprintf("Could not execute less:%s\n", err))
		}
	}
	cw.opened = true
}

// Close closes cw based on its configuration.
func (cw *ColWriter) Close() {
	if !cw.opened {
		panic("Close() on unopened ColWriter\n")
	}
	cw.writer.Flush()
	if cw.UseLess {
		cw.pipe.Close()
		err := cw.cmd.Wait()
		if err != nil {
			panic(fmt.Sprintf("Could not execute less:%s\n", err))
		}
	}
	cw.opened = false
}

// WriteString writes str to cw's pipe.
func (cw *ColWriter) WriteString(str string) {
	if !cw.opened {
		panic("WriteString() on unopened ColWriter\n")
	}
	fmt.Fprintf(cw.writer, "%s", str)
}

// Write writes the data in rows in a pretty columnized format to cw's pipe.
func (cw *ColWriter) Write(rows [][]string) {
	if !cw.opened {
		panic("Write() on unopened ColWriter\n")
	}
	cw.ComputeSize(rows)
	if len(rows) == 0 {
		return
	}
	max := len(rows[0]) - 1

	if cw.Headers {
		for i, str := range rows[0] {
			out := cw.Padding[i](str, cw.Size[i], " ")
			out = ansi.Underline(out)
			out = ansi.Color(out, cw.Colors[i], true)
			if i < max {
				out += " "
			} else {
				out += "\n"
			}
			fmt.Fprintf(cw.writer, "%s", out)
		}
		rows = rows[1:]
	}
	for lineNum, row := range rows {
		var bright bool
		if lineNum%2 == 0 {
			bright = true
		} else {
			bright = false
		}
		for i, str := range row {
			if cw.Trim[i] {
				str = strings.TrimSpace(str)
			}
			out := cw.Padding[i](str, cw.Size[i], " ")
			out = ansi.Color(out, cw.Colors[i], bright)
			if i < max {
				out += " "
			} else {
				out += "\n"
			}
			fmt.Fprintf(cw.writer, "%s", out)
		}
	}
}

// PadLeft prefixes str with padding times pad.
func PadLeft(str string, padding int, pad string) string {
	padding -= len(str)
	if padding < 0 {
		return str
	}
	return strings.Repeat(pad, padding) + str
}

// PadRight suffixes s with padding times pad.
func PadRight(str string, padding int, pad string) string {
	padding -= len(str)
	if padding < 0 {
		return str
	}
	return str + strings.Repeat(pad, padding)
}

// PadNone adds no padding and returns str.
func PadNone(str string, padding int, pad string) string {
	return str
}
