package display

import (
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/term"
)

const progressPrintInterval = 100 * time.Millisecond

// ProgressReader wraps an io.Reader and prints a download progress line to
// stderr at most once per progressPrintInterval. Output is suppressed when
// stderr is not a TTY (e.g. CI, pipes) to avoid polluting logs.
type ProgressReader struct {
	r         io.Reader
	w         io.Writer
	read      int64
	total     int64 // -1 when Content-Length is unknown
	isTTY     bool
	lastPrint time.Time
}

// NewProgressReader returns a ProgressReader. total should be the expected
// number of bytes, or -1 if unknown (e.g. no Content-Length header).
func NewProgressReader(r io.Reader, total int64) *ProgressReader {
	return &ProgressReader{
		r:     r,
		w:     os.Stderr,
		total: total,
		isTTY: term.IsTerminal(int(os.Stderr.Fd())),
	}
}

func (p *ProgressReader) Read(buf []byte) (int, error) {
	n, err := p.r.Read(buf)
	p.read += int64(n)
	if p.isTTY && time.Since(p.lastPrint) >= progressPrintInterval {
		p.print()
		p.lastPrint = time.Now()
	}
	return n, err
}

func (p *ProgressReader) print() {
	if p.total > 0 {
		pct := float64(p.read) / float64(p.total) * 100
		fmt.Fprintf(p.w, "\r  downloading:  %s / %s (%.0f%%)",
			HumanBytes(p.read), HumanBytes(p.total), pct)
	} else {
		fmt.Fprintf(p.w, "\r  downloading:  %s", HumanBytes(p.read))
	}
}

// Clear erases the progress line so the next output starts on a clean line.
// Should be called after the read is complete (success or failure).
func (p *ProgressReader) Clear() {
	if p.isTTY {
		fmt.Fprint(p.w, "\r\033[2K")
	}
}

// HumanBytes formats a byte count as a human-readable string (e.g. "42MB").
func HumanBytes(n int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%dGB", n/gb)
	case n >= mb:
		return fmt.Sprintf("%dMB", n/mb)
	case n >= kb:
		return fmt.Sprintf("%dKB", n/kb)
	default:
		return fmt.Sprintf("%dB", n)
	}
}
