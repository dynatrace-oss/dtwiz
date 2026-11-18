package cmd

import "testing"

func TestWatchCmd_ExitOnDataFlagRegistered(t *testing.T) {
	// Arrange
	const name = "exit-on-data"

	// Act
	flag := watchCmd.Flags().Lookup(name)

	// Assert
	if flag == nil {
		t.Fatalf("watch has no --%s flag", name)
	}
	if flag.Value.Type() != "bool" {
		t.Errorf("--%s type = %q, want bool", name, flag.Value.Type())
	}
	if flag.DefValue != "false" {
		t.Errorf("--%s default = %q, want false", name, flag.DefValue)
	}
}
