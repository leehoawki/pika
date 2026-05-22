package chart

import (
	"strings"
	"testing"
)

func TestColorize(t *testing.T) {
	result := Colorize("hello", Red)
	if !strings.Contains(result, "\033[31m") {
		t.Error("expected red ANSI code")
	}
	if !strings.Contains(result, "hello") {
		t.Error("expected original text")
	}
	if !strings.Contains(result, "\033[0m") {
		t.Error("expected reset code")
	}
}

func TestBold(t *testing.T) {
	result := Bold("test")
	if !strings.Contains(result, "\033[1m") {
		t.Error("expected bold escape code")
	}
	if !strings.Contains(result, "\033[0m") {
		t.Error("expected reset code")
	}
}
