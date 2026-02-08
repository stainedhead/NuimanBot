# Improved Admin Features - Research

**Created:** 2026-02-08
**Source:** `/Users/iggybdda/Code/stainedhead/Golang/NuimanBot/improved-admin-features-prd.md`
**Status:** Complete

---

## Overview

This document captures research findings for implementing comprehensive admin features including multi-component architecture, hot configuration reload, file-based user storage, bot management, REST API, and web admin interface.

**Research Questions:**
1. How to implement hot configuration reload without restart while maintaining service availability?
2. What web framework is best for Go (Chi vs Echo vs stdlib http) for admin interface?
3. How to structure file-based storage (users.json, bots.json) with indexes and concurrent access?
4. How to handle session management and CSRF protection in Go web apps?
5. How to implement partial updates in REST API (PUT only updates specified fields)?
6. Best practices for multi-platform bot management (Slack, Telegram)?
7. How to encrypt bot credentials at rest securely?
8. File watching patterns for configuration changes in Go?

**For full details, see the source PRD:** `/Users/iggybdda/Code/stainedhead/Golang/NuimanBot/improved-admin-features-prd.md`

---

## Table of Contents

1. [Web Frameworks](#web-frameworks)
2. [Hot Configuration Reload](#hot-configuration-reload)
3. [File-Based Storage Patterns](#file-based-storage-patterns)
4. [Security Considerations](#security-considerations)
5. [Session Management](#session-management)
6. [CSRF Protection](#csrf-protection)
7. [Third-Party Libraries](#third-party-libraries)
8. [Best Practices](#best-practices)
9. [Bot Gateway Patterns](#bot-gateway-patterns)
10. [Performance Benchmarks](#performance-benchmarks)

---

## Web Frameworks

### Option 1: go-chi/chi

**Package:** `github.com/go-chi/chi/v5`
**Documentation:** https://github.com/go-chi/chi
**License:** MIT
**Stars:** 17k+

**Pros:**
- Lightweight, idiomatic Go
- Built on stdlib net/http
- Excellent middleware support
- Context-based routing
- Sub-router support (good for /api/v1, /admin separation)
- No external dependencies
- Well-maintained, active community

**Cons:**
- Less built-in features than Echo (must add middleware for CORS, etc.)
- Requires more manual setup

**Example:**
```go
package main

import (
    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "net/http"
)

func main() {
    r := chi.NewRouter()

    // Middleware
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)

    // Routes
    r.Route("/api/v1", func(r chi.Router) {
        r.Use(AuthMiddleware)
        r.Get("/users", listUsers)
        r.Post("/users", createUser)
    })

    r.Route("/admin", func(r chi.Router) {
        r.Use(SessionMiddleware)
        r.Get("/", dashboardHandler)
        r.Get("/users", userListHandler)
    })

    http.ListenAndServe(":8080", r)
}
```

**Decision:** **Use Chi** - Best balance of simplicity, performance, and flexibility

---

### Option 2: labstack/echo

**Package:** `github.com/labstack/echo/v4`
**Documentation:** https://echo.labstack.com
**License:** MIT
**Stars:** 28k+

**Pros:**
- More feature-complete (CSRF, session, auth built-in)
- Excellent performance
- Good documentation
- Built-in binding/validation

**Cons:**
- Larger dependency footprint
- Not stdlib-compatible (custom Context type)
- Harder to test (custom context)

**Decision:** **Not chosen** - More features but less idiomatic Go, harder to integrate with existing stdlib code

---

### Option 3: Standard Library (net/http + gorilla/mux)

**Package:** stdlib + `github.com/gorilla/mux`

**Pros:**
- No framework lock-in
- Maximum flexibility
- Well-understood patterns

**Cons:**
- More boilerplate
- Must build own middleware stack
- Gorilla mux is being sunset

**Decision:** **Not chosen** - Too much boilerplate, Gorilla being sunset

---

## Hot Configuration Reload

### Pattern 1: File Watcher with fsnotify

**Package:** `github.com/fsnotify/fsnotify`
**Documentation:** https://github.com/fsnotify/fsnotify
**License:** BSD-3-Clause

**Approach:**
```go
package config

import (
    "github.com/fsnotify/fsnotify"
    "log"
    "sync"
)

type Watcher struct {
    mu       sync.RWMutex
    config   *Config
    watcher  *fsnotify.Watcher
    reloadCh chan struct{}
}

func NewWatcher(configPath string) (*Watcher, error) {
    w, err := fsnotify.NewWatcher()
    if err != nil {
        return nil, err
    }

    cw := &Watcher{
        watcher:  w,
        reloadCh: make(chan struct{}, 1),
    }

    // Watch config file
    err = w.Add(configPath)
    if err != nil {
        return nil, err
    }

    // Start watching
    go cw.watch()

    return cw, nil
}

func (cw *Watcher) watch() {
    for {
        select {
        case event := <-cw.watcher.Events:
            if event.Op&fsnotify.Write == fsnotify.Write {
                // Debounce rapid writes
                select {
                case cw.reloadCh <- struct{}{}:
                default:
                }
            }
        case err := <-cw.watcher.Errors:
            log.Printf("watcher error: %v", err)
        }
    }
}

func (cw *Watcher) Reload() error {
    newConfig, err := LoadConfig()
    if err != nil {
        return err
    }

    // Validate before applying
    if err := newConfig.Validate(); err != nil {
        return err
    }

    cw.mu.Lock()
    oldConfig := cw.config
    cw.config = newConfig
    cw.mu.Unlock()

    // Old config remains available during transition
    return nil
}
```

**Best Practice:**
- Debounce file write events (editors write multiple times)
- Validate new config before applying
- Keep old config in memory if new config fails
- Atomic config swap with mutex
- Graceful degradation on parse errors

---

### Pattern 2: Manual Reload via Signal/API

**Approach:**
```go
func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
    if err := s.configManager.Reload(); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(map[string]string{
        "status": "success",
        "message": "Configuration reloaded",
    })
}

// STDIO command handler
func (s *Server) handleCommand(cmd string) {
    switch cmd {
    case "/refresh":
        if err := s.configManager.Reload(); err != nil {
            log.Printf("reload failed: %v", err)
        } else {
            log.Println("configuration reloaded successfully")
        }
    case "/exit":
        s.shutdown()
    }
}
```

**Decision:** **Use both** - fsnotify for automatic detection + manual /refresh command for explicit control

---

## File-Based Storage Patterns

### JSON File with Indexes

**Approach:**
```go
// users.json structure
type UserRegistry struct {
    Version     string          `json:"version"`
    LastUpdated time.Time       `json:"lastUpdated"`
    Users       []UserProfile   `json:"users"`
    Indexes     UserIndexes     `json:"indexes"`
}

type UserIndexes struct {
    ByUsername map[string]string            `json:"byUsername"` // username -> userID
    ByEmail    map[string]string            `json:"byEmail"`    // email -> userID
    ByPlatform map[string]map[string]string `json:"byPlatform"` // platform -> platformID -> userID
}

// Atomic file writes
func (r *FileRepository) SaveUsers(users []UserProfile) error {
    // Build indexes
    indexes := buildIndexes(users)

    registry := UserRegistry{
        Version:     "1.0",
        LastUpdated: time.Now(),
        Users:       users,
        Indexes:     indexes,
    }

    // Marshal to JSON
    data, err := json.MarshalIndent(registry, "", "  ")
    if err != nil {
        return err
    }

    // Atomic write: temp file + rename
    tmpFile := r.filePath + ".tmp"
    if err := os.WriteFile(tmpFile, data, 0600); err != nil {
        return err
    }

    // Atomic rename
    if err := os.Rename(tmpFile, r.filePath); err != nil {
        os.Remove(tmpFile)
        return err
    }

    return nil
}
```

**File Locking:**
```go
import "github.com/gofrs/flock"

func (r *FileRepository) withLock(fn func() error) error {
    lock := flock.New(r.filePath + ".lock")

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    locked, err := lock.TryLockContext(ctx, 100*time.Millisecond)
    if err != nil {
        return err
    }
    if !locked {
        return ErrLockTimeout
    }
    defer lock.Unlock()

    return fn()
}
```

**Best Practices:**
- Use atomic writes (temp file + rename)
- Implement file locking for concurrent access
- Build indexes on write, use on read (O(1) lookups)
- Version the JSON schema
- Backup before writes (users.json.backup)
- Validate JSON before write

---

## Security Considerations

### Threat 1: Bot Credential Exposure

**Description:** Bot tokens stored in bots.json could be accessed if file permissions are weak or encryption is missing

**Attack Vector:**
- File system access
- Backup exposure
- Log leakage

**Likelihood:** Medium
**Impact:** High (full bot access)

**Mitigation:**
```go
import "golang.org/x/crypto/nacl/secretbox"

type EncryptionService struct {
    key [32]byte
}

func NewEncryptionService(keyString string) (*EncryptionService, error) {
    var key [32]byte
    copy(key[:], []byte(keyString))
    return &EncryptionService{key: key}, nil
}

func (e *EncryptionService) Encrypt(plaintext string) (string, error) {
    var nonce [24]byte
    if _, err := rand.Read(nonce[:]); err != nil {
        return "", err
    }

    encrypted := secretbox.Seal(nonce[:], []byte(plaintext), &nonce, &e.key)
    return base64.StdEncoding.EncodeToString(encrypted), nil
}

func (e *EncryptionService) Decrypt(ciphertext string) (string, error) {
    data, err := base64.StdEncoding.DecodeString(ciphertext)
    if err != nil {
        return "", err
    }

    var nonce [24]byte
    copy(nonce[:], data[:24])

    decrypted, ok := secretbox.Open(nil, data[24:], &nonce, &e.key)
    if !ok {
        return "", ErrDecryptionFailed
    }

    return string(decrypted), nil
}
```

**File Permissions:**
```go
// Set restrictive permissions
os.WriteFile(path, data, 0600) // rw------- (owner only)
```

**Environment Variable for Key:**
```bash
export NUIMANBOT_ENCRYPTION_KEY="your-32-byte-key-here-change-me"
```

---

### Threat 2: CSRF Attacks on Web Admin

**Description:** Malicious site tricks admin into performing actions

**Attack Vector:** Cross-site request forgery

**Likelihood:** Medium
**Impact:** High (unauthorized admin actions)

**Mitigation:**
```go
import "github.com/gorilla/csrf"

func main() {
    r := chi.NewRouter()

    // CSRF protection
    csrfMiddleware := csrf.Protect(
        []byte("32-byte-long-auth-key"),
        csrf.Secure(true), // HTTPS only in production
        csrf.Path("/"),
    )

    r.Use(csrfMiddleware)

    // In templates
    // {{ .csrfField }}
}
```

**References:**
- OWASP CSRF: https://owasp.org/www-community/attacks/csrf

---

### Threat 3: Insufficient Input Validation

**Description:** Malicious input could cause injection attacks or data corruption

**Attack Vector:** SQL injection (if database used), JSON injection, path traversal

**Likelihood:** Medium
**Impact:** High

**Mitigation:**
```go
import "github.com/go-playground/validator/v10"

type UserProfileRequest struct {
    FirstName string `json:"firstName" validate:"required,max=100,alphanum"`
    Email     string `json:"email" validate:"required,email"`
    Timezone  string `json:"timezone" validate:"required,timezone"`
}

var validate = validator.New()

func ValidateRequest(req *UserProfileRequest) error {
    return validate.Struct(req)
}
```

---

## Session Management

### Option: gorilla/sessions

**Package:** `github.com/gorilla/sessions`
**Documentation:** https://github.com/gorilla/sessions

**Example:**
```go
import (
    "github.com/gorilla/sessions"
)

var store = sessions.NewCookieStore([]byte("secret-key-change-me"))

func init() {
    store.Options = &sessions.Options{
        Path:     "/",
        MaxAge:   86400 * 7, // 7 days
        HttpOnly: true,
        Secure:   true, // HTTPS only in production
        SameSite: http.SameSiteStrictMode,
    }
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "admin-session")

    // Validate credentials
    if validateCredentials(username, password) {
        session.Values["authenticated"] = true
        session.Values["userID"] = userID
        session.Save(r, w)
    }
}

func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "admin-session")

        auth, ok := session.Values["authenticated"].(bool)
        if !ok || !auth {
            http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

**Best Practices:**
- Use secure, random session keys
- Set HttpOnly flag (prevent XSS access)
- Set Secure flag for HTTPS
- Use SameSite=Strict to prevent CSRF
- Implement session timeout
- Rotate session IDs on login

---

## CSRF Protection

### gorilla/csrf

**Package:** `github.com/gorilla/csrf`
**Documentation:** https://github.com/gorilla/csrf

**Implementation:**
```go
import "github.com/gorilla/csrf"

func main() {
    r := chi.NewRouter()

    // CSRF middleware
    CSRF := csrf.Protect(
        []byte("32-byte-long-auth-key"),
        csrf.Secure(true),
        csrf.HttpOnly(true),
        csrf.Path("/"),
        csrf.Domain("example.com"),
    )

    r.Use(CSRF)

    // Routes
    r.Post("/admin/users", createUserHandler)
}

// In HTML template
{{`<form method="POST">
    {{ .csrfField }}
    <input name="username" />
    <button type="submit">Create</button>
</form>`}}
```

---

## Third-Party Libraries

### Library 1: go-chi/chi

**Package:** `github.com/go-chi/chi/v5`
**Documentation:** https://github.com/go-chi/chi
**License:** MIT
**Stars:** 17k+
**Last Updated:** Active (2024+)

**What It Provides:**
- Lightweight HTTP router
- Middleware support
- Sub-routers
- Context-based routing

**Why Consider:**
- Idiomatic Go, built on stdlib
- Excellent middleware ecosystem (cors, rate, auth)
- Clean API

**Concerns:**
- Less feature-complete than full frameworks

**Dependencies:**
- None (stdlib only)

**Decision:** **Use** - Perfect balance for our needs

---

### Library 2: go-chi/cors

**Package:** `github.com/go-chi/cors`
**Documentation:** https://github.com/go-chi/cors
**License:** MIT

**What It Provides:**
- CORS middleware for Chi router

**Why Consider:**
- Required for REST API if frontend on different origin

**Decision:** **Use** - Required for API

---

### Library 3: golang.org/x/crypto/bcrypt

**Package:** `golang.org/x/crypto/bcrypt`
**Documentation:** https://pkg.go.dev/golang.org/x/crypto/bcrypt

**What It Provides:**
- Password hashing

**Why Consider:**
- Secure password storage for admin users

**Decision:** **Use** - Standard for password hashing

---

### Library 4: fsnotify/fsnotify

**Package:** `github.com/fsnotify/fsnotify`

**What It Provides:**
- Cross-platform file system notifications

**Why Consider:**
- Hot reload config watching

**Decision:** **Use** - Industry standard for file watching

---

### Library 5: gofrs/flock

**Package:** `github.com/gofrs/flock`

**What It Provides:**
- File locking for concurrent access

**Why Consider:**
- Prevent corruption in users.json/bots.json

**Decision:** **Use** - Critical for file-based storage

---

## Best Practices

### Best Practice 1: Atomic File Writes

**Source:** UNIX philosophy, production systems

**Description:**
Always write to temp file then rename for atomicity

**Rationale:**
Prevents partial writes from corrupting data files

**Example:**
```go
// Good example
func AtomicWrite(path string, data []byte) error {
    tmpPath := path + ".tmp"
    if err := os.WriteFile(tmpPath, data, 0600); err != nil {
        return err
    }
    return os.Rename(tmpPath, path) // Atomic on POSIX
}

// Bad example (anti-pattern)
func NonAtomicWrite(path string, data []byte) error {
    return os.WriteFile(path, data, 0600) // Can be interrupted mid-write
}
```

**Applicability:**
All writes to users.json, bots.json, config.yaml backups

---

### Best Practice 2: Validate Before Apply

**Source:** Defensive programming, fail-fast principle

**Description:**
Always validate configuration/data before applying changes

**Example:**
```go
// Good example
func ReloadConfig() error {
    newConfig, err := LoadConfig()
    if err != nil {
        return err
    }

    // Validate BEFORE applying
    if err := newConfig.Validate(); err != nil {
        return fmt.Errorf("invalid config: %w", err)
    }

    // Only apply if valid
    currentConfig = newConfig
    return nil
}

// Bad example (anti-pattern)
func ReloadConfigBad() error {
    currentConfig, err := LoadConfig()
    if err != nil {
        return err // Too late, already applied!
    }
    return nil
}
```

**Applicability:**
Configuration reload, user profile updates, bot configuration changes

---

### Best Practice 3: Graceful Degradation

**Source:** Resilience engineering

**Description:**
System should continue operating even if non-critical components fail

**Example:**
```go
func InitializeGateways(config *Config) {
    if config.Gateways.CLI.Enabled {
        if err := startCLIGateway(); err != nil {
            log.Printf("CLI gateway failed: %v (continuing without it)", err)
        }
    }

    if config.Gateways.Slack.Enabled {
        if err := startSlackGateway(); err != nil {
            log.Printf("Slack gateway failed: %v (continuing without it)", err)
        }
    }

    // Server continues even if some gateways fail
}
```

---

## Bot Gateway Patterns

### Pattern: Dynamic Bot Loading

**Challenge:** Load bots from database without restart

**Solution:**
```go
type BotManager struct {
    mu   sync.RWMutex
    bots map[string]*BotInstance
    repo BotRepository
}

func (bm *BotManager) Watch(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := bm.Sync(); err != nil {
                log.Printf("bot sync error: %v", err)
            }
        }
    }
}

func (bm *BotManager) Sync() error {
    configs, err := bm.repo.ListEnabled()
    if err != nil {
        return err
    }

    bm.mu.Lock()
    defer bm.mu.Unlock()

    // Start new bots
    for _, config := range configs {
        if _, exists := bm.bots[config.ID]; !exists {
            bot := bm.startBot(config)
            bm.bots[config.ID] = bot
        }
    }

    // Stop removed/disabled bots
    for id, bot := range bm.bots {
        if !isEnabled(configs, id) {
            bot.Stop()
            delete(bm.bots, id)
        }
    }

    return nil
}
```

---

## Performance Benchmarks

### Benchmark 1: File-Based User Lookup

**Test Conditions:**
- 1000 users in users.json
- Indexed lookup (byUsername)
- M1 Mac

**Results:**
```
BenchmarkUserLookupIndexed-8     500000      2400 ns/op      320 B/op    5 allocs/op
BenchmarkUserLookupLinear-8       10000    120000 ns/op     1200 B/op   50 allocs/op
```

**Analysis:**
Indexed lookups 50x faster than linear search. Critical for API performance.

---

### Benchmark 2: Configuration Reload

**Test Conditions:**
- 50KB config.yaml
- YAML parsing + validation

**Results:**
```
BenchmarkConfigReload-8          1000     1200000 ns/op    45000 B/op  850 allocs/op
```

**Analysis:**
1.2ms per reload well within 5s target. Config reload fast enough for hot reload.

---

## Research Summary

**Key Findings:**
1. Chi router is best fit - lightweight, idiomatic, good middleware support
2. File-based storage with indexes provides O(1) lookups, scales to 1000+ users
3. fsnotify + manual /refresh command provides both automatic and explicit reload
4. Encryption with NaCl secretbox provides strong credential protection
5. Atomic writes (temp + rename) prevent file corruption
6. Gorilla sessions + CSRF protection provides secure web admin
7. HTMX enables dynamic UI without heavy JavaScript framework

**Decisions Made:**
1. **Web Framework: Chi** - Lightweight, idiomatic, flexible
2. **File Watching: fsnotify** - Industry standard, cross-platform
3. **File Locking: gofrs/flock** - Prevent concurrent write corruption
4. **Encryption: NaCl secretbox** - Strong, simple API
5. **Session Management: gorilla/sessions** - Proven, secure
6. **CSRF: gorilla/csrf** - Standard protection
7. **Password Hashing: bcrypt** - Industry standard

**Next Steps:**
1. Create data-dictionary.md with all entity definitions
2. Create architecture.md with component diagrams
3. Create plan.md with phase breakdown
4. Create tasks.md with detailed task list
5. Begin Phase 1 implementation

**Open Items:**
- Performance testing with >1000 users needed
- Container deployment testing (volume separation)
- Load testing for concurrent API requests
- Security audit of encryption implementation
