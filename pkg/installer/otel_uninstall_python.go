package installer

type pythonCleaner struct{}

func (pythonCleaner) Label() string { return "Python" }
func (pythonCleaner) DetectProcesses() []DetectedProcess {
	return detectInstrumentedPythonProcesses()
}
