package dotnet

import (
	"path/filepath"
	"strings"
)

// profilerCLSID identifies the OpenTelemetry .NET native profiler.
const profilerCLSID = "{918728DD-259F-4A6A-AC2B-B85E1B658318}"

// ActivationEnv returns the environment variables that switch the
// instrumentation on for a .NET process, for an installation in root with the
// layout of archive. It mirrors the upstream instrument.sh for modern .NET
// (CORECLR_*): the startup hook, the profiler and its native library. Exporter
// settings are not part of it. The variables are meant for the launched
// process only; nothing here is a credential.
func ActivationEnv(root string, archive Archive) map[string]string {
	env := map[string]string{
		"OTEL_DOTNET_AUTO_HOME":    root,
		"DOTNET_STARTUP_HOOKS":     filepath.Join(root, filepath.FromSlash(startupHookPath)),
		"CORECLR_ENABLE_PROFILING": "1",
		"CORECLR_PROFILER":         profilerCLSID,
	}
	native := func(dir string) string {
		return filepath.Join(root, dir, archive.NativeFile)
	}
	if len(archive.NativeDirs) > 0 && strings.HasPrefix(archive.NativeDirs[0], "win-") {
		// Windows sets both bitnesses; the runtime picks the one that fits.
		for _, dir := range archive.NativeDirs {
			switch dir {
			case "win-x64":
				env["CORECLR_PROFILER_PATH_64"] = native(dir)
			case "win-x86":
				env["CORECLR_PROFILER_PATH_32"] = native(dir)
			}
		}
		return env
	}
	if len(archive.NativeDirs) > 0 {
		env["CORECLR_PROFILER_PATH"] = native(archive.NativeDirs[0])
	}
	return env
}

// ActivationEnv returns the activation environment for the installation in
// Root on this machine.
func (in *Installer) ActivationEnv() (map[string]string, error) {
	a, err := in.Archive()
	if err != nil {
		return nil, err
	}
	return ActivationEnv(in.Root, a), nil
}
