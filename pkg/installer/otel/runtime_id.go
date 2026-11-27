package otel

import (
	"errors"
	"fmt"
	"strings"
)

// RuntimeID is the stable, user-facing identifier of an OTel runtime, as
// accepted by --runtime and printed by `dtwiz analyze projects`.
type RuntimeID string

const (
	RuntimePython RuntimeID = "python"
	RuntimeJava   RuntimeID = "java"
	RuntimeNodeJS RuntimeID = "nodejs"
	RuntimeGo     RuntimeID = "go"
	RuntimeDotNet RuntimeID = "dotnet"
)

// ErrRuntimeSingleValue is returned when a single runtime is expected but a
// comma-separated list was given.
var ErrRuntimeSingleValue = errors.New("--runtime takes a single value")

// UnknownRuntimeError reports a runtime identifier outside the active set.
type UnknownRuntimeError struct {
	Value string
	Valid []RuntimeID
}

func (e *UnknownRuntimeError) Error() string {
	return fmt.Sprintf("unknown runtime %q (valid: %s)", e.Value, FormatRuntimeIDs(e.Valid))
}

// ActiveRuntimeIDs returns the runtime identifiers that are currently valid, in
// their documented order. Runtimes gated by a feature flag that is off are not
// part of the set.
func ActiveRuntimeIDs() []RuntimeID {
	ids := make([]RuntimeID, 0, 5)
	for _, rt := range detectAvailableRuntimes() {
		if rt.enabled {
			ids = append(ids, rt.id)
		}
	}
	return ids
}

// FormatRuntimeIDs joins identifiers for help and error texts.
func FormatRuntimeIDs(ids []RuntimeID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = string(id)
	}
	return strings.Join(parts, ", ")
}

// activeRuntime returns the runtime definition for id when id is active.
func activeRuntime(id RuntimeID) (runtimeInfo, bool) {
	for _, rt := range detectAvailableRuntimes() {
		if rt.enabled && rt.id == id {
			return rt, true
		}
	}
	return runtimeInfo{}, false
}

// ParseRuntime parses exactly one runtime identifier. Matching is exact.
func ParseRuntime(value string) (RuntimeID, error) {
	if strings.Contains(value, ",") {
		return "", ErrRuntimeSingleValue
	}
	if _, ok := activeRuntime(RuntimeID(value)); !ok {
		return "", &UnknownRuntimeError{Value: value, Valid: ActiveRuntimeIDs()}
	}
	return RuntimeID(value), nil
}

// ParseRuntimeFilter parses a comma-separated list of runtime identifiers.
// An empty value means no filter and returns nil. Empty elements and unknown
// identifiers are rejected; duplicates are collapsed, keeping the first.
func ParseRuntimeFilter(value string) ([]RuntimeID, error) {
	if value == "" {
		return nil, nil
	}
	var ids []RuntimeID
	seen := make(map[RuntimeID]bool)
	for _, element := range strings.Split(value, ",") {
		id := RuntimeID(element)
		if _, ok := activeRuntime(id); !ok {
			return nil, &UnknownRuntimeError{Value: element, Valid: ActiveRuntimeIDs()}
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, nil
}
