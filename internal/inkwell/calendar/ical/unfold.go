package ical

import (
	"bufio"
	"io"
	"strings"
)

// unfold reads RFC 5545 content lines from r, joining continuation lines
// (lines starting with a space or tab) back to the previous line.
func unfold(r io.Reader) []string {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			continue
		}
		if (line[0] == ' ' || line[0] == '\t') && len(lines) > 0 {
			lines[len(lines)-1] += line[1:]
		} else {
			lines = append(lines, line)
		}
	}
	return lines
}
