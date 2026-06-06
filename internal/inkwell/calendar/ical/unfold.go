package ical

import (
	"bufio"
	"io"
	"strings"
)

// unfold reads RFC 5545 content lines from r, joining continuation lines
// (lines starting with a space or tab) back to the previous line. A
// scanner error (read failure or oversized line) is surfaced so callers
// don't get a silently truncated parse.
func unfold(r io.Reader) ([]string, error) {
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
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}
