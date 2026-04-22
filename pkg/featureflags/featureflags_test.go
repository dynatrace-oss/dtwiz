package featureflags

import (
	"testing"

	"github.com/spf13/pflag"
)

func TestIsEnabled_DefaultIsReturned(t *testing.T) {
	t.Setenv("DTWIZ_ALL_RUNTIMES", "")
	if IsEnabled(AllRuntimes) {
		t.Error("expected AllRuntimes to be disabled by default")
	}
}

func TestIsEnabled_EnvTrue(t *testing.T) {
	t.Setenv("DTWIZ_ALL_RUNTIMES", "true")
	if !IsEnabled(AllRuntimes) {
		t.Error("expected AllRuntimes to be enabled with env=true")
	}
}

func TestIsEnabled_EnvOne(t *testing.T) {
	t.Setenv("DTWIZ_ALL_RUNTIMES", "1")
	if !IsEnabled(AllRuntimes) {
		t.Error("expected AllRuntimes to be enabled with env=1")
	}
}

func TestIsEnabled_EnvFalse(t *testing.T) {
	t.Setenv("DTWIZ_ALL_RUNTIMES", "false")
	if IsEnabled(AllRuntimes) {
		t.Error("expected AllRuntimes to be disabled with env=false")
	}
}

func TestIsEnabled_EnvCaseInsensitive(t *testing.T) {
	t.Setenv("DTWIZ_ALL_RUNTIMES", "TRUE")
	if !IsEnabled(AllRuntimes) {
		t.Error("expected AllRuntimes enabled with env=TRUE (case-insensitive match)")
	}
}

func TestIsEnabled_EnvWhitespaceNotTrimmed(t *testing.T) {
	t.Setenv("DTWIZ_ALL_RUNTIMES", " true")
	if IsEnabled(AllRuntimes) {
		t.Error("expected AllRuntimes disabled with env=\" true\" (whitespace is not trimmed)")
	}
}

func TestIsEnabled_EnvZeroIsFalsy(t *testing.T) {
	t.Setenv("DTWIZ_ALL_RUNTIMES", "0")
	if IsEnabled(AllRuntimes) {
		t.Error("expected AllRuntimes disabled with env=0 (only \"1\" is truthy, not \"0\")")
	}
}

func TestIsEnabled_UnknownFlag(t *testing.T) {
	unknown := Flag(999)
	if IsEnabled(unknown) {
		t.Error("expected unknown flag to return false")
	}
}

func TestSetCLIOverrideForTest_OverridesAndRestores(t *testing.T) {
	t.Setenv("DTWIZ_ALL_RUNTIMES", "")

	if IsEnabled(AllRuntimes) {
		t.Fatal("precondition: expected AllRuntimes disabled")
	}

	t.Run("inner", func(t *testing.T) {
		SetCLIOverrideForTest(t, AllRuntimes, true)
		if !IsEnabled(AllRuntimes) {
			t.Error("expected AllRuntimes enabled via SetCLIOverrideForTest")
		}
	})

	// After inner test cleanup, override should be removed
	if IsEnabled(AllRuntimes) {
		t.Error("expected AllRuntimes disabled after inner test cleanup")
	}
}

func TestList_DefaultSource(t *testing.T) {
	t.Setenv("DTWIZ_ALL_RUNTIMES", "")

	flags := List()
	if len(flags) == 0 {
		t.Fatal("expected at least one flag in list")
	}
	f := flags[0]
	if f.Name != "all-runtimes" {
		t.Errorf("expected name all-runtimes, got %s", f.Name)
	}
	if f.EnvVar != "DTWIZ_ALL_RUNTIMES" {
		t.Errorf("expected env var DTWIZ_ALL_RUNTIMES, got %s", f.EnvVar)
	}
	if f.Enabled {
		t.Error("expected flag disabled")
	}
	if f.Source != "default" {
		t.Errorf("expected source default, got %s", f.Source)
	}
}

func TestList_EnvSource(t *testing.T) {
	t.Setenv("DTWIZ_ALL_RUNTIMES", "true")

	flags := List()
	f := flags[0]
	if !f.Enabled {
		t.Error("expected flag enabled")
	}
	if f.Source != "env" {
		t.Errorf("expected source env, got %s", f.Source)
	}
}

func TestList_CLISource(t *testing.T) {
	SetCLIOverrideForTest(t, AllRuntimes, true)
	t.Setenv("DTWIZ_ALL_RUNTIMES", "")

	flags := List()
	f := flags[0]
	if !f.Enabled {
		t.Error("expected flag enabled")
	}
	if f.Source != "cli" {
		t.Errorf("expected source cli, got %s", f.Source)
	}
}

func newFlagSet() *pflag.FlagSet {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(fs)
	return fs
}

func TestRegisterFlags_FlagAppearsInFlagSet(t *testing.T) {
	fs := newFlagSet()
	f := fs.Lookup("all-runtimes")
	if f == nil {
		t.Fatal("expected --all-runtimes to be registered")
	}
	if f.DefValue != "false" {
		t.Errorf("expected default false, got %s", f.DefValue)
	}
}

func TestApplyCLIOverrides_ExplicitFlagStored(t *testing.T) {
	SetCLIOverrideForTest(t, AllRuntimes, false)
	t.Setenv("DTWIZ_ALL_RUNTIMES", "")

	fs := newFlagSet()
	if err := fs.Parse([]string{"--all-runtimes"}); err != nil {
		t.Fatal(err)
	}
	ApplyCLIOverrides(fs)

	if !IsEnabled(AllRuntimes) {
		t.Error("expected AllRuntimes enabled after CLI override")
	}
	if _, source := resolveFlag(getFlag(AllRuntimes)); source != "cli" {
		t.Errorf("expected source cli, got %s", source)
	}
}

func TestApplyCLIOverrides_CLIBeatsEnvVar(t *testing.T) {
	ClearCLIOverrideForTest(t, AllRuntimes)
	t.Setenv("DTWIZ_ALL_RUNTIMES", "false")

	fs := newFlagSet()
	if err := fs.Parse([]string{"--all-runtimes=true"}); err != nil {
		t.Fatal(err)
	}
	ApplyCLIOverrides(fs)

	if !IsEnabled(AllRuntimes) {
		t.Error("expected AllRuntimes enabled: CLI true should beat env false")
	}
	if _, source := resolveFlag(getFlag(AllRuntimes)); source != "cli" {
		t.Errorf("expected source cli, got %s", source)
	}
}

func TestApplyCLIOverrides_UnsetFlagDoesNotStompEnvVar(t *testing.T) {
	ClearCLIOverrideForTest(t, AllRuntimes)
	t.Setenv("DTWIZ_ALL_RUNTIMES", "false")

	fs := newFlagSet()
	if err := fs.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	ApplyCLIOverrides(fs)

	if IsEnabled(AllRuntimes) {
		t.Error("expected AllRuntimes enabled via env var (CLI flag not passed)")
	}
	if _, source := resolveFlag(getFlag(AllRuntimes)); source != "env" {
		t.Errorf("expected source env, got %s", source)
	}
}
