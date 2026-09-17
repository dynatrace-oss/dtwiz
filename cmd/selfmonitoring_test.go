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

func TestDeriveCommandNames_InformationalCommands(t *testing.T) {
	tests := []struct {
		cmdName string
		wantCmd string
		wantSub string
	}{
		{"analyze", "analyze", ""},
		{"recommend", "recommend", ""},
		{"status", "status", ""},
		{"version", "version", ""},
	}
	for _, tt := range tests {
		t.Run(tt.cmdName, func(t *testing.T) {
			cmd := &cobra.Command{Use: tt.cmdName, Args: cobra.NoArgs}
			rootCmd.AddCommand(cmd)
			defer rootCmd.RemoveCommand(cmd)

			gotCmd, gotSub := deriveCommandNames(cmd)
			if gotCmd != tt.wantCmd {
				t.Errorf("deriveCommandNames(%q) cmdName = %q, want %q", tt.cmdName, gotCmd, tt.wantCmd)
			}
			if gotSub != tt.wantSub {
				t.Errorf("deriveCommandNames(%q) subName = %q, want %q", tt.cmdName, gotSub, tt.wantSub)
			}
		})
	}
}
