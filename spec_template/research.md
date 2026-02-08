# [Feature Name] - Research

**Created:** [YYYY-MM-DD]
**Source:** `[path/to/PRD-or-research-doc]` (if applicable)
**Status:** [In Progress | Complete]

---

## Overview

[Brief summary of what this research document covers and its purpose]

**Research Questions:**
1. [Key question 1 this research aims to answer]
2. [Key question 2 this research aims to answer]
3. [Key question 3 this research aims to answer]

**For full details, see the source PRD:** `[path/to/source-document]` (if applicable)

---

## Table of Contents

1. [Industry Standards](#industry-standards)
2. [Existing Implementations](#existing-implementations)
3. [API Documentation](#api-documentation)
4. [Code Examples](#code-examples)
5. [Performance Benchmarks](#performance-benchmarks)
6. [Security Considerations](#security-considerations)
7. [Third-Party Libraries](#third-party-libraries)
8. [Best Practices](#best-practices)

---

## Industry Standards

### [Standard/Specification Name]

**Source:** [URL or reference]
**Version:** [Version number]
**Relevance:** [Why this standard matters for our implementation]

**Key Points:**
- [Key point 1]
- [Key point 2]
- [Key point 3]

**Compliance Requirements:**
- [ ] [Requirement 1]
- [ ] [Requirement 2]

**Example:**
```
[Code example or specification snippet]
```

---

## Existing Implementations

### Implementation 1: [Name]

**Source:** [GitHub URL or reference]
**Language:** [Programming language]
**License:** [License type]

**What We Can Learn:**
- [Learning 1]
- [Learning 2]

**Approach:**
[Description of their approach]

**Pros:**
- [Pro 1]
- [Pro 2]

**Cons:**
- [Con 1]
- [Con 2]

**Code Example:**
```go
// Relevant code snippet
```

**Applicability to Our Project:**
[How this relates to our implementation]

---

### Implementation 2: [Name]

[Repeat structure from Implementation 1]

---

## API Documentation

### [API/Library Name]

**Documentation:** [URL]
**Version:** [Version we'll use]
**Installation:** `go get [package]`

**Key Functions/Methods:**

#### Function 1: `FunctionName()`

**Signature:**
```go
func FunctionName(param1 type1, param2 type2) (returnType, error)
```

**Purpose:** [What it does]

**Parameters:**
- `param1` - [Description]
- `param2` - [Description]

**Returns:**
- [Return value description]
- `error` - [Error conditions]

**Example:**
```go
result, err := FunctionName(arg1, arg2)
if err != nil {
    return err
}
// Use result
```

**Gotchas:**
- [Gotcha 1]
- [Gotcha 2]

---

## Code Examples

### Example 1: [Use Case]

**Scenario:** [What this example demonstrates]

**Source:** [Where this example came from]

**Code:**
```go
package main

import (
    // imports
)

func example() {
    // Example implementation
}
```

**Explanation:**
[Step-by-step explanation of how this works]

**Lessons:**
- [Lesson 1]
- [Lesson 2]

---

### Example 2: [Use Case]

[Repeat structure from Example 1]

---

## Performance Benchmarks

### Benchmark 1: [Operation]

**Source:** [Where this benchmark came from]

**Test Conditions:**
- Dataset size: [Size]
- Hardware: [Specs]
- Concurrency: [Number of goroutines]

**Results:**
```
Benchmark[Name]-8    [operations]    [ns/op]    [MB/s]    [allocs/op]
```

**Analysis:**
[What these results tell us]

**Implications for Our Design:**
[How this affects our implementation decisions]

---

### Benchmark 2: [Operation]

[Repeat structure from Benchmark 1]

---

## Security Considerations

### Threat 1: [Threat Name]

**Description:** [What is the threat]

**Attack Vector:** [How it could be exploited]

**Likelihood:** [High | Medium | Low]

**Impact:** [High | Medium | Low]

**Mitigation:**
- [Mitigation strategy 1]
- [Mitigation strategy 2]

**Implementation:**
```go
// Example of secure implementation
```

**References:**
- [OWASP reference]
- [CVE reference]

---

### Threat 2: [Threat Name]

[Repeat structure from Threat 1]

---

## Third-Party Libraries

### Library 1: [Name]

**Package:** `[import path]`
**Documentation:** [URL]
**License:** [License type]
**Stars:** [GitHub stars]
**Last Updated:** [Date]

**What It Provides:**
- [Feature 1]
- [Feature 2]

**Why Consider:**
- [Reason 1]
- [Reason 2]

**Concerns:**
- [Concern 1]
- [Concern 2]

**Dependencies:**
- [Dependency 1]
- [Dependency 2]

**Decision:** [Use | Don't Use | Consider Alternative]

**Rationale:** [Why we made this decision]

---

### Library 2: [Name]

[Repeat structure from Library 1]

---

## Best Practices

### Best Practice 1: [Practice Name]

**Source:** [Where this comes from - blog, book, official docs]

**Description:**
[What the best practice is]

**Rationale:**
[Why this is considered best practice]

**Example:**
```go
// Good example
```

```go
// Bad example (anti-pattern)
```

**Applicability:**
[How we'll apply this in our implementation]

---

### Best Practice 2: [Practice Name]

[Repeat structure from Best Practice 1]

---

## Design Patterns

### Pattern 1: [Pattern Name]

**Type:** [Creational | Structural | Behavioral]

**Problem:** [What problem this pattern solves]

**Solution:** [How the pattern solves it]

**Structure:**
```go
// Pattern structure in Go
```

**When to Use:**
- [Use case 1]
- [Use case 2]

**When NOT to Use:**
- [Anti-use case 1]
- [Anti-use case 2]

**Applicability to Our Feature:**
[Whether and how we'll use this pattern]

---

## Comparison Matrix

### [Comparison of Options/Approaches]

| Criteria | Option A | Option B | Option C |
|----------|----------|----------|----------|
| Performance | [Rating] | [Rating] | [Rating] |
| Complexity | [Rating] | [Rating] | [Rating] |
| Maintainability | [Rating] | [Rating] | [Rating] |
| Community Support | [Rating] | [Rating] | [Rating] |
| License | [Type] | [Type] | [Type] |

**Recommendation:** [Which option we should choose]

**Rationale:** [Why this is the best choice]

---

## Open Questions

### Question 1: [Question]

**Context:** [Why this question matters]

**Options:**
- Option A: [Description] - Pros: [Pros] - Cons: [Cons]
- Option B: [Description] - Pros: [Pros] - Cons: [Cons]

**Research Needed:**
- [Further research item 1]
- [Further research item 2]

**Decision:** [TBD | Decided on Option X]

**Rationale:** [If decided, why]

---

## References

### Documentation
- [Documentation link 1]
- [Documentation link 2]

### Articles/Blog Posts
- [Article 1]
- [Article 2]

### GitHub Repositories
- [Repo 1]
- [Repo 2]

### Books
- [Book 1]
- [Book 2]

### Specifications
- [Spec 1]
- [Spec 2]

---

## Research Summary

**Key Findings:**
1. [Finding 1]
2. [Finding 2]
3. [Finding 3]

**Decisions Made:**
1. [Decision 1] - [Rationale]
2. [Decision 2] - [Rationale]

**Next Steps:**
1. [Action item 1]
2. [Action item 2]

**Open Items:**
- [Item 1]
- [Item 2]
