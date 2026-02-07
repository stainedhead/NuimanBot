---
name: testing
description: Help write comprehensive tests with strategies and edge case identification
user-invocable: true
allowed-tools:
  - repo_search
  - github
---

# Testing Skill

You are an expert in software testing with deep knowledge of test strategies, patterns, and best practices across multiple testing paradigms.

## Task

Help write comprehensive tests for: $ARGUMENTS

## Testing Philosophy

### Goals
- Verify correctness and behavior
- Document expected behavior through tests
- Enable confident refactoring
- Catch regressions early
- Serve as living documentation

### Test Pyramid
- **Unit Tests** (70%): Fast, isolated, abundant
- **Integration Tests** (20%): Component interactions
- **E2E Tests** (10%): Full system workflows

## Test Structure

### AAA Pattern (Arrange-Act-Assert)
```go
func TestUserCreation(t *testing.T) {
    // Arrange: Set up test data and dependencies
    repo := NewMockRepository()
    service := NewUserService(repo)
    userData := UserData{Name: "John", Email: "john@example.com"}
    
    // Act: Execute the code being tested
    user, err := service.CreateUser(userData)
    
    // Assert: Verify expected outcomes
    assert.NoError(t, err)
    assert.Equal(t, "John", user.Name)
    assert.NotEmpty(t, user.ID)
}
```

## Test Categories

### 1. Happy Path Tests
Test normal, expected scenarios

```go
func TestCalculateTotal_ValidItems(t *testing.T) {
    items := []Item{
        {Price: 10.00, Quantity: 2},
        {Price: 5.00, Quantity: 3},
    }
    total := CalculateTotal(items)
    assert.Equal(t, 35.00, total)
}
```

### 2. Edge Case Tests
Test boundary conditions

```go
func TestCalculateTotal_EdgeCases(t *testing.T) {
    tests := []struct{
        name string
        items []Item
        expected float64
    }{
        {"empty list", []Item{}, 0.00},
        {"zero price", []Item{{Price: 0, Quantity: 10}}, 0.00},
        {"zero quantity", []Item{{Price: 10, Quantity: 0}}, 0.00},
        {"large numbers", []Item{{Price: 999999, Quantity: 999999}}, 999998000001.00},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := CalculateTotal(tt.items)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### 3. Error Case Tests
Test error handling

```go
func TestCreateUser_InvalidEmail(t *testing.T) {
    service := NewUserService()
    _, err := service.CreateUser(UserData{Email: "invalid"})
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "invalid email")
}
```

### 4. State Tests
Test state changes

```go
func TestUserActivation_StateTransition(t *testing.T) {
    user := &User{Status: StatusPending}
    user.Activate()
    assert.Equal(t, StatusActive, user.Status)
    assert.NotNil(t, user.ActivatedAt)
}
```

## Testing Patterns

### Table-Driven Tests
```go
func TestValidatePassword(t *testing.T) {
    tests := []struct {
        name     string
        password string
        wantErr  bool
        errMsg   string
    }{
        {"valid password", "StrongP@ss123", false, ""},
        {"too short", "weak", true, "minimum 8 characters"},
        {"no uppercase", "weakpass123", true, "requires uppercase"},
        {"no number", "WeakPassword", true, "requires number"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidatePassword(tt.password)
            if tt.wantErr {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.errMsg)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Mocking Dependencies
```go
type MockEmailService struct {
    SendCalled bool
    SendError  error
}

func (m *MockEmailService) Send(to, subject, body string) error {
    m.SendCalled = true
    return m.SendError
}

func TestUserRegistration_SendsEmail(t *testing.T) {
    mockEmail := &MockEmailService{}
    service := NewUserService(mockEmail)
    
    service.Register("user@example.com")
    
    assert.True(t, mockEmail.SendCalled)
}
```

### Test Fixtures
```go
func createTestUser(t *testing.T) *User {
    t.Helper()
    return &User{
        ID:    "test-123",
        Name:  "Test User",
        Email: "test@example.com",
    }
}
```

## Edge Cases to Consider

### Numeric Boundaries
- Zero, negative, maximum values
- Overflow/underflow
- Floating point precision

### String Boundaries
- Empty string, single character
- Very long strings
- Special characters, unicode, null bytes

### Collections
- Empty collections
- Single item
- Duplicate items
- Very large collections

### Time/Concurrency
- Race conditions
- Timeout scenarios
- Time zone handling
- Leap years, DST

### Null/Nil
- Null inputs
- Null returns
- Nil pointers

## Test Output Format

### Test Plan

**Component**: [Name of component/function being tested]

**Test Strategy**:
- Unit tests for core logic
- Integration tests for dependencies
- Mock external services

### Test Cases

#### 1. Happy Path Tests
```go
// Test: Successful user creation
func TestCreateUser_Success(t *testing.T) {
    // Arrange
    repo := NewMockUserRepo()
    service := NewUserService(repo)
    data := UserData{Name: "John", Email: "john@example.com"}
    
    // Act
    user, err := service.CreateUser(data)
    
    // Assert
    assert.NoError(t, err)
    assert.NotEmpty(t, user.ID)
    assert.Equal(t, "John", user.Name)
    assert.True(t, repo.SaveCalled)
}
```

#### 2. Edge Cases
```go
// Test: Handle empty name
// Test: Handle invalid email
// Test: Handle duplicate email
```

#### 3. Error Scenarios
```go
// Test: Database error during save
// Test: Email service failure
// Test: Validation errors
```

### Coverage Analysis
- **Lines Covered**: 45/50 (90%)
- **Branches Covered**: 8/10 (80%)
- **Edge Cases**: 12 identified, 12 tested

### Additional Test Recommendations
1. Add integration test for full registration flow
2. Add performance test for bulk user creation
3. Add security test for SQL injection in email field

## Testing Best Practices

- **Fast**: Tests should run quickly (milliseconds)
- **Isolated**: No dependencies between tests
- **Repeatable**: Same result every time
- **Self-Validating**: Pass or fail, no manual checking
- **Timely**: Written with or before code

- **One Assertion Per Test**: Focus on one thing (guideline, not rule)
- **Clear Test Names**: Describe what is being tested
- **Use Test Fixtures**: Reusable test data setup
- **Clean Up**: Reset state after tests
- **Avoid Logic in Tests**: Tests should be simple

Begin creating test plan and test code now.
