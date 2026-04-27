package sdk

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNoArcInstance is returned (wrapped) when the caller's organization has no
// Arc instance configured. Memtrace deployments are multi-tenant; an
// administrator must run `memtrace org add-arc <org_id>` before that
// organization can read or write memories. Use errors.Is to test.
//
//	if errors.Is(err, sdk.ErrNoArcInstance) {
//	    // surface a clearer message to the user
//	}
var ErrNoArcInstance = errors.New("no arc instance configured for org")

// APIError is the typed error returned by every Client method on a non-2xx
// response. It exposes the HTTP status code and the server's `error` message
// so callers can branch on either. APIError wraps ErrNoArcInstance when the
// server reports a missing Arc instance, so errors.Is(err, ErrNoArcInstance)
// works without the caller needing to know about APIError.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("memtrace api error (status %d): %s", e.StatusCode, e.Message)
}

// Unwrap reports ErrNoArcInstance for 503 responses whose body matches the
// server's "no arc instance" string, so callers can write
// errors.Is(err, ErrNoArcInstance).
func (e *APIError) Unwrap() error {
	if e.StatusCode == 503 && strings.Contains(strings.ToLower(e.Message), "arc instance") {
		return ErrNoArcInstance
	}
	return nil
}
