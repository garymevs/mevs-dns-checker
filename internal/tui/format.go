package tui

import (
	"strings"
	"unicode/utf8"
)

// formatValuesBlock renders record values one per line.
// The first line uses prefix; following lines use contPrefix.
func formatValuesBlock(values []string, prefix, contPrefix string, width int) string {
	if len(values) == 0 {
		return prefix + "(empty)"
	}
	var b strings.Builder
	for i, v := range values {
		p := prefix
		if i > 0 {
			p = contPrefix
		}
		b.WriteString(wrapLine(p+v, width))
		if i < len(values)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func wrapLine(s string, width int) string {
	if width <= 0 || utf8.RuneCountInString(s) <= width {
		return s
	}
	var b strings.Builder
	runes := []rune(s)
	for len(runes) > 0 {
		n := width
		if n > len(runes) {
			n = len(runes)
		}
		// Prefer breaking at a space when possible.
		if n < len(runes) {
			if cut := lastSpace(runes[:n]); cut > width/4 {
				n = cut
			}
		}
		b.WriteString(string(runes[:n]))
		runes = runes[n:]
		if len(runes) > 0 {
			b.WriteByte('\n')
			// continuation indent for wrapped overflow
			indent := "  "
			b.WriteString(indent)
			if len(runes) > 0 && runes[0] == ' ' {
				runes = runes[1:]
			}
			// reduce available width for subsequent wraps
			if width > len(indent) {
				width = width // keep using original for simplicity on further loops via n
			}
		}
	}
	return b.String()
}

func lastSpace(runes []rune) int {
	for i := len(runes) - 1; i >= 0; i-- {
		if runes[i] == ' ' {
			return i
		}
	}
	return len(runes)
}

func contentWidth(termWidth int) int {
	if termWidth <= 4 {
		return 40
	}
	return termWidth - 2
}

