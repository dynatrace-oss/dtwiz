package featureflags

import (
	"testing"
)

func TestIsEnabled_DefaultIsReturned(t *testing.T) {
	// No env var, no CLI, no test override → default (false)
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

func TestIsEnabled_UnknownFlag(t *testing.T) {
	unknown := Flag(999)
	if IsEnabled(unknown) {
		t.Error("expected unknown flag to return false")
	}
}

func TestSetForTest_OverridesAndRestores(t *testing.T) {
	t.Setenv("DTWIZ_ALL_RUNTIMES", "")

	// Verify default is false
	if IsEnabled(AllRuntimes) {
		t.Fatal("precondition: expected AllRuntimes disabled")
	}

	t.Run("inner", func(t *testing.T) {
		SetForTest(t, AllRuntimes, true)
		if !IsEnabled(AllRuntimes) {
			t.Error("expected AllRuntimes enabled via SetForTest")
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

func TestList_TestSource(t *testing.T) {
	t.Setenv("DTWIZ_ALL_RUNTIMES", "")
	SetForTest(t, AllRuntimes, true)

	flags := List()
	f := flags[0]
	if !f.Enabled {
		t.Error("expected flag enabled")
	}
	if f.Source != "test" {
		t.Errorf("expected source test, got %s", f.Source)
	}
}
