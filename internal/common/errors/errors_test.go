package errors

import (
	"errors"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrNotFound", ErrNotFound},
		{"ErrAlreadyExists", ErrAlreadyExists},
		{"ErrStorageFull", ErrStorageFull},
		{"ErrSyncFailed", ErrSyncFailed},
		{"ErrConflict", ErrConflict},
		{"ErrUnauthorized", ErrUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("%s should not be nil", tt.name)
			}
			if tt.err.Error() == "" {
				t.Errorf("%s should have error message", tt.name)
			}
		})
	}
}

func TestJzSEError(t *testing.T) {
	baseErr := errors.New("base error")
	jzseErr := E("TestOp", ErrNotFound, baseErr, "extra details")

	t.Run("Error message format", func(t *testing.T) {
		msg := jzseErr.Error()
		if msg == "" {
			t.Error("error message should not be empty")
		}
		// Should contain operation, kind, and details
		if !contains(msg, "TestOp") {
			t.Error("error message should contain operation")
		}
	})

	t.Run("Unwrap", func(t *testing.T) {
		unwrapped := errors.Unwrap(jzseErr)
		if unwrapped != baseErr {
			t.Errorf("Unwrap() = %v, want %v", unwrapped, baseErr)
		}
	})

	t.Run("Is ErrNotFound", func(t *testing.T) {
		if !errors.Is(jzseErr, ErrNotFound) {
			t.Error("errors.Is should match ErrNotFound")
		}
	})

	t.Run("Is base error", func(t *testing.T) {
		if !errors.Is(jzseErr, baseErr) {
			t.Error("errors.Is should match base error")
		}
	})
}

func TestE_WithoutDetails(t *testing.T) {
	err := E("Op", ErrConflict, nil)

	msg := err.Error()
	if msg == "" {
		t.Error("error message should not be empty")
	}
}

func TestWrap(t *testing.T) {
	t.Run("Wrap nil", func(t *testing.T) {
		if Wrap("Op", nil) != nil {
			t.Error("Wrap(nil) should return nil")
		}
	})

	t.Run("Wrap error", func(t *testing.T) {
		baseErr := errors.New("base")
		wrapped := Wrap("Op", baseErr)
		if wrapped == nil {
			t.Error("Wrap should return wrapped error")
		}
		if !errors.Is(wrapped, baseErr) {
			t.Error("wrapped error should match base")
		}
	})
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"ErrNotFound", ErrNotFound, true},
		{"wrapped ErrNotFound", E("Op", ErrNotFound, nil), true},
		{"other error", ErrConflict, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.want {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsConflict(t *testing.T) {
	if !IsConflict(ErrConflict) {
		t.Error("IsConflict(ErrConflict) should be true")
	}
	if IsConflict(ErrNotFound) {
		t.Error("IsConflict(ErrNotFound) should be false")
	}
}

func TestIsUnauthorized(t *testing.T) {
	if !IsUnauthorized(ErrUnauthorized) {
		t.Error("IsUnauthorized(ErrUnauthorized) should be true")
	}
	if IsUnauthorized(ErrNotFound) {
		t.Error("IsUnauthorized(ErrNotFound) should be false")
	}
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
