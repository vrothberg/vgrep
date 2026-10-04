package ansi

import (
	"strings"
	"testing"
)

func TestColorEnabled(t *testing.T) {
	enable()
	result := Color("hello", RED, true)
	if !strings.Contains(result, "\033[") {
		t.Errorf("expected ANSI escape codes, got %q", result)
	}
}

func TestColorDisabled(t *testing.T) {
	Disable()
	defer enable()
	result := Color("hello", RED, true)
	if result != "hello" {
		t.Errorf("expected plain string, got %q", result)
	}
}

func TestColorDefault(t *testing.T) {
	enable()
	result := Color("hello", DEFAULT, true)
	if result != "hello" {
		t.Errorf("expected plain string for DEFAULT color, got %q", result)
	}
}

func TestBoldEnabled(t *testing.T) {
	enable()
	result := Bold("hello")
	if !strings.Contains(result, "\033[1m") {
		t.Errorf("expected bold ANSI code, got %q", result)
	}
}

func TestBoldDisabled(t *testing.T) {
	Disable()
	defer enable()
	result := Bold("hello")
	if result != "hello" {
		t.Errorf("expected plain string, got %q", result)
	}
}

func TestUnderlineEnabled(t *testing.T) {
	enable()
	result := Underline("hello")
	if !strings.Contains(result, "\033[4m") {
		t.Errorf("expected underline ANSI code, got %q", result)
	}
}

func TestUnderlineDisabled(t *testing.T) {
	Disable()
	defer enable()
	result := Underline("hello")
	if result != "hello" {
		t.Errorf("expected plain string, got %q", result)
	}
}
