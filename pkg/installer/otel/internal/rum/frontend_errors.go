package rum

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const (
	// frontendsListPath is the UI page that lists all frontends of a tenant.
	frontendsListPath = "/ui/apps/dynatrace.experience.vitals/frontends/"

	deleteFrontendDocsURL = "https://docs.dynatrace.com/docs/observe/digital-experience/rum/web-frontends/additional-configuration/delete-frontend-web"

	// frontendsWriteScope is the platform token scope required to create frontends.
	frontendsWriteScope = "rum:frontends:write"
)

// ConflictError is returned when the tenant already has a frontend with the requested
// name, typically because an earlier run created it but could not save its ID.
type ConflictError struct {
	FrontendName string
	FrontendsURL string // tenant UI page listing the frontends
	ConfigPath   string // project config file the ID can be copied into
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf(`a frontend named %q already exists on the tenant
  Find it in the list: %s
  Then either:
    - copy its ID into %s (entry under "frontends" for this environment), or
    - delete the frontend (%s) and run dtwiz again`,
		e.FrontendName, e.FrontendsURL, e.ConfigPath, deleteFrontendDocsURL)
}

// errorEnvelope is the common error body of the Dynatrace Platform APIs.
type errorEnvelope struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// isNameConflict reports whether the response says the frontend name is not unique.
// The API uses a plain 400 for this, so the message text is the only signal; an
// unparseable body or an unrelated message is not treated as a conflict.
func isNameConflict(status int, body []byte) bool {
	if status != http.StatusBadRequest {
		return false
	}
	var env errorEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(env.Error.Message), "unique")
}

// permissionError explains a 401/403. A missing scope and a policy restricting it look
// the same in the response, so the scope is offered as the likely cause, not a certainty.
func permissionError(status int, body []byte) error {
	return fmt.Errorf("create frontend application: rejected with status %d; the platform token most likely needs the %s scope: %s",
		status, frontendsWriteScope, body)
}
