package tui

import (
	"strings"
	"testing"
)

func TestFormatValuesBlockOnePerLine(t *testing.T) {
	got := formatValuesBlock([]string{"a", "b", "c"}, "PRE ", "    ", 80)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d: %q", len(lines), got)
	}
	if !strings.HasPrefix(lines[0], "PRE a") {
		t.Fatalf("line0: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "    b") {
		t.Fatalf("line1: %q", lines[1])
	}
}

func TestWrapLineBreaksLong(t *testing.T) {
	s := strings.Repeat("word ", 40)
	got := wrapLine(s, 40)
	if !strings.Contains(got, "\n") {
		t.Fatalf("expected wrap: %q", got)
	}
}
