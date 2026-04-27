package sdk

import (
	"errors"
	"testing"
)

func TestAPIError_NoArcInstance(t *testing.T) {
	err := &APIError{StatusCode: 503, Message: "no arc instance configured for this org"}

	if !errors.Is(err, ErrNoArcInstance) {
		t.Fatalf("expected errors.Is to match ErrNoArcInstance, got false")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected errors.As to match *APIError")
	}
	if apiErr.StatusCode != 503 {
		t.Errorf("expected StatusCode 503, got %d", apiErr.StatusCode)
	}
}

func TestAPIError_OtherStatusDoesNotMatchSentinel(t *testing.T) {
	err := &APIError{StatusCode: 500, Message: "internal server error"}
	if errors.Is(err, ErrNoArcInstance) {
		t.Fatal("non-503 error should not match ErrNoArcInstance")
	}
}

func TestAPIError_Unrelated503DoesNotMatchSentinel(t *testing.T) {
	err := &APIError{StatusCode: 503, Message: "upstream timeout"}
	if errors.Is(err, ErrNoArcInstance) {
		t.Fatal("503 without arc-instance message should not match ErrNoArcInstance")
	}
}
