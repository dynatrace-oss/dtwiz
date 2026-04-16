//go:build windows

package analyzer

import "strings"

// detectServices checks for common application runtimes and databases on Windows.
func detectServices() []string {
	// runtimes: detected if the binary is on PATH (where.exe exits 0 when found)
	runtimes := []struct {
		name string
		bins []string // first match wins; order matters (python before python3)
	}{
		{"java", []string{"java"}},
		{"node", []string{"node"}},
		{"python", []string{"python", "python3"}},
		{"go", []string{"go"}},
	}

	// daemons: detected only if a matching process is currently running
	daemons := []struct {
		name    string
		process string // substring matched against Get-Process names
	}{
		{"nginx", "nginx"},
		{"postgres", "postgres"},
		{"mysql", "mysqld"},
		{"redis", "redis-server"},
		{"mongodb", "mongod"},
	}

	var found []string
	for _, r := range runtimes {
		for _, bin := range r.bins {
			ok, _ := runCmd("where.exe", bin)
			if ok {
				found = append(found, r.name)
				break
			}
		}
	}

	for _, d := range daemons {
		ok, out := runCmd("powershell", "-NoProfile", "-Command",
			"Get-Process -Name '"+d.process+"' -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty Id")
		if ok && strings.TrimSpace(out) != "" {
			found = append(found, d.name)
		}
	}
	return found
}
