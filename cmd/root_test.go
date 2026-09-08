package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestNormCmd(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want string
	}{
		{"install", "install", "ins"},
		{"uninstall", "uninstall", "uni"},
		{"update", "update", "upd"},
		{"analyze", "analyze", "ana"},
		{"recommend", "recommend", "rec"},
		{"status", "status", "sta"},
		{"watch", "watch", "wch"},
		{"setup", "setup", "set"},
		{"version", "version", "ver"},
		{"unknown", "unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normCmd(tt.cmd)
			if got != tt.want {
				t.Errorf("normCmd(%q) = %q, want %q", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestNormSub(t *testing.T) {
	tests := []struct {
		name string
		sub  string
		want string
	}{
		{"otel", "otel", "otel"},
		{"otel-collector", "otel-collector", "otlc"},
		{"otel-python", "otel-python", "otlp"},
		{"otel-node", "otel-node", "otln"},
		{"otel-java", "otel-java", "otlj"},
		{"kubernetes", "kubernetes", "k8s"},
		{"oneagent", "oneagent", "oa"},
		{"gcp", "gcp", "gcp"},
		{"azure", "azure", "az"},
		{"aws", "aws", "aws"},
		{"aws-lambda", "aws-lambda", "awsl"},
		{"docker", "docker", "dock"},
		{"demo", "demo", "demo"},
		{"self", "self", "self"},
		{"unknown", "unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normSub(tt.sub)
			if got != tt.want {
				t.Errorf("normSub(%q) = %q, want %q", tt.sub, got, tt.want)
			}
		})
	}
}

func TestDeriveCommandIDs(t *testing.T) {
	tests := []struct {
		name    string
		setupFn func() *cobra.Command
		wantCmd string
		wantSub string
	}{
		{
			name: "root_level_command",
			setupFn: func() *cobra.Command {
				cmd := &cobra.Command{Use: "watch"}
				// No parent means root-level
				return cmd
			},
			wantCmd: "wch",
			wantSub: "",
		},
		{
			name: "subcommand_install_otel",
			setupFn: func() *cobra.Command {
				parent := &cobra.Command{Use: "install"}
				cmd := &cobra.Command{Use: "otel"}
				parent.AddCommand(cmd)
				return cmd
			},
			wantCmd: "ins",
			wantSub: "otel",
		},
		{
			name: "subcommand_uninstall_kubernetes",
			setupFn: func() *cobra.Command {
				parent := &cobra.Command{Use: "uninstall"}
				cmd := &cobra.Command{Use: "kubernetes"}
				parent.AddCommand(cmd)
				return cmd
			},
			wantCmd: "uni",
			wantSub: "k8s",
		},
		{
			name: "subcommand_update_azure",
			setupFn: func() *cobra.Command {
				parent := &cobra.Command{Use: "update"}
				cmd := &cobra.Command{Use: "azure"}
				parent.AddCommand(cmd)
				return cmd
			},
			wantCmd: "upd",
			wantSub: "az",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.setupFn()
			gotCmd, gotSub := deriveCommandIDs(cmd)
			if gotCmd != tt.wantCmd {
				t.Errorf("deriveCommandIDs cmd = %q, want %q", gotCmd, tt.wantCmd)
			}
			if gotSub != tt.wantSub {
				t.Errorf("deriveCommandIDs sub = %q, want %q", gotSub, tt.wantSub)
			}
		})
	}
}

func TestResolveMode(t *testing.T) {
	tests := []struct {
		name        string
		setup       func()
		cleanup     func()
		wantInvalid bool // we can't easily set debugFlag or TTY in tests
	}{
		{
			name:        "resolveMode_returns_valid_value",
			setup:       func() {},
			cleanup:     func() {},
			wantInvalid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			defer tt.cleanup()

			mode := resolveMode()

			// Mode should be one of the three valid values
			validModes := map[string]bool{
				"deb": true,
				"tty": true,
				"ntt": true,
			}

			if !validModes[mode] {
				t.Errorf("resolveMode() = %q, want one of: deb, tty, ntt", mode)
			}

			if tt.wantInvalid {
				t.Error("unexpected valid mode in test expecting invalid")
			}
		})
	}
}
