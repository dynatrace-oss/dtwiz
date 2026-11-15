//go:build !windows

package otel

import (
	"reflect"
	"testing"
)

func TestParsePsOutput(t *testing.T) {
	out := "  100 /bin/sh -c x\n  200 ./server --a\nbad line\n  300 go run .\n"
	got := parsePsOutput(out, 300, map[int]string{200: "/work/shop"})
	want := []DetectedProcess{
		{PID: 100, Command: "/bin/sh -c x"},
		{PID: 200, Command: "./server --a", WorkingDirectory: "/work/shop"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePsOutput = %#v, want %#v", got, want)
	}
}

func TestParseLsofCwd(t *testing.T) {
	got := parseLsofCwd("p100\nn/\np200\nn/work/shop\n")
	if got[100] != "/" || got[200] != "/work/shop" {
		t.Fatalf("parseLsofCwd = %v", got)
	}
}
