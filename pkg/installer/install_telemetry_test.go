package installer

import (
	"sync"
	"testing"
	"time"
)

// fakeClock replaces nowFn for the duration of a test and returns a function that
// moves the clock forward.
func fakeClock(t *testing.T) (now func() time.Time, advance func(time.Duration)) {
	t.Helper()
	cur := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	old := nowFn
	nowFn = func() time.Time { return cur }
	ResetInstallTelemetry()
	t.Cleanup(func() {
		nowFn = old
		ResetInstallTelemetry()
	})
	return func() time.Time { return cur }, func(d time.Duration) { cur = cur.Add(d) }
}

func TestFeatureOutcomes_DefaultIsEmpty(t *testing.T) {
	ResetInstallTelemetry()
	if got := FeatureOutcomes(); len(got) != 0 {
		t.Errorf("FeatureOutcomes() = %v, want empty (all not tried)", got)
	}
}

func TestRecordFeature_LastWriteWins(t *testing.T) {
	ResetInstallTelemetry()
	t.Cleanup(ResetInstallTelemetry)

	RecordFeature(FeatureHostMonitoring, false)
	RecordFeature(FeatureHostMonitoring, true)
	RecordFeature(FeatureOtelConfig, false)

	got := FeatureOutcomes()
	if got[FeatureHostMonitoring] != OutcomeSucceeded {
		t.Errorf("host monitoring = %v, want succeeded", got[FeatureHostMonitoring])
	}
	if got[FeatureOtelConfig] != OutcomeFailed {
		t.Errorf("otel config = %v, want failed", got[FeatureOtelConfig])
	}
	if got[FeatureRUM] != OutcomeNotTried {
		t.Errorf("rum = %v, want not tried", got[FeatureRUM])
	}
}

func TestFeatureOutcomes_ReturnsCopy(t *testing.T) {
	ResetInstallTelemetry()
	t.Cleanup(ResetInstallTelemetry)

	RecordFeature(FeatureOtelConfig, true)
	got := FeatureOutcomes()
	got[FeatureOtelConfig] = OutcomeFailed
	if FeatureOutcomes()[FeatureOtelConfig] != OutcomeSucceeded {
		t.Error("mutating the returned map must not change the recorded outcomes")
	}
}

func TestRecordFeature_Concurrent(t *testing.T) {
	ResetInstallTelemetry()
	t.Cleanup(ResetInstallTelemetry)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(ok bool) {
			defer wg.Done()
			RecordFeature(FeatureHostMonitoring, ok)
			_ = FeatureOutcomes()
		}(i%2 == 0)
	}
	wg.Wait()

	if FeatureOutcomes()[FeatureHostMonitoring] == OutcomeNotTried {
		t.Error("an outcome must be recorded after concurrent writes")
	}
}

func TestResetInstallTelemetry_ClearsOutcomesAndStopwatch(t *testing.T) {
	_, advance := fakeClock(t)
	RecordFeature(FeatureOtelConfig, true)
	StartInstallTimer()
	advance(time.Second)
	MarkInstallDone()

	ResetInstallTelemetry()

	if len(FeatureOutcomes()) != 0 {
		t.Error("reset must clear outcomes")
	}
	if _, started := InstallWorkTime(); started {
		t.Error("reset must stop and clear the stopwatch")
	}
}

func TestInstallWorkTime_NotStarted(t *testing.T) {
	fakeClock(t)
	if d, started := InstallWorkTime(); started || d != 0 {
		t.Errorf("InstallWorkTime() = %v, %v; want 0, false", d, started)
	}
}

func TestInstallWorkTime_EndsWhenReadIfNotMarked(t *testing.T) {
	_, advance := fakeClock(t)
	StartInstallTimer()
	advance(20 * time.Second)

	d, started := InstallWorkTime()
	if !started || d != 20*time.Second {
		t.Errorf("InstallWorkTime() = %v, %v; want 20s, true", d, started)
	}
}

func TestStartInstallTimer_DoesNotRestart(t *testing.T) {
	_, advance := fakeClock(t)
	StartInstallTimer()
	advance(10 * time.Second)
	StartInstallTimer()
	advance(5 * time.Second)

	if d, _ := InstallWorkTime(); d != 15*time.Second {
		t.Errorf("InstallWorkTime() = %v, want 15s", d)
	}
}

func TestMarkInstallDone_FirstCallWins(t *testing.T) {
	_, advance := fakeClock(t)
	StartInstallTimer()
	advance(8 * time.Second)
	MarkInstallDone()
	advance(10 * time.Minute) // post-install watch
	MarkInstallDone()

	if d, _ := InstallWorkTime(); d != 8*time.Second {
		t.Errorf("InstallWorkTime() = %v, want 8s", d)
	}
}

func TestMarkInstallDone_IgnoredBeforeStart(t *testing.T) {
	_, advance := fakeClock(t)
	MarkInstallDone()
	advance(time.Second)
	StartInstallTimer()
	advance(3 * time.Second)

	if d, started := InstallWorkTime(); !started || d != 3*time.Second {
		t.Errorf("InstallWorkTime() = %v, %v; want 3s, true", d, started)
	}
}

func TestInstallWorkTime_SubtractsPause(t *testing.T) {
	now, advance := fakeClock(t)
	StartInstallTimer()
	advance(5 * time.Second)

	// A prompt shown after 5s of work and answered after 40s.
	promptStart := now()
	advance(40 * time.Second)
	afterPrompt(promptStart, true, true)

	advance(10 * time.Second)

	if d, _ := InstallWorkTime(); d != 15*time.Second {
		t.Errorf("InstallWorkTime() = %v, want 15s (5s before + 10s after the prompt)", d)
	}
}

func TestInstallWorkTime_PauseAfterMarkedEndNotSubtracted(t *testing.T) {
	now, advance := fakeClock(t)
	StartInstallTimer()
	advance(4 * time.Minute)
	MarkInstallDoneAt(now().Add(-3 * time.Minute)) // AWS: the later of deploy and Lambda, in the past

	// A watch-time prompt entirely after the marked end.
	promptStart := now()
	advance(30 * time.Second)
	afterPrompt(promptStart, true, true)

	if d, _ := InstallWorkTime(); d != time.Minute {
		t.Errorf("InstallWorkTime() = %v, want 1m", d)
	}
}

func TestInstallWorkTime_PauseStraddlingEndIsClipped(t *testing.T) {
	now, advance := fakeClock(t)
	StartInstallTimer()
	advance(10 * time.Second)
	promptStart := now()
	advance(20 * time.Second)
	endAt := promptStart.Add(5 * time.Second) // work ended 5s into the prompt
	afterPrompt(promptStart, true, true)
	MarkInstallDoneAt(endAt)

	if d, _ := InstallWorkTime(); d != 10*time.Second {
		t.Errorf("InstallWorkTime() = %v, want 10s", d)
	}
}

func TestAfterPrompt_AcceptedStartingPromptStartsStopwatch(t *testing.T) {
	now, advance := fakeClock(t)
	promptStart := now()
	advance(30 * time.Second) // think time, must not count
	afterPrompt(promptStart, true, true)
	advance(20 * time.Second)

	if d, started := InstallWorkTime(); !started || d != 20*time.Second {
		t.Errorf("InstallWorkTime() = %v, %v; want 20s, true", d, started)
	}
}

func TestAfterPrompt_DeclinedStartingPromptDoesNotStart(t *testing.T) {
	now, advance := fakeClock(t)
	promptStart := now()
	advance(time.Second)
	afterPrompt(promptStart, false, true)

	if _, started := InstallWorkTime(); started {
		t.Error("a declined confirmation must not start the stopwatch")
	}
}

func TestAfterPrompt_NonStartingPromptDoesNotStart(t *testing.T) {
	now, advance := fakeClock(t)
	promptStart := now()
	advance(time.Second)
	afterPrompt(promptStart, true, false)

	if _, started := InstallWorkTime(); started {
		t.Error("a non-install question must not start the stopwatch")
	}
}

func TestConfirmProceed_AcceptStartsStopwatch(t *testing.T) {
	old := AutoConfirm
	AutoConfirm = false
	defer func() { AutoConfirm = old }()
	ResetInstallTelemetry()
	t.Cleanup(ResetInstallTelemetry)

	withStdin(t, "y\n", func() {
		if ok, err := ConfirmProceed("  Proceed?"); !ok || err != nil {
			t.Fatalf("ConfirmProceed = %v, %v; want true, nil", ok, err)
		}
	})
	if _, started := InstallWorkTime(); !started {
		t.Error("an accepted confirmation must start the stopwatch")
	}
}

func TestConfirmProceed_DeclineDoesNotStartStopwatch(t *testing.T) {
	old := AutoConfirm
	AutoConfirm = false
	defer func() { AutoConfirm = old }()
	ResetInstallTelemetry()
	t.Cleanup(ResetInstallTelemetry)

	withStdin(t, "n\n", func() {
		if ok, _ := ConfirmProceed("  Proceed?"); ok {
			t.Fatal("ConfirmProceed must return false for n")
		}
	})
	if _, started := InstallWorkTime(); started {
		t.Error("a declined confirmation must not start the stopwatch")
	}
}

func TestConfirmProceed_AutoConfirmStartsStopwatch(t *testing.T) {
	old := AutoConfirm
	AutoConfirm = true
	defer func() { AutoConfirm = old }()
	ResetInstallTelemetry()
	t.Cleanup(ResetInstallTelemetry)

	if ok, err := ConfirmProceed("  Proceed?"); !ok || err != nil {
		t.Fatalf("ConfirmProceed = %v, %v; want true, nil", ok, err)
	}
	if _, started := InstallWorkTime(); !started {
		t.Error("AutoConfirm must start the stopwatch")
	}
}

func TestConfirmProceed_SecondPromptDoesNotRestartStopwatch(t *testing.T) {
	old := AutoConfirm
	AutoConfirm = false
	defer func() { AutoConfirm = old }()
	_, advance := fakeClock(t)

	withStdin(t, "y\ny\n", func() {
		_, _ = ConfirmProceed("  First?")
		advance(10 * time.Second)
		_, _ = ConfirmProceed("  Second?")
		advance(5 * time.Second)
	})

	// The fake clock only moves when advanced, so the prompts themselves take no time:
	// the work before the second prompt must be kept, not discarded by a restart.
	if d, _ := InstallWorkTime(); d != 15*time.Second {
		t.Errorf("InstallWorkTime() = %v, want 15s", d)
	}
}

func TestConfirmQuestion_NeverStartsStopwatch(t *testing.T) {
	old := AutoConfirm
	AutoConfirm = false
	defer func() { AutoConfirm = old }()
	ResetInstallTelemetry()
	t.Cleanup(ResetInstallTelemetry)

	withStdin(t, "y\n", func() {
		if ok, err := ConfirmQuestion("  Select another project?"); !ok || err != nil {
			t.Fatalf("ConfirmQuestion = %v, %v; want true, nil", ok, err)
		}
	})
	if _, started := InstallWorkTime(); started {
		t.Error("ConfirmQuestion must not start the stopwatch")
	}
}

func TestConfirmQuestion_AutoConfirmNeverStartsStopwatch(t *testing.T) {
	old := AutoConfirm
	AutoConfirm = true
	defer func() { AutoConfirm = old }()
	ResetInstallTelemetry()
	t.Cleanup(ResetInstallTelemetry)

	if ok, _ := ConfirmQuestion("  Select another project?"); !ok {
		t.Fatal("AutoConfirm must answer yes")
	}
	if _, started := InstallWorkTime(); started {
		t.Error("ConfirmQuestion must not start the stopwatch, even with AutoConfirm")
	}
}
