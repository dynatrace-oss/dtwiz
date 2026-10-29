package oneagent

import (
	"reflect"
	"strings"
	"testing"
)

func TestValidateHostProperties(t *testing.T) {
	tests := []struct {
		name    string
		props   []string
		wantErr string
	}{
		{name: "none", props: nil},
		{name: "key only", props: []string{"pci"}},
		{name: "key value", props: []string{"team=payments"}},
		{name: "allowed punctuation", props: []string{"aws.account_id=123-456"}},
		{name: "several valid", props: []string{"team=payments", "env=prod", "pci"}},
		{name: "max length", props: []string{"k=" + strings.Repeat("v", maxHostPropertyLen-2)}},
		{name: "too long", props: []string{"k=" + strings.Repeat("v", maxHostPropertyLen-1)}, wantErr: "longer than 256 characters"},
		{name: "whitespace", props: []string{"team=web shop"}, wantErr: `unsupported character ' '`},
		{name: "two equals", props: []string{"a=b=c"}, wantErr: "more than one '='"},
		{name: "empty key", props: []string{"=prod"}, wantErr: "empty key or value"},
		{name: "empty value", props: []string{"team="}, wantErr: "empty key or value"},
		{name: "empty string", props: []string{""}, wantErr: "empty key or value"},
		{name: "non-ascii", props: []string{"team=zürich"}, wantErr: "unsupported character"},
		{name: "second value invalid", props: []string{"team=payments", "env=pr od"}, wantErr: `"env=pr od"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHostProperties(tt.props)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestHostPropertyArgs_PreservesOrder(t *testing.T) {
	got := hostPropertyArgs([]string{"team=payments", "env=prod", "pci"})
	want := []string{"--set-host-property=team=payments", "--set-host-property=env=prod", "--set-host-property=pci"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestHostPropertyArgs_Empty(t *testing.T) {
	if got := hostPropertyArgs(nil); len(got) != 0 {
		t.Errorf("expected no args, got %v", got)
	}
}
