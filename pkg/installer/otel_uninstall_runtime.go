package installer

// RuntimeCleaner detects running OTel-instrumented processes for a specific runtime.
// Implement this interface and register an instance in runtimeCleaners to have the
// runtime's processes included in the uninstall preview and stop flow automatically.
type RuntimeCleaner interface {
	Label() string
	DetectProcesses() []DetectedProcess
}

var runtimeCleaners = []RuntimeCleaner{
	pythonCleaner{},
}
