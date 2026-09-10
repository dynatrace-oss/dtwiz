package cmd

import (
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
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

// ── watchResultToProps ───────────────────────────────────────────────────────

func TestWatchResultToProps_Encoding(t *testing.T) {
	result := installer.WatchSessionResult{
		Duration:    10 * time.Second,
		ExitReason:  "user_exit",
		FirstDataMs: map[string]int64{"svc": 1234, "hst": 5678},
	}
	props := watchResultToProps(result)

	if got := props["watch.dur"]; got != "10000" {
		t.Errorf("watch.dur = %q, want %q", got, "10000")
	}
	if got := props["watch.exit"]; got != "user_exit" {
		t.Errorf("watch.exit = %q, want %q", got, "user_exit")
	}
	// signals must be sorted alphabetically
	if got := props["watch.sig"]; got != "hst,svc" {
		t.Errorf("watch.sig = %q, want %q", got, "hst,svc")
	}
	if got := props["watch.t_svc"]; got != "1234" {
		t.Errorf("watch.t_svc = %q, want %q", got, "1234")
	}
	if got := props["watch.t_hst"]; got != "5678" {
		t.Errorf("watch.t_hst = %q, want %q", got, "5678")
	}
}

func TestWatchResultToProps_AbsentSignalProducesNoKey(t *testing.T) {
	result := installer.WatchSessionResult{
		Duration:    2 * time.Second,
		ExitReason:  "timeout",
		FirstDataMs: map[string]int64{"svc": 500},
	}
	props := watchResultToProps(result)

	if _, ok := props["watch.t_hst"]; ok {
		t.Error("watch.t_hst must not be present when hst was not seen")
	}
	if got := props["watch.sig"]; got != "svc" {
		t.Errorf("watch.sig = %q, want %q", got, "svc")
	}
}

func TestWatchResultToProps_NilFirstDataMsDoesNotPanic(t *testing.T) {
	// WatchSessionResult zero value — pToken == "" early-return path leaves FirstDataMs nil.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("watchResultToProps panicked with nil FirstDataMs: %v", r)
		}
	}()
	props := watchResultToProps(installer.WatchSessionResult{})
	if got := props["watch.sig"]; got != "" {
		t.Errorf("watch.sig = %q, want empty string for zero-value result", got)
	}
}
