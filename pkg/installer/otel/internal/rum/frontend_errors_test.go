package rum

import (
	"net/http"
	"testing"
)

func TestIsNameConflict(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"observed duplicate response", http.StatusBadRequest, duplicateNameBody, true},
		{"case insensitive", http.StatusBadRequest, `{"error":{"message":"Name must be UNIQUE"}}`, true},
		{"unrelated 400", http.StatusBadRequest, `{"error":{"message":"displayName is blank"}}`, false},
		{"unparseable body", http.StatusBadRequest, `must be unique`, false},
		{"unique message on other status", http.StatusInternalServerError, duplicateNameBody, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNameConflict(tt.status, []byte(tt.body)); got != tt.want {
				t.Errorf("isNameConflict = %v, want %v", got, tt.want)
			}
		})
	}
}
