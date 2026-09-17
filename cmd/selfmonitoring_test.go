package cmd

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/dtwiz/pkg/selfmonitoring"
)

func TestCompletedEventParams_ErrField(t *testing.T) {
	cmd := &cobra.Command{Use: "analyze", Args: cobra.NoArgs}
	rootCmd.AddCommand(cmd)
	defer rootCmd.RemoveCommand(cmd)

	t.Run("nil error produces empty Err", func(t *testing.T) {
		p := completedEventParams(cmd, nil)
		if p.StepID != selfmonitoring.StepCompleted {
			t.Errorf("StepID = %q, want %q", p.StepID, selfmonitoring.StepCompleted)
		}
		if p.Err != "" {
			t.Errorf("Err = %q, want empty", p.Err)
		}
	})

	t.Run("non-nil error produces Err=err", func(t *testing.T) {
		p := completedEventParams(cmd, errors.New("something failed"))
		if p.Err != "err" {
			t.Errorf("Err = %q, want %q", p.Err, "err")
		}
	})
}

func TestNormCmdInformationalCommands(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"analyze", "ana"},
		{"recommend", "rec"},
		{"status", "sta"},
		{"version", "ver"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normCmd(tt.input)
			if got != tt.want {
				t.Errorf("normCmd(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormSubUpdateVariants(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"otel-update", "otlu"},
		{"azure-update", "azu"},
		{"gcp-update", "gcpu"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normSub(tt.input)
			if got != tt.want {
				t.Errorf("normSub(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
