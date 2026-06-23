//go:build windows

package oneagent

import (
	"testing"
)

func TestQuoteWindowsArg(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "simple flag — no quoting needed",
			in:   "--quiet",
			want: "--quiet",
		},
		{
			name: "flag with value — no quoting needed",
			in:   "--set-monitoring-mode=fullstack",
			want: "--set-monitoring-mode=fullstack",
		},
		{
			name: "empty string — must be quoted so the arg survives",
			in:   "",
			want: `""`,
		},
		{
			name: "contains space — must be quoted",
			in:   "hello world",
			want: `"hello world"`,
		},
		{
			name: "contains tab — must be quoted",
			in:   "hello\tworld",
			want: "\"hello\tworld\"",
		},
		{
			name: "contains double-quote — quote must be escaped",
			in:   `say "hi"`,
			want: `"say \"hi\""`,
		},
		{
			name: "backslash not before quote — passed through literally",
			in:   `C:\path\to\file`,
			want: `C:\path\to\file`,
		},
		{
			name: "path with space — backslashes passed through, arg quoted",
			in:   `C:\Program Files\tool`,
			want: `"C:\Program Files\tool"`,
		},
		{
			name: "trailing backslash inside quoted arg — must be doubled",
			in:   `C:\Program Files\`,
			want: `"C:\Program Files\\"`,
		},
		{
			name: "backslash immediately before embedded quote — both must be escaped",
			in:   `path\"value`,
			want: `"path\\\"value"`,
		},
		{
			name: "multiple trailing backslashes in quoted arg — all doubled",
			in:   `C:\Program Files\\`,
			want: `"C:\Program Files\\\\"`,
		},
		{
			name: "backslashes before quote in middle of arg",
			in:   `a\\\"b`,
			want: `"a\\\\\\\"b"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := quoteWindowsArg(tc.in)
			if got != tc.want {
				t.Errorf("quoteWindowsArg(%q)\n got  %q\n want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestQuoteWindowsArgs(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{
			name: "no args",
			in:   []string{},
			want: "",
		},
		{
			name: "single simple arg",
			in:   []string{"--quiet"},
			want: "--quiet",
		},
		{
			name: "multiple simple args joined by space",
			in:   []string{"--quiet", "--set-monitoring-mode=fullstack"},
			want: "--quiet --set-monitoring-mode=fullstack",
		},
		{
			name: "mix of plain and args-needing-quoting",
			in:   []string{"--quiet", `C:\Program Files\tool`, "--set-monitoring-mode=fullstack"},
			want: `--quiet "C:\Program Files\tool" --set-monitoring-mode=fullstack`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := quoteWindowsArgs(tc.in)
			if got != tc.want {
				t.Errorf("quoteWindowsArgs(%q)\n got  %q\n want %q", tc.in, got, tc.want)
			}
		})
	}
}
