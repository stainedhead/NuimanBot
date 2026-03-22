package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/domain"
)

// --- Task 3.1: requireRole middleware tests ---

// TestRequireRoleUnauthenticated verifies that unauthenticated requests receive 401.
func TestRequireRoleUnauthenticated(t *testing.T) {
	server := NewServer(":0")

	handler := server.requireRole(domain.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated request, got %d", w.Code)
	}
}

// TestRequireRoleNonAdminForbidden verifies non-admin authenticated session receives 403.
func TestRequireRoleNonAdminForbidden(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	// Create a non-admin user session
	if err := auth.AddUser("regularuser", "password", "user"); err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}
	sessionID := auth.CreateSession("regularuser", "user")

	handler := server.requireRole(domain.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	req.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: sessionID,
	})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-admin user, got %d", w.Code)
	}
}

// TestRequireRoleAdminAllowed verifies admin session passes through.
func TestRequireRoleAdminAllowed(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	if err := auth.AddUser("admin", "password", "admin"); err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}
	sessionID := auth.CreateSession("admin", "admin")

	handler := server.requireRole(domain.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	req.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: sessionID,
	})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for admin user, got %d", w.Code)
	}
}

// TestRequireRoleForbiddenBodyDoesNotLeakRoles verifies the 403 body does not disclose role details.
func TestRequireRoleForbiddenBodyDoesNotLeakRoles(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	if err := auth.AddUser("regularuser", "password", "user"); err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}
	sessionID := auth.CreateSession("regularuser", "user")

	handler := server.requireRole(domain.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	req.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: sessionID,
	})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	// Body must not reveal which role is required
	if strings.Contains(strings.ToLower(body), "admin") {
		t.Errorf("403 response body must not leak role name; got: %s", body)
	}
}

// --- Task 3.2: Login rate limiter tests ---

// TestLoginRateLimitAfterFiveFailures verifies 429 is returned after 5 failed attempts.
func TestLoginRateLimitAfterFiveFailures(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	if err := auth.AddUser("admin", "correctpassword", "admin"); err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}

	// Drain all 5 tokens directly via the rate limiter store to avoid slow
	// password-hashing overhead in handleLogin. The bucket is exhausted
	// instantly; no credential verification occurs.
	const clientIP = "192.0.2.1"
	for i := 0; i < loginRateLimitCapacity; i++ {
		server.loginLimiter.allow(clientIP)
	}

	// Next attempt should be rate limited — bucket is empty.
	csrfToken := auth.GenerateCSRFToken()
	form := url.Values{}
	form.Add("username", "admin")
	form.Add("password", "wrongpassword")
	form.Add("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = clientIP + ":12345"
	w := httptest.NewRecorder()

	server.handleLogin(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 after exhausting rate limit, got %d", w.Code)
	}
}

// TestLoginRateLimitDifferentIPsAreIndependent verifies different IPs have independent buckets.
func TestLoginRateLimitDifferentIPsAreIndependent(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	if err := auth.AddUser("admin", "correctpassword", "admin"); err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}

	// Exhaust rate limit for IP1 directly via the store (avoids slow password hashing).
	for i := 0; i < loginRateLimitCapacity; i++ {
		server.loginLimiter.allow("192.0.2.1")
	}

	// IP2 should still be able to attempt login (not rate limited)
	csrfToken := auth.GenerateCSRFToken()
	form := url.Values{}
	form.Add("username", "admin")
	form.Add("password", "wrongpassword")
	form.Add("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "10.0.0.2:12345"
	w := httptest.NewRecorder()

	server.handleLogin(w, req)

	// Should return 401 (failed credentials), NOT 429
	if w.Code == http.StatusTooManyRequests {
		t.Error("different IP should not be rate limited by another IP's failures")
	}
}

// TestLoginRateLimitSuccessfulLoginResetsCounter verifies a successful login resets the rate limit bucket.
func TestLoginRateLimitSuccessfulLoginResetsCounter(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	if err := auth.AddUser("admin", "correctpassword", "admin"); err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}

	// Drain 4 tokens directly (avoids slow password hashing for failed attempts).
	for i := 0; i < 4; i++ {
		server.loginLimiter.allow("192.0.2.3")
	}

	// Successful login should reset counter
	csrfToken := auth.GenerateCSRFToken()
	form := url.Values{}
	form.Add("username", "admin")
	form.Add("password", "correctpassword")
	form.Add("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "192.0.2.3:12345"
	w := httptest.NewRecorder()

	server.handleLogin(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect after successful login, got %d", w.Code)
	}

	// Now attempt login again from same IP — should not be rate limited (counter was reset)
	// Log out first
	auth.DestroySession(w.Result().Cookies()[0].Value)

	csrfToken2 := auth.GenerateCSRFToken()
	form2 := url.Values{}
	form2.Add("username", "admin")
	form2.Add("password", "wrongpassword")
	form2.Add("csrf_token", csrfToken2)

	req2 := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.RemoteAddr = "192.0.2.3:12345"
	w2 := httptest.NewRecorder()

	server.handleLogin(w2, req2)

	// Should return 401 (bad creds) not 429 (rate limited)
	if w2.Code == http.StatusTooManyRequests {
		t.Error("successful login should reset rate limit counter, but got 429 on next attempt")
	}
}

// --- Task 3.3: Input sanitization tests ---

// TestSanitizedFormValueStripsInjectionPatterns verifies injection patterns in form values are sanitized.
func TestSanitizedFormValueStripsInjectionPatterns(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
	r.Form = url.Values{}
	r.Form.Set("username", "admin; DROP TABLE users")
	r.Form.Set("safe", "normalvalue")

	sanitized := sanitizedFormValue(r, "username")
	// The value "admin; DROP TABLE users" contains shell metacharacter ';'
	// sanitizedFormValue returns empty string for detected injection
	if sanitized == "admin; DROP TABLE users" {
		t.Error("expected injection pattern to be sanitized, but raw value was returned")
	}

	// Safe value should pass through trimmed
	safe := sanitizedFormValue(r, "safe")
	if safe != "normalvalue" {
		t.Errorf("expected 'normalvalue', got %q", safe)
	}
}

// TestSanitizedFormValueHandlesMissingKey verifies missing keys return empty string.
func TestSanitizedFormValueHandlesMissingKey(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
	r.Form = url.Values{}

	result := sanitizedFormValue(r, "nonexistent")
	if result != "" {
		t.Errorf("expected empty string for missing key, got %q", result)
	}
}

// --- Task 3.4: Default credential rotation tests ---

// TestDefaultAdminLoginRedirectsToChangePassword verifies admin/admin login redirects to change-password.
func TestDefaultAdminLoginRedirectsToChangePassword(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	// Set up default admin credentials
	if err := auth.AddUser("admin", "admin", "admin"); err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}

	csrfToken := auth.GenerateCSRFToken()
	form := url.Values{}
	form.Add("username", "admin")
	form.Add("password", "admin")
	form.Add("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()

	server.handleLogin(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if location != "/admin/change-password" {
		t.Errorf("expected redirect to /admin/change-password, got %s", location)
	}
}

// TestAdminRoutesBlockedWithDefaultCredentials verifies admin routes redirect when default password active.
func TestAdminRoutesBlockedWithDefaultCredentials(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	if err := auth.AddUser("admin", "admin", "admin"); err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}

	// Log in with default credentials
	csrfToken := auth.GenerateCSRFToken()
	form := url.Values{}
	form.Add("username", "admin")
	form.Add("password", "admin")
	form.Add("csrf_token", csrfToken)

	loginReq := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginReq.RemoteAddr = "127.0.0.1:12345"
	loginW := httptest.NewRecorder()
	server.handleLogin(loginW, loginReq)

	// Extract session cookie
	var sessionCookie *http.Cookie
	for _, c := range loginW.Result().Cookies() {
		if c.Name == sessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected session cookie after login")
	}

	// Accessing dashboard should redirect to change-password
	handler := server.requirePasswordChange(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	req.AddCookie(sessionCookie)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect while default password active, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if location != "/admin/change-password" {
		t.Errorf("expected redirect to /admin/change-password, got %s", location)
	}
}

// TestAdminRoutesAllowedAfterPasswordChange verifies admin routes work after password changed.
func TestAdminRoutesAllowedAfterPasswordChange(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	if err := auth.AddUser("admin", "newstrongpassword", "admin"); err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}

	// Log in with non-default credentials
	csrfToken := auth.GenerateCSRFToken()
	form := url.Values{}
	form.Add("username", "admin")
	form.Add("password", "newstrongpassword")
	form.Add("csrf_token", csrfToken)

	loginReq := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginReq.RemoteAddr = "127.0.0.1:12345"
	loginW := httptest.NewRecorder()
	server.handleLogin(loginW, loginReq)

	// Extract session cookie
	var sessionCookie *http.Cookie
	for _, c := range loginW.Result().Cookies() {
		if c.Name == sessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected session cookie after login")
	}

	// Accessing dashboard should NOT redirect (no force-change flag)
	handler := server.requirePasswordChange(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	req.AddCookie(sessionCookie)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 after password changed, got %d", w.Code)
	}
}

// --- R-05: loginRateLimiterStore idle eviction tests ---

// TestLoginRateLimiterStoreEvictIdleZeroDuration verifies that evictIdle(0) empties the buckets map.
func TestLoginRateLimiterStoreEvictIdleZeroDuration(t *testing.T) {
	store := newLoginRateLimiterStore()

	// Populate some entries.
	store.allow("192.0.2.1")
	store.allow("192.0.2.2")
	store.allow("192.0.2.3")

	store.mu.Lock()
	count := len(store.buckets)
	store.mu.Unlock()
	if count != 3 {
		t.Fatalf("expected 3 entries before eviction, got %d", count)
	}

	// evictIdle(0) should evict everything because all entries were last used before now+0.
	store.evictIdle(0)

	store.mu.Lock()
	count = len(store.buckets)
	store.mu.Unlock()
	if count != 0 {
		t.Errorf("expected 0 entries after evictIdle(0), got %d", count)
	}
}

// TestLoginRateLimiterStoreEvictIdleRetainsRecent verifies that evictIdle keeps recently-used IPs
// and removes IPs not used within the idle window.
func TestLoginRateLimiterStoreEvictIdleRetainsRecent(t *testing.T) {
	store := newLoginRateLimiterStore()

	// Insert an "old" entry by directly manipulating lastUsed.
	store.allow("old-ip")
	store.mu.Lock()
	store.buckets["old-ip"].lastUsed = time.Now().Add(-2 * time.Hour)
	store.mu.Unlock()

	// Insert a recent entry.
	store.allow("new-ip")

	// Evict entries idle for more than 1 hour.
	store.evictIdle(1 * time.Hour)

	store.mu.Lock()
	_, oldExists := store.buckets["old-ip"]
	_, newExists := store.buckets["new-ip"]
	store.mu.Unlock()

	if oldExists {
		t.Error("expected old-ip (last used 2h ago) to be evicted, but it still exists")
	}
	if !newExists {
		t.Error("expected new-ip (recently used) to survive eviction, but it was removed")
	}
}

// TestLoginRateLimiterStoreBackgroundGoroutineStarted verifies that newLoginRateLimiterStore
// starts exactly one background goroutine.
func TestLoginRateLimiterStoreBackgroundGoroutineStarted(t *testing.T) {
	before := runtime.NumGoroutine()
	_ = newLoginRateLimiterStore()

	// Poll for up to 200ms for the background goroutine to be scheduled.
	// The goroutine is started with `go func()`, so it may not appear instantly.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() > before {
			return // goroutine started — test passes
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Errorf("expected goroutine count to increase by at least 1 after newLoginRateLimiterStore(), before=%d after=%d",
		before, runtime.NumGoroutine())
}

// TestLoginRateLimiterStoreAllowBlockBehaviorUnchanged verifies that the existing
// allow/block behavior is unchanged after adding ipBucket.
func TestLoginRateLimiterStoreAllowBlockBehaviorUnchanged(t *testing.T) {
	store := newLoginRateLimiterStore()
	ip := "10.0.0.1"

	// Should allow up to loginRateLimitCapacity attempts.
	for i := 0; i < loginRateLimitCapacity; i++ {
		if !store.allow(ip) {
			t.Fatalf("expected allow=true on attempt %d", i+1)
		}
	}

	// The next attempt should be blocked.
	if store.allow(ip) {
		t.Error("expected allow=false after exhausting the rate limit capacity")
	}
}
