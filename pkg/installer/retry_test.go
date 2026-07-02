package installer

import "testing"

func TestJitter_WithinExpectedBounds(t *testing.T) {
	const base = 100
	for i := 0; i < 1000; i++ {
		got := Jitter(base)
		if got < base || got > base+base/2 {
			t.Fatalf("Jitter(%d) = %d, want within [%d, %d]", base, got, base, base+base/2)
		}
	}
}

func TestJitter_ZeroOrNegativeIsUnchanged(t *testing.T) {
	if got := Jitter(0); got != 0 {
		t.Errorf("Jitter(0) = %d, want 0", got)
	}
	if got := Jitter(-5); got != -5 {
		t.Errorf("Jitter(-5) = %d, want -5", got)
	}
}
