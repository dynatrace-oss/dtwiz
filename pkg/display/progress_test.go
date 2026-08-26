package display

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

func TestHumanBytes(t *testing.T) {
	t.Parallel()

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
		c := c
		t.Run(c.want, func(t *testing.T) {
			t.Parallel()

			if got := HumanBytes(c.input); got != c.want {
				t.Errorf("HumanBytes(%d) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

func TestProgressReaderReadsAndPrintsKnownTotal(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := &ProgressReader{
		r:         strings.NewReader("abcdef"),
		w:         &out,
		total:     6,
		isTTY:     true,
		lastPrint: time.Now().Add(-progressPrintInterval),
	}

	buf := make([]byte, 3)
	n, err := p.Read(buf)
	if err != nil {
		t.Fatalf("Read() returned error: %v", err)
	}
	if n != 3 || string(buf) != "abc" {
		t.Fatalf("Read() = (%d, %q), want (3, %q)", n, string(buf), "abc")
	}
	if p.read != 3 {
		t.Fatalf("read count = %d, want 3", p.read)
	}
	if got := out.String(); !strings.Contains(got, "3B / 6B (50%)") {
		t.Fatalf("progress output = %q, want known-total progress", got)
	}
}

func TestProgressReaderSuppressesOutputWhenNotTTY(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := &ProgressReader{
		r:     strings.NewReader("abc"),
		w:     &out,
		total: -1,
		isTTY: false,
	}

	if _, err := io.ReadAll(p); err != nil {
		t.Fatalf("ReadAll() returned error: %v", err)
	}
	if got := out.String(); got != "" {
		t.Fatalf("progress output = %q, want empty for non-TTY", got)
	}
}

func TestProgressReaderClear(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		isTTY bool
		want  string
	}{
		{name: "tty clears line", isTTY: true, want: "\r\033[2K"},
		{name: "non tty does nothing", isTTY: false, want: ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			p := &ProgressReader{w: &out, isTTY: tt.isTTY}
			p.Clear()
			if got := out.String(); got != tt.want {
				t.Fatalf("Clear() output = %q, want %q", got, tt.want)
			}
		})
	}
}
