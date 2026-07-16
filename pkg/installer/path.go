package installer

import "strings"

func pathEntries(pathValue string) []string {
	var entries []string
	for _, entry := range strings.Split(pathValue, ";") {
		if entry != "" {
			entries = append(entries, entry)
		}
	}
	return entries
}

// mergePathEntries appends entries from newPath not already in current (case-insensitive).
func mergePathEntries(current, newPath string) string {
	existing := make(map[string]bool)
	for _, e := range pathEntries(current) {
		existing[strings.ToLower(e)] = true
	}
	var toAdd []string
	for _, e := range pathEntries(newPath) {
		if !existing[strings.ToLower(e)] {
			toAdd = append(toAdd, e)
		}
	}
	if len(toAdd) == 0 {
		return current
	}
	if current == "" {
		return strings.Join(toAdd, ";")
	}
	return current + ";" + strings.Join(toAdd, ";")
}
