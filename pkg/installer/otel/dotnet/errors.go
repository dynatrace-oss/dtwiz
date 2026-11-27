package dotnet

import "fmt"

// Stage names the part of the installation that failed.
type Stage string

// Installation stages, as named in error messages.
const (
	StageReleaseLookup    Stage = "release lookup"
	StageDownload         Stage = "download"
	StageVerification     Stage = "verification"
	StageExtraction       Stage = "extraction"
	StageLayoutValidation Stage = "layout validation"
	StageInstall          Stage = "install"
)

// InstallError is a failed installation. Stage says what failed; Hint, when
// set, tells the user how to fix a cause that is theirs to fix (network,
// permissions, disk space).
type InstallError struct {
	Stage Stage
	Err   error
	Hint  string
}

func (e *InstallError) Error() string {
	msg := fmt.Sprintf("OpenTelemetry .NET instrumentation %s failed: %v", e.Stage, e.Err)
	if e.Hint != "" {
		msg += " (" + e.Hint + ")"
	}
	return msg
}

func (e *InstallError) Unwrap() error { return e.Err }

func stageErr(stage Stage, err error) *InstallError {
	return &InstallError{Stage: stage, Err: err}
}
