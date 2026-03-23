package analyzer

// detectServices checks for common application runtimes and databases.
func detectServices() []string {
	// runtimes: detected if the binary is installed (not necessarily running)
	runtimes := []struct {
		name string
		bin  string
	}{
		{"java", "java"},
		{"node", "node"},
		{"python", "python3"},
		{"go", "go"},
	}

	// daemons: detected only if a process is actively running
	daemons := []struct {
		name string
		bin  string
	}{
		{"nginx", "nginx"},
		{"postgres", "psql"},
		{"mysql", "mysql"},
		{"redis", "redis-cli"},
		{"mongodb", "mongod"},
	}

	var found []string
	for _, r := range runtimes {
		ok, _ := runCmd("which", r.bin)
		if ok {
			found = append(found, r.name)
		}
	}
	for _, d := range daemons {
		ok, _ := runCmd("pgrep", "-x", d.bin)
		if ok {
			found = append(found, d.name)
		}
	}
	return found
}
