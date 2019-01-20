// Package ansi implements a small subset of ANSI codes and functions in golang.
// It doesn't aim to be a full implementation of ANSI, and implements only
// what's required and used by vgrep.
//
// (c) 2017 Valentin Rothberg <valentinrothberg@gmail.com>
//
// Licensed under the terms of the GNU GPL License version 3.
package ansi

import (
	"fmt"
	"regexp"
	"strconv"
)

var ansiReg, _ = regexp.Compile("\x1B\\[[0-9;]*[ABCDEFGHJKSTfmnsulh]")

// COLOR is a numerical value representing ANSI colors.
type COLOR int

const (
	// DEFAULT ANSI color.
	DEFAULT COLOR = -1
	// BLACK ANSI color.
	BLACK COLOR = 0
	// RED ANSI color.
	RED COLOR = 1
	// GREEN ANSI color.
	GREEN COLOR = 2
	// YELLOW ANSI color.
	YELLOW COLOR = 3
	// BLUE ANSI color.
	BLUE COLOR = 4
	// MAGENTA ANSI color.
	MAGENTA COLOR = 5
	// CYAN ANSI color.
	CYAN COLOR = 6
	// GRAY ANSI color.
	GRAY COLOR = 7
)

// Color colors str with col in bright.
func Color(str string, col COLOR, bright bool) string {
	var code COLOR

	if col == DEFAULT {
		return str
	}

	if bright {
		code = 90 + col
	} else {
		code = 30 + col
	}

	return "\033[" + strconv.Itoa(int(code)) + "m" + str + "\033[0m"
}

// Bold returns bold str.
func Bold(str string) string {
	return "\033[1m" + str + "\033[0m"
}

// Underline returns underlined str.
func Underline(str string) string {
	return "\033[4m" + str + "\033[0m"
}

// RemoveANSI removes all ANSI codes from str.
func RemoveANSI(str string) string {
	return ansiReg.ReplaceAllString(str, "")
}

// ClearLine clears all characters from the cursor position to the end of the
// line (including the character at the cursor position).
func ClearLine() {
	fmt.Printf("\033[K")
}
