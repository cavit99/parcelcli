package textutil

import "strings"

// CleanLines splits rendered page text into non-empty, trimmed lines.
func CleanLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			out = append(out, t)
		}
	}
	return out
}
