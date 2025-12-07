package main

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Renders an ASCII table from columns + rows.
func renderTable(columns []string, rows [][]string) string {
	if len(columns) == 0 {
		return "(No columns)\n"
	}

	// Calculate width of each column
	widths := make([]int, len(columns))
	for i, col := range columns {
		widths[i] = utf8.RuneCountInString(col)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i >= len(widths) {
				continue
			}
			l := utf8.RuneCountInString(cell)
			if l > widths[i] {
				widths[i] = l
			}
		}
	}

	// Helper to draw a border line
	makeBorder := func() string {
		var b strings.Builder
		b.WriteString("+")
		for _, w := range widths {
			b.WriteString(strings.Repeat("-", w+2))
			b.WriteString("+")
		}
		b.WriteString("\n")
		return b.String()
	}

	var sb strings.Builder

	// Top border
	sb.WriteString(makeBorder())

	// Header
	sb.WriteString("|")
	for i, col := range columns {
		sb.WriteString(" ")
		sb.WriteString(fmt.Sprintf("%-*s", widths[i], col))
		sb.WriteString(" |")
	}
	sb.WriteString("\n")

	// Separator
	sb.WriteString(makeBorder())

	// Rows
	for _, row := range rows {
		sb.WriteString("|")
		for i := range columns {
			var cell string
			if i < len(row) {
				cell = row[i]
			} else {
				cell = ""
			}
			sb.WriteString(" ")
			sb.WriteString(fmt.Sprintf("%-*s", widths[i], cell))
			sb.WriteString(" |")
		}
		sb.WriteString("\n")
	}

	// Bottom border
	sb.WriteString(makeBorder())

	return sb.String()
}

// Clips text horizontally based on offset and width (for scrolling).
func applyHorizontalScroll(s string, offset, width int) string {
	if width <= 0 {
		return s
	}
	if offset < 0 {
		offset = 0
	}

	lines := strings.Split(s, "\n")
	var out []string
	for _, line := range lines {
		runes := []rune(line)

		if offset >= len(runes) {
			out = append(out, "")
			continue
		}

		end := offset + width
		if end > len(runes) {
			end = len(runes)
		}

		out = append(out, string(runes[offset:end]))
	}

	return strings.Join(out, "\n")
}
