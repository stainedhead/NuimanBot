# [Feature Name] - Task Breakdown

**Feature:** [Feature Name]
**Version:** 1.0
**Created:** [YYYY-MM-DD]
**Status:** [Planning | In Progress | Complete]
**Estimated Total Duration:** [X-Y hours]

---

## Task Organization

Tasks are organized by implementation phase following Clean Architecture layers (bottom-up). Each task includes:
- **ID:** Unique task identifier (e.g., P1.1, P1.2)
- **Phase:** Implementation phase (1-N)
- **Dependencies:** Prerequisites that must complete first
- **Estimated Duration:** Time estimate in hours
- **Priority:** P0 (Critical), P1 (High), P2 (Medium), P3 (Low)
- **Description:** What needs to be done
- **Files to Create/Modify:** Specific files affected
- **Acceptance Criteria:** Definition of done (checkboxes)
- **Verification Commands:** Commands to verify completion

**Development Approach:** TDD + Bottom-Up Clean Architecture
- Red → Green → Refactor for each component
- Domain → Infrastructure → Use Case → Adapter
- Each phase produces working, tested code

---

## Progress Summary

**Overall Progress:** [0/N] tasks complete ([0%])

**By Phase:**
- Phase 1: [0/X] tasks complete
- Phase 2: [0/Y] tasks complete
- Phase 3: [0/Z] tasks complete

**By Priority:**
- P0 (Critical): [0/X] complete
- P1 (High): [0/Y] complete
- P2 (Medium): [0/Z] complete

---

## Phase 1: [Phase Name] ([X-Y hours])

### Task P1.1: [Task Name]

**ID:** P1.1
**Dependencies:** None
**Duration:** [X hours]
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started | 🟡 In Progress | ✅ Complete

**Description:**
[Clear description of what needs to be done]

**Files to Create:**
- `[path/to/file1.go]` - [Purpose]
- `[path/to/file1_test.go]` - [Tests for file1]

**Files to Modify:**
- `[path/to/existing.go]` - [What changes are needed]

**Acceptance Criteria:**
- [ ] [Specific, testable criterion 1]
- [ ] [Specific, testable criterion 2]
- [ ] [Specific, testable criterion 3]
- [ ] Tests written and passing
- [ ] Code coverage >90%
- [ ] Documentation updated

**Implementation Details:**

```go
// Example code structure or pseudocode
package [package]

type [TypeName] struct {
    // fields
}

func [FunctionName]() {
    // implementation
}
```

**Test Cases:**
- [ ] Test case 1: [Description]
- [ ] Test case 2: [Description]
- [ ] Test case 3: [Edge case]

**Verification Commands:**
```bash
# Run tests for this component
go test ./[path/to/package] -v

# Check coverage
go test ./[path/to/package] -cover

# Build to ensure no breaking changes
go build -o bin/nuimanbot ./cmd/nuimanbot
```

---

### Task P1.2: [Task Name]

**ID:** P1.2
**Dependencies:** None (can run in parallel with P1.1)
**Duration:** [X hours]
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

[Repeat structure from P1.1]

---

### Task P1.3: [Task Name]

**ID:** P1.3
**Dependencies:** P1.1, P1.2 (must complete first)
**Duration:** [X hours]
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

[Repeat structure from P1.1]

---

## Phase 2: [Phase Name] ([X-Y hours])

### Task P2.1: [Task Name]

**ID:** P2.1
**Dependencies:** P1.3
**Duration:** [X hours]
**Priority:** P1 (High)
**Status:** ⬜ Not Started

[Repeat structure from P1.1]

---

## Phase 3: [Phase Name] ([X-Y hours])

### Task P3.1: [Task Name]

**ID:** P3.1
**Dependencies:** P2.3
**Duration:** [X hours]
**Priority:** P1 (High)
**Status:** ⬜ Not Started

[Repeat structure from P1.1]

---

## Blocked Tasks

**Tasks currently blocked:**
- [Task ID]: Blocked by [blocking task or external dependency]
- [Task ID]: Blocked by [blocking task or external dependency]

---

## Completed Tasks Log

### [YYYY-MM-DD] - Task P1.1 Complete
**Completed By:** [Name]
**Actual Duration:** [X hours] (Estimated: [Y hours])
**Notes:**
- [Any deviations from plan]
- [Lessons learned]
- [Issues encountered]

---

## Task Dependencies Graph

**Visual representation of task dependencies:**

```
P1.1 ──┐
       ├─→ P1.3 ─→ P2.1 ─→ P2.3 ─→ P3.1 ─→ P3.3
P1.2 ──┘              ↓
                    P2.2 ─→ P3.2 ──┘

Legend:
─→  Dependency (must complete before)
┐   Merge point (all parents must complete)
```

**Critical Path:**
```
P1.1 → P1.3 → P2.1 → P2.3 → P3.1 → P3.3
Total: [X hours]
```

---

## Daily Progress Tracking

### [YYYY-MM-DD]
**Tasks Completed:** [List of task IDs]
**Tasks In Progress:** [List of task IDs]
**Blockers:** [Any blockers identified]
**Notes:** [Daily notes]

---

## Quality Gate Checklist

Before marking ANY task as complete:

- [ ] **Tests Pass** - `go test ./...`
- [ ] **Code Formatted** - `go fmt ./...`
- [ ] **Dependencies Tidy** - `go mod tidy`
- [ ] **Vet Passes** - `go vet ./...`
- [ ] **Linter Passes** - `golangci-lint run`
- [ ] **Build Succeeds** - `go build -o bin/nuimanbot ./cmd/nuimanbot`
- [ ] **Code Review** - At least one approval
- [ ] **Documentation** - Updated if needed

**Never mark a task complete until ALL gates pass.**

---

## Estimation Accuracy

**Track actual vs estimated time for continuous improvement:**

| Task ID | Estimated | Actual | Variance | Notes |
|---------|-----------|--------|----------|-------|
| P1.1    | 2h        | [Xh]   | [±Xh]    | [Reason for variance] |
| P1.2    | 3h        | [Xh]   | [±Xh]    | [Reason for variance] |

**Average Variance:** [±X%]

**Adjustments for Future Estimates:**
- [Learning 1]
- [Learning 2]

---

## Notes

**Assumptions:**
- [Assumption 1]
- [Assumption 2]

**Risks:**
- [Risk 1]: [Mitigation]
- [Risk 2]: [Mitigation]

**Open Questions:**
- [Question 1]
- [Question 2]
