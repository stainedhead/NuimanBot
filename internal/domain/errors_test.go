package domain_test

import (
	"errors"
	"testing"

	"nuimanbot/internal/domain"
)

func TestErrorCategory_String(t *testing.T) {
	tests := []struct {
		category domain.ErrorCategory
		want     string
	}{
		{domain.ErrorCategoryUser, "user_error"},
		{domain.ErrorCategorySystem, "system_error"},
		{domain.ErrorCategoryExternal, "external_error"},
		{domain.ErrorCategoryAuth, "auth_error"},
	}

	for _, tt := range tests {
		if got := string(tt.category); got != tt.want {
			t.Errorf("ErrorCategory.String() = %v, want %v", got, tt.want)
		}
	}
}

func TestCategorizedError_Error(t *testing.T) {
	err := &domain.CategorizedError{
		Category: domain.ErrorCategoryUser,
		Code:     "INPUT_TOO_LONG",
		Message:  "input exceeds maximum length",
	}

	expected := "[user_error] INPUT_TOO_LONG: input exceeds maximum length"
	if got := err.Error(); got != expected {
		t.Errorf("CategorizedError.Error() = %q, want %q", got, expected)
	}
}

func TestCategorizedError_Error_WithCause(t *testing.T) {
	cause := errors.New("underlying error")
	err := &domain.CategorizedError{
		Category: domain.ErrorCategorySystem,
		Code:     "DB_FAILURE",
		Message:  "database operation failed",
		Cause:    cause,
	}

	expected := "[system_error] DB_FAILURE: database operation failed: underlying error"
	if got := err.Error(); got != expected {
		t.Errorf("CategorizedError.Error() = %q, want %q", got, expected)
	}
}

func TestCategorizedError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := &domain.CategorizedError{
		Category: domain.ErrorCategorySystem,
		Code:     "TEST_ERROR",
		Message:  "test error",
		Cause:    cause,
	}

	if unwrapped := errors.Unwrap(err); unwrapped != cause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}
}

func TestCategorizedError_Unwrap_NoCause(t *testing.T) {
	err := &domain.CategorizedError{
		Category: domain.ErrorCategoryUser,
		Code:     "TEST_ERROR",
		Message:  "test error",
	}

	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		t.Errorf("Unwrap() = %v, want nil", unwrapped)
	}
}

func TestCategorizedError_Is(t *testing.T) {
	target := domain.ErrInvalidInput
	err := &domain.CategorizedError{
		Category: domain.ErrorCategoryUser,
		Code:     "INVALID_INPUT",
		Message:  "invalid input provided",
		Cause:    target,
	}

	if !errors.Is(err, target) {
		t.Error("errors.Is() should return true for wrapped error")
	}
}

func TestIsUserError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "user error",
			err: &domain.CategorizedError{
				Category: domain.ErrorCategoryUser,
				Code:     "TEST",
			},
			want: true,
		},
		{
			name: "system error",
			err: &domain.CategorizedError{
				Category: domain.ErrorCategorySystem,
				Code:     "TEST",
			},
			want: false,
		},
		{
			name: "non-categorized error",
			err:  errors.New("plain error"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := domain.IsUserError(tt.err); got != tt.want {
				t.Errorf("IsUserError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSystemError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "system error",
			err: &domain.CategorizedError{
				Category: domain.ErrorCategorySystem,
				Code:     "TEST",
			},
			want: true,
		},
		{
			name: "user error",
			err: &domain.CategorizedError{
				Category: domain.ErrorCategoryUser,
				Code:     "TEST",
			},
			want: false,
		},
		{
			name: "non-categorized error",
			err:  errors.New("plain error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := domain.IsSystemError(tt.err); got != tt.want {
				t.Errorf("IsSystemError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetErrorCategory(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want domain.ErrorCategory
		ok   bool
	}{
		{
			name: "categorized error",
			err: &domain.CategorizedError{
				Category: domain.ErrorCategoryExternal,
				Code:     "TEST",
			},
			want: domain.ErrorCategoryExternal,
			ok:   true,
		},
		{
			name: "non-categorized error",
			err:  errors.New("plain error"),
			want: "",
			ok:   false,
		},
		{
			name: "nil error",
			err:  nil,
			want: "",
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := domain.GetErrorCategory(tt.err)
			if ok != tt.ok {
				t.Errorf("GetErrorCategory() ok = %v, want %v", ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("GetErrorCategory() category = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetUserMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "with user message",
			err: &domain.CategorizedError{
				Category:    domain.ErrorCategoryUser,
				Code:        "INPUT_TOO_LONG",
				UserMessage: "Please keep your message under 4096 characters.",
			},
			want: "Please keep your message under 4096 characters.",
		},
		{
			name: "without user message",
			err: &domain.CategorizedError{
				Category: domain.ErrorCategoryUser,
				Code:     "TEST",
				Message:  "technical message",
			},
			want: "technical message",
		},
		{
			name: "non-categorized error",
			err:  errors.New("plain error"),
			want: "An error occurred. Please try again.",
		},
		{
			name: "nil error",
			err:  nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := domain.GetUserMessage(tt.err); got != tt.want {
				t.Errorf("GetUserMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewUserError(t *testing.T) {
	err := domain.NewUserError("TEST_CODE", "test message", "user message")

	if err.Category != domain.ErrorCategoryUser {
		t.Errorf("Category = %v, want %v", err.Category, domain.ErrorCategoryUser)
	}
	if err.Code != "TEST_CODE" {
		t.Errorf("Code = %v, want TEST_CODE", err.Code)
	}
	if err.Message != "test message" {
		t.Errorf("Message = %v, want 'test message'", err.Message)
	}
	if err.UserMessage != "user message" {
		t.Errorf("UserMessage = %v, want 'user message'", err.UserMessage)
	}
}

func TestNewSystemError(t *testing.T) {
	cause := errors.New("underlying cause")
	err := domain.NewSystemError("SYS_ERROR", "system failure", cause)

	if err.Category != domain.ErrorCategorySystem {
		t.Errorf("Category = %v, want %v", err.Category, domain.ErrorCategorySystem)
	}
	if err.Code != "SYS_ERROR" {
		t.Errorf("Code = %v, want SYS_ERROR", err.Code)
	}
	if err.Cause != cause {
		t.Errorf("Cause = %v, want %v", err.Cause, cause)
	}
	if err.UserMessage == "" {
		t.Error("UserMessage should have default value")
	}
}

func TestNewExternalError(t *testing.T) {
	cause := errors.New("API timeout")
	err := domain.NewExternalError("API_TIMEOUT", "external API timeout", cause)

	if err.Category != domain.ErrorCategoryExternal {
		t.Errorf("Category = %v, want %v", err.Category, domain.ErrorCategoryExternal)
	}
	if err.Code != "API_TIMEOUT" {
		t.Errorf("Code = %v, want API_TIMEOUT", err.Code)
	}
}

func TestNewAuthError(t *testing.T) {
	err := domain.NewAuthError("PERMISSION_DENIED", "insufficient permissions", "You don't have permission to access this resource.")

	if err.Category != domain.ErrorCategoryAuth {
		t.Errorf("Category = %v, want %v", err.Category, domain.ErrorCategoryAuth)
	}
	if err.Code != "PERMISSION_DENIED" {
		t.Errorf("Code = %v, want PERMISSION_DENIED", err.Code)
	}
}
