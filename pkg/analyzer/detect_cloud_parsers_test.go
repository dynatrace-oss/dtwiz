package analyzer

import "testing"

func TestParseIntFirst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		out  string
		want int
	}{
		{name: "empty output", out: "", want: 0},
		{name: "integer", out: "42", want: 42},
		{name: "integer with whitespace", out: "  7\n", want: 7},
		{name: "first field wins", out: "12 ignored", want: 12},
		{name: "invalid first field", out: "none 12", want: 0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := parseIntFirst(tt.out); got != tt.want {
				t.Fatalf("parseIntFirst() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseAzureInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		out  string
		want int
	}{
		{name: "empty output", out: "", want: 0},
		{name: "integer", out: "3", want: 3},
		{name: "integer with trailing text", out: "5 resources", want: 5},
		{name: "invalid", out: "N/A", want: 0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := parseAzureInt(tt.out); got != tt.want {
				t.Fatalf("parseAzureInt() = %d, want %d", got, tt.want)
			}
		})
	}
}
