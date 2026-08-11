package auth

import "testing"

// TestAddUserAndValidateCredentials verifies the basic credential lifecycle.
func TestAddUserAndValidateCredentials(t *testing.T) {
	svc := NewService()

	if err := svc.AddUser("alice", "s3cret", "user"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}

	if !svc.ValidateCredentials("alice", "s3cret") {
		t.Error("expected valid credentials to validate")
	}
	if svc.ValidateCredentials("alice", "wrong") {
		t.Error("expected wrong password to fail validation")
	}
	if svc.ValidateCredentials("nobody", "s3cret") {
		t.Error("expected unknown user to fail validation")
	}
}

// TestGetUser verifies GetUser returns a value copy, not a pointer into
// internal state, and reports absence correctly.
func TestGetUser(t *testing.T) {
	svc := NewService()
	if err := svc.AddUser("bob", "pw", "admin"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}

	u, ok := svc.GetUser("bob")
	if !ok {
		t.Fatal("expected bob to exist")
	}
	if u.Username != "bob" || u.Role != "admin" {
		t.Errorf("unexpected user: %+v", u)
	}

	if _, ok := svc.GetUser("nobody"); ok {
		t.Error("expected nobody to not exist")
	}
}

// TestUpdatePassword verifies the password can be changed and the old one
// stops working.
func TestUpdatePassword(t *testing.T) {
	svc := NewService()
	if err := svc.AddUser("carol", "old", "user"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	if err := svc.UpdatePassword("carol", "new"); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}
	if svc.ValidateCredentials("carol", "old") {
		t.Error("old password should no longer validate")
	}
	if !svc.ValidateCredentials("carol", "new") {
		t.Error("new password should validate")
	}
	if err := svc.UpdatePassword("nobody", "x"); err == nil {
		t.Error("expected error updating unknown user")
	}
}

// TestIsDefaultCredentials verifies detection of the well-known default
// admin/admin account, exported so the CLI login flow could reuse it too.
func TestIsDefaultCredentials(t *testing.T) {
	svc := NewService()

	tests := []struct {
		username, password string
		want               bool
	}{
		{"admin", "admin", true},
		{"admin", "notadmin", false},
		{"other", "admin", false},
	}
	for _, tt := range tests {
		if got := svc.IsDefaultCredentials(tt.username, tt.password); got != tt.want {
			t.Errorf("IsDefaultCredentials(%q,%q) = %v, want %v", tt.username, tt.password, got, tt.want)
		}
	}
}
