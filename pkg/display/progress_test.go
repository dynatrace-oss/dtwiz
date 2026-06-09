package display

import "testing"

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		input int64
		want  string
	}{
		{0, "0B"},
		{512, "512B"},
		{1024, "1KB"},
		{1536, "1KB"},
		{2 * 1024 * 1024, "2MB"},
		{3 * 1024 * 1024 * 1024, "3GB"},
	}
	for _, c := range cases {
		if got := HumanBytes(c.input); got != c.want {
			t.Errorf("HumanBytes(%d) = %q, want %q", c.input, got, c.want)
		}
	}
}
