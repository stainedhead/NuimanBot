# Specification Process Rules

**Last Updated:** 2026-02-07
**Version:** 1.0

---

## Overview

This document defines the specification process for feature development in NuimanBot. All features follow a structured workflow from initial research through implementation to completion.

---

## Directory Structure

Each feature gets its own subdirectory in `specs/`:

```
specs/
└── <feature-name>/
    ├── spec.md                  # Feature specification and requirements
    ├── plan.md                  # Implementation plan and architecture decisions
    ├── tasks.md                 # Task breakdown and progress tracking
    ├── research.md              # Research findings, API docs, examples
    ├── data-dictionary.md       # Data structures, types, schemas
    ├── architecture.md          # System architecture and design
    ├── implementation-notes.md  # Implementation details, gotchas, decisions
    └── STATUS.md                # (Optional) Progress tracking dashboard
```

---

## Document Purposes

### 1. spec.md - Feature Specification
**Purpose:** Define WHAT the feature does and WHY it's needed.

**Created:** Before any implementation work begins
**Audience:** Product owners, stakeholders, developers
**Content:**
- Executive summary
- Problem statement
- Goals and non-goals
- User requirements and acceptance criteria
- Success metrics
- Risks and constraints

**Key Principle:** Focus on outcomes, not implementation details.

---

### 2. research.md - Research Findings
**Purpose:** Gather background information, API documentation, and examples.

**Created:** Early in the specification process
**Audience:** Developers, architects
**Content:**
- Industry standards and specifications
- Existing API documentation
- Code examples from similar implementations
- Third-party library research
- Performance benchmarks
- Security considerations

**Key Principle:** Fact-gathering phase - collect evidence to inform decisions.

---

### 3. data-dictionary.md - Data Structures
**Purpose:** Define all data types, schemas, and domain entities.

**Created:** After research, before detailed planning
**Audience:** Developers, database designers
**Content:**
- Domain entities (structs, interfaces)
- Database schemas
- API request/response types
- Enumerations and constants
- Type aliases
- Validation rules

**Key Principle:** Single source of truth for all data structures in the feature.

---

### 4. architecture.md - System Architecture
**Purpose:** Design the high-level system architecture and component interactions.

**Created:** After data dictionary, concurrent with planning
**Audience:** Architects, senior developers
**Content:**
- System architecture diagrams
- Component responsibilities
- Layer interactions (Domain → Use Case → Infrastructure → Adapter)
- Data flow diagrams
- Sequence diagrams for key workflows
- Integration points with existing systems
- Architectural decisions and trade-offs

**Key Principle:** Focus on HOW components interact, not detailed implementation.

---

### 5. plan.md - Implementation Plan
**Purpose:** Define the implementation approach and phases.

**Created:** After architecture design
**Audience:** Developers, project managers
**Content:**
- Development approach (TDD, bottom-up, etc.)
- Phase breakdown with milestones
- Critical path analysis
- Dependency graph
- Testing strategy
- Rollout strategy
- Timeline estimates

**Key Principle:** Roadmap for execution - breaks large features into manageable phases.

---

### 6. tasks.md - Task Breakdown
**Purpose:** Break down implementation into concrete, testable tasks.

**Created:** After planning is complete
**Audience:** Developers executing the work
**Content:**
- Granular task list with IDs
- Task dependencies
- Time estimates per task
- Files to create/modify
- Acceptance criteria (checkboxes)
- Verification commands

**Key Principle:** Each task should be completable in 1-4 hours with clear done criteria.

---

### 7. implementation-notes.md - Implementation Log
**Purpose:** Record decisions, gotchas, and lessons learned during implementation.

**Created:** At implementation start, updated continuously
**Audience:** Future developers, code reviewers
**Content:**
- Technical decisions made during implementation
- Edge cases discovered and solutions
- Performance optimizations
- Deviations from original plan (with rationale)
- Refactoring insights
- Lessons learned

**Key Principle:** Living document - capture knowledge as you discover it.

---

### 8. STATUS.md (Optional)
**Purpose:** High-level progress dashboard for long-running features.

**Created:** For multi-phase features (>40 hours effort)
**Audience:** Project managers, stakeholders
**Content:**
- Overall progress percentage
- Phase status (not started, in progress, complete)
- Blockers and risks
- Recent activity log
- Next steps

**Key Principle:** Executive summary - quickly see where the feature stands.

---

## Progressive Documentation Workflow

Documents are created progressively as understanding evolves:

### Phase 0: Initial Research (PRD/Feature Research)
**Input:** Feature idea, user request, business need
**Output:** Initial PRD or feature research document (external to specs/)

**Activities:**
- Identify problem space
- Research existing solutions
- Gather requirements from stakeholders
- Define success criteria at a high level

**Decision Point:** Is this feature worth specifying? → Proceed to Phase 1

---

### Phase 1: Specification
**Input:** PRD/Feature research document
**Output:** `spec.md`

**Activities:**
1. Create feature directory: `specs/<feature-name>/`
2. Write `spec.md`:
   - Executive summary
   - Problem statement
   - Goals and non-goals
   - User requirements
   - Acceptance criteria
   - Success metrics

**Decision Point:** Is the specification approved? → Proceed to Phase 2

---

### Phase 2: Research & Data Modeling
**Input:** Approved spec.md
**Output:** `research.md`, `data-dictionary.md`

**Activities:**
1. Write `research.md`:
   - Gather API documentation
   - Find code examples
   - Research industry standards
   - Benchmark performance
   - Identify security concerns

2. Write `data-dictionary.md`:
   - Define domain entities
   - Design database schemas
   - Specify API types
   - List enumerations and constants
   - Document validation rules

**Decision Point:** Do we have enough information to architect? → Proceed to Phase 3

---

### Phase 3: Architecture & Planning
**Input:** research.md, data-dictionary.md
**Output:** `architecture.md`, `plan.md`

**Activities:**
1. Write `architecture.md`:
   - Design system architecture
   - Define component responsibilities
   - Map layer interactions
   - Create data flow diagrams
   - Document architectural decisions

2. Write `plan.md`:
   - Define development approach
   - Break into implementation phases
   - Identify critical path
   - Plan testing strategy
   - Estimate timeline

**Decision Point:** Is the plan feasible? → Proceed to Phase 4

---

### Phase 4: Task Breakdown
**Input:** architecture.md, plan.md
**Output:** `tasks.md`

**Activities:**
1. Write `tasks.md`:
   - Break each phase into tasks
   - Assign task IDs
   - Define dependencies
   - Estimate effort per task
   - Write acceptance criteria
   - Specify verification commands

**Decision Point:** Are tasks clear and actionable? → Proceed to Phase 5

---

### Phase 5: Implementation
**Input:** All specification documents
**Output:** Working code, `implementation-notes.md`

**Activities:**
1. Create `implementation-notes.md` (initially empty template)
2. Follow TDD workflow (Red → Green → Refactor)
3. Work through tasks in dependency order
4. Update `implementation-notes.md` as you go:
   - Record technical decisions
   - Document edge cases
   - Note performance optimizations
   - Track deviations from plan
5. Update `STATUS.md` (if using) to track progress

**Decision Point:** All tasks complete and tests passing? → Proceed to Phase 6

---

### Phase 6: Completion & Archival
**Input:** Completed implementation, passing tests
**Output:** Deployed feature, archived specs

**Activities:**
1. Final update to `implementation-notes.md`
2. Mark `STATUS.md` as complete
3. Update product documentation
4. Move specs to archive: `mv specs/<feature-name> specs/archive/`

**Decision Point:** Feature stable in production? → Archive complete

---

## Workflow Rules

### Rule 1: Create Feature Directory First
```bash
# Before any work begins
mkdir -p specs/<feature-name>
```

**Rationale:** Ensures all documentation is centralized.

---

### Rule 2: Update Progressively
- Specifications are **living documents**
- Update as understanding evolves
- Don't wait for "complete" information
- Capture decisions immediately

---

### Rule 3: Reference from Commits
```bash
# In commit messages
git commit -m "feat: implement user authentication

See specs/user-auth/spec.md for requirements
Implements tasks P2.1-P2.4 from specs/user-auth/tasks.md"
```

**Rationale:** Links code changes to specification context.

---

### Rule 4: Archive When Complete
```bash
# When feature is stable in production
mv specs/<feature-name> specs/archive/
```

**Rationale:** Keeps active specs clean, preserves history.

---

### Rule 5: Specs Are Gitignored
**Default:** `specs/` directory is gitignored

**Exceptions:**
- Can commit specs for shared team visibility (remove from .gitignore)
- Template directory (`spec_template/`) is committed

**Rationale:** Specs are planning artifacts, not source code.

---

## Development Approach

### Test-Driven Development (TDD)
All feature development follows strict TDD:

1. **Red:** Write a failing test
2. **Green:** Write minimal code to pass
3. **Refactor:** Improve code quality while keeping tests green

**Mandatory:** Refactoring phase is NOT optional.

---

### Clean Architecture (Bottom-Up)
Implementation follows layer order:

```
Domain Layer (entities, interfaces)
    ↓
Infrastructure Layer (parsers, storage, APIs)
    ↓
Use Case Layer (business logic, orchestration)
    ↓
Adapter Layer (CLI, gateways, repositories)
    ↓
Integration (wiring, configuration)
```

**Rationale:**
- Domain has no dependencies → start here
- Each layer only depends on layers below it
- Can test each layer in isolation

---

### Incremental Milestones
- Each phase produces working, tested code
- Can deploy after MVP phase
- Later phases are enhancements, not blockers

---

## Quality Gates

Before marking any task complete:

1. ✅ **Tests pass** - `go test ./...`
2. ✅ **Code formatted** - `go fmt ./...`
3. ✅ **Dependencies tidy** - `go mod tidy`
4. ✅ **Vet passes** - `go vet ./...`
5. ✅ **Linter passes** - `golangci-lint run`
6. ✅ **Build succeeds** - `go build -o bin/nuimanbot ./cmd/nuimanbot`
7. ✅ **Documentation updated** - Product docs reflect changes

**Never mark a task complete until all gates pass.**

---

## Example Feature Development Flow

### Initial Research Phase
```bash
# You have a PRD: feature-research/auth-system.md
# Read PRD, understand requirements
```

### Create Spec Directory
```bash
mkdir -p specs/user-authentication
cd specs/user-authentication
```

### Write Specification
```bash
# Create spec.md from template
cp ../../spec_template/spec.md ./spec.md

# Fill in:
# - Executive summary
# - Problem statement
# - Goals/non-goals
# - Requirements
# - Acceptance criteria
```

### Research Phase
```bash
# Create research.md from template
cp ../../spec_template/research.md ./research.md

# Gather:
# - OAuth 2.0 specification
# - JWT library documentation
# - Security best practices
# - Example implementations
```

### Data Modeling
```bash
# Create data-dictionary.md from template
cp ../../spec_template/data-dictionary.md ./data-dictionary.md

# Define:
# - User entity
# - Session entity
# - Token types
# - Database schema
```

### Architecture Design
```bash
# Create architecture.md from template
cp ../../spec_template/architecture.md ./architecture.md

# Design:
# - Auth service component
# - Token validation flow
# - Session management
# - Integration points
```

### Implementation Planning
```bash
# Create plan.md from template
cp ../../spec_template/plan.md ./plan.md

# Define:
# - 3 phases (domain, infrastructure, integration)
# - TDD approach
# - Testing strategy
# - Timeline: 40 hours
```

### Task Breakdown
```bash
# Create tasks.md from template
cp ../../spec_template/tasks.md ./tasks.md

# Break into tasks:
# - P1.1: Define User domain entity (2h)
# - P1.2: Define Session domain entity (2h)
# - P2.1: Implement JWT service (4h)
# ...
```

### Implementation
```bash
# Create implementation-notes.md from template
cp ../../spec_template/implementation-notes.md ./implementation-notes.md

# Follow TDD for each task:
# 1. Write test (Red)
# 2. Implement (Green)
# 3. Refactor

# Update implementation-notes.md as you go:
# - Record decisions
# - Document edge cases
# - Note optimizations
```

### Completion
```bash
# All tests pass, quality gates pass
go test ./...
go build -o bin/nuimanbot ./cmd/nuimanbot

# Update product documentation
# Mark STATUS.md complete (if used)

# Archive when stable
mv specs/user-authentication specs/archive/
```

---

## Common Pitfalls

### ❌ Starting Implementation Too Early
**Problem:** Writing code before spec is clear
**Solution:** Complete spec.md and get approval first

---

### ❌ Skipping Research Phase
**Problem:** Making uninformed architectural decisions
**Solution:** Always create research.md, even if brief

---

### ❌ No Data Dictionary
**Problem:** Inconsistent type names, schema mismatches
**Solution:** Define all types upfront in data-dictionary.md

---

### ❌ Tasks Too Large
**Problem:** Tasks taking >4 hours, unclear completion criteria
**Solution:** Break into smaller tasks with specific acceptance criteria

---

### ❌ Not Updating Implementation Notes
**Problem:** Losing context, repeating mistakes, unclear decisions
**Solution:** Update implementation-notes.md in real-time during development

---

### ❌ Skipping Refactor Phase
**Problem:** Accumulated technical debt, hard-to-maintain code
**Solution:** Refactoring is MANDATORY in TDD cycle (Red → Green → **Refactor**)

---

## Templates

All templates are located in `spec_template/`:

- `spec.md` - Feature specification template
- `research.md` - Research findings template
- `data-dictionary.md` - Data structures template
- `architecture.md` - System architecture template
- `plan.md` - Implementation plan template
- `tasks.md` - Task breakdown template
- `implementation-notes.md` - Implementation log template

**Usage:**
```bash
cp ../spec_template/spec.md ./spec.md
# Fill in the template
```

---

## Questions?

- Check existing specs in `specs/archive/` for examples
- Review `AGENTS.md` for development methodology
- See `README.md` for project overview

---

**Remember:** Specifications are for clarity, not bureaucracy. If a document doesn't add value, skip it or keep it minimal. The goal is to think through the problem space before writing code, not to create documentation for its own sake.
