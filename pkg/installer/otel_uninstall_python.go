package installer

// detectProcessesFn is an overridable function variable for testing.
// In production it calls the real detectProcesses; in tests it can be stubbed.
var detectProcessesFn = func(filterTerm string, excludeTerms []string) []DetectedProcess {
	return detectProcesses(filterTerm, excludeTerms)
}

type pythonCleaner struct{}

func (pythonCleaner) Label() string { return "Python" }
func (pythonCleaner) DetectProcesses() []DetectedProcess {
	return detectProcessesFn("python", []string{"pip ", "setup.py", "/bin/dtwiz"})
}
