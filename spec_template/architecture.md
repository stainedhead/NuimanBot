# [Feature Name] - System Architecture

**Created:** [YYYY-MM-DD]
**Version:** 1.0
**Status:** [Draft | Complete]
**Last Updated:** [YYYY-MM-DD]

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [System Context](#system-context)
3. [Component Architecture](#component-architecture)
4. [Layer Responsibilities](#layer-responsibilities)
5. [Data Flow](#data-flow)
6. [Sequence Diagrams](#sequence-diagrams)
7. [Integration Points](#integration-points)
8. [Architectural Decisions](#architectural-decisions)
9. [Trade-offs](#trade-offs)

---

## Architecture Overview

**High-Level Summary:**
[1-2 paragraph overview of the system architecture]

**Architectural Style:** Clean Architecture + [Other patterns]

**Key Principles:**
- Dependency Inversion: Outer layers depend on inner layers
- Single Responsibility: Each component has one clear purpose
- Open/Closed: Open for extension, closed for modification
- [Other principles]

**Architecture Diagram:**
```
┌─────────────────────────────────────────────────────┐
│               Infrastructure Layer                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐          │
│  │  [Impl1] │  │  [Impl2] │  │  [Impl3] │          │
│  └──────────┘  └──────────┘  └──────────┘          │
└────────────────────┬────────────────────────────────┘
                     │ implements interfaces
┌────────────────────▼────────────────────────────────┐
│                 Adapter Layer                        │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐          │
│  │[Adapter1]│  │[Adapter2]│  │[Adapter3]│          │
│  └──────────┘  └──────────┘  └──────────┘          │
└────────────────────┬────────────────────────────────┘
                     │ uses
┌────────────────────▼────────────────────────────────┐
│                Use Case Layer                        │
│  ┌──────────────────────────────────────┐           │
│  │         [Service/Orchestrator]        │           │
│  └──────────────────────────────────────┘           │
└────────────────────┬────────────────────────────────┘
                     │ uses
┌────────────────────▼────────────────────────────────┐
│                 Domain Layer                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐          │
│  │ [Entity1]│  │ [Entity2]│  │[Interface]│          │
│  └──────────┘  └──────────┘  └──────────┘          │
└─────────────────────────────────────────────────────┘
```

---

## System Context

**External Systems:**
```
┌──────────────┐
│   User/CLI   │
└──────┬───────┘
       │
       ▼
┌──────────────────────────────────────┐
│         NuimanBot System              │
│  ┌────────────────────────────────┐  │
│  │      [Feature Name]            │  │
│  │  ┌──────────┐  ┌──────────┐   │  │
│  │  │Component1│  │Component2│   │  │
│  │  └──────────┘  └──────────┘   │  │
│  └────────────────────────────────┘  │
└──────┬───────────────────┬───────────┘
       │                   │
       ▼                   ▼
┌──────────────┐    ┌──────────────┐
│   Database   │    │External API  │
└──────────────┘    └──────────────┘
```

**System Boundaries:**
- **Inputs:** [What comes into the system]
- **Outputs:** [What goes out of the system]
- **External Dependencies:** [What the system depends on]

**Integration Points:**
| System | Type | Protocol | Purpose |
|--------|------|----------|---------|
| [System 1] | Database | SQL | Data persistence |
| [System 2] | API | REST/HTTP | External service |
| [System 3] | Message Queue | AMQP | Event processing |

---

## Component Architecture

### Component Diagram

```
┌────────────────────────────────────────────────────┐
│                  [Feature Name]                     │
│                                                     │
│  ┌─────────────┐         ┌─────────────┐          │
│  │ Component A │────────>│ Component B │          │
│  │             │         │             │          │
│  │ - Method1() │         │ - Method1() │          │
│  │ - Method2() │         │ - Method2() │          │
│  └─────────────┘         └─────────────┘          │
│         │                        │                 │
│         └────────┬───────────────┘                 │
│                  ▼                                  │
│         ┌─────────────┐                            │
│         │ Component C │                            │
│         │             │                            │
│         │ - Method1() │                            │
│         │ - Method2() │                            │
│         └─────────────┘                            │
└────────────────────────────────────────────────────┘
```

### Component Descriptions

#### Component A: [Name]

**Responsibility:**
[What this component is responsible for]

**Dependencies:**
- Component B (via interface)
- [Other dependencies]

**Provides:**
- [Service/Interface 1]
- [Service/Interface 2]

**Lifecycle:**
- Created during [initialization phase]
- Lifespan: [Singleton | Request-scoped | etc.]

**Concurrency:**
- Thread-safe: [Yes/No]
- Synchronization: [How it handles concurrent access]

---

#### Component B: [Name]

[Repeat structure from Component A]

---

## Layer Responsibilities

### Domain Layer

**Location:** `internal/domain/`

**Responsibility:**
- Define core business entities
- Specify business rules
- Define repository/service interfaces

**Contains:**
- [Entity 1]
- [Entity 2]
- [Interface 1]
- [Interface 2]

**Dependencies:** None (pure domain logic)

**Example:**
```go
// Domain entity
type [Entity] struct {
    ID   string
    Name string
}

// Domain interface
type [Repository] interface {
    Get(ctx context.Context, id string) (*[Entity], error)
}
```

---

### Use Case Layer

**Location:** `internal/usecase/[feature]/`

**Responsibility:**
- Orchestrate business logic
- Coordinate between domain and infrastructure
- Implement application-specific workflows

**Contains:**
- [Service 1]
- [Service 2]
- [Use case 1]

**Dependencies:**
- Domain layer (entities, interfaces)

**Example:**
```go
// Use case orchestrator
type Service struct {
    repo domain.[Repository]
}

func (s *Service) ExecuteWorkflow(ctx context.Context) error {
    // Orchestrate domain entities
}
```

---

### Infrastructure Layer

**Location:** `internal/infrastructure/[feature]/`

**Responsibility:**
- Implement domain interfaces
- Handle external system interactions
- Provide technical capabilities

**Contains:**
- [Implementation 1]
- [Implementation 2]
- [Client 1]

**Dependencies:**
- Domain layer (interfaces to implement)
- External libraries/SDKs

**Example:**
```go
// Infrastructure implementation
type SQLiteRepository struct {
    db *sql.DB
}

func (r *SQLiteRepository) Get(ctx context.Context, id string) (*domain.[Entity], error) {
    // Database implementation
}
```

---

### Adapter Layer

**Location:** `internal/adapter/[type]/`

**Responsibility:**
- Adapt external interfaces to internal ones
- Handle protocol conversions
- Manage request/response transformations

**Contains:**
- [Adapter 1]
- [Adapter 2]
- [Handler 1]

**Dependencies:**
- Use case layer (services)
- Infrastructure layer (implementations)

**Example:**
```go
// CLI adapter
type CLIAdapter struct {
    service *usecase.Service
}

func (a *CLIAdapter) HandleCommand(cmd string) error {
    // Convert CLI command to use case call
}
```

---

## Data Flow

### Request Flow

**1. User Request:**
```
User → CLI Adapter → Use Case Service → Domain Entity → Infrastructure Repository → Database
```

**Step-by-Step:**
1. User issues command via CLI
2. CLI Adapter receives input
3. Adapter validates and transforms to use case input
4. Use Case Service executes business logic
5. Service uses Domain entities to model data
6. Service calls Repository interface
7. Infrastructure Repository implements database access
8. Database returns data
9. Flow reverses back to user

**Example:**
```go
// 1. User command
> /[command] [args]

// 2. CLI Adapter
func (a *Adapter) Handle(cmd string) {
    input := parseCommand(cmd)  // Transform
    a.service.Execute(input)    // Delegate
}

// 3. Use Case Service
func (s *Service) Execute(input Input) {
    entity := s.repo.Get(input.ID)  // Domain operation
    entity.DoSomething()             // Business logic
    s.repo.Save(entity)              // Persist
}
```

---

### Response Flow

**1. Service Response:**
```
Database → Infrastructure → Use Case → Adapter → User
```

**Transformation Layers:**
- Database row → Domain entity (Infrastructure)
- Domain entity → Use case output (Use Case)
- Use case output → User-facing message (Adapter)

---

### Error Flow

**Error Propagation:**
```
Infrastructure (DB error) → Use Case (business error) → Adapter (user message)
```

**Error Handling Strategy:**
- Infrastructure: Wrap errors with context
- Use Case: Categorize errors (user/system/external)
- Adapter: Format user-friendly messages

**Example:**
```go
// Infrastructure
if err := db.Query(); err != nil {
    return fmt.Errorf("database query failed: %w", err)
}

// Use Case
if err := repo.Get(); err != nil {
    return domain.ErrNotFound  // Business error
}

// Adapter
if err := service.Execute(); err != nil {
    fmt.Println("Error: Resource not found")  // User message
}
```

---

## Sequence Diagrams

### Sequence 1: [Primary Workflow]

**Scenario:** [Description of what this sequence shows]

```
User     Adapter   Service   Repository   Database
 |          |         |          |            |
 |─request─>|         |          |            |
 |          |─call───>|          |            |
 |          |         |─load────>|            |
 |          |         |          |─query─────>|
 |          |         |          |<──result──|
 |          |         |<─entity─|            |
 |          |         |─process─|            |
 |          |         |─save────>|            |
 |          |         |          |─insert───>|
 |          |         |          |<──ack─────|
 |          |<─response|          |            |
 |<─result─|         |          |            |
```

**Steps:**
1. User initiates request
2. Adapter receives and validates
3. Service loads domain entity
4. Repository queries database
5. Service processes business logic
6. Repository saves changes
7. Response flows back to user

---

### Sequence 2: [Error Scenario]

[Repeat structure for error/edge case scenarios]

---

## Integration Points

### Integration 1: Database

**Type:** SQLite
**Purpose:** Persistent data storage
**Protocol:** SQL

**Connection:**
```go
db, err := sql.Open("sqlite3", "./data/[feature].db")
```

**Schema:**
- Tables: [table1, table2]
- Migrations: Handled by [migration tool]

**Error Handling:**
- Connection failures: Retry with backoff
- Query errors: Wrapped and propagated

---

### Integration 2: External API

**Type:** REST API
**Purpose:** [Purpose of integration]
**Protocol:** HTTP/JSON

**Endpoint:**
```
POST https://api.example.com/v1/[endpoint]
Authorization: Bearer [token]
Content-Type: application/json
```

**Request:**
```json
{
  "field": "value"
}
```

**Response:**
```json
{
  "status": "success",
  "data": {}
}
```

**Error Handling:**
- Rate limits: Retry with exponential backoff
- Network errors: Circuit breaker pattern
- Invalid responses: Validation and fallback

---

## Architectural Decisions

### ADR-001: [Decision Title]

**Date:** [YYYY-MM-DD]
**Status:** [Accepted | Rejected | Superseded]

**Context:**
[What is the situation/problem that led to this decision?]

**Decision:**
[What did we decide to do?]

**Rationale:**
[Why did we make this choice?]

**Consequences:**
**Positive:**
- [Benefit 1]
- [Benefit 2]

**Negative:**
- [Trade-off 1]
- [Trade-off 2]

**Alternatives Considered:**
1. **[Alternative 1]**
   - Pros: [Pros]
   - Cons: [Cons]
   - Why rejected: [Reason]

2. **[Alternative 2]**
   - Pros: [Pros]
   - Cons: [Cons]
   - Why rejected: [Reason]

---

### ADR-002: [Decision Title]

[Repeat structure from ADR-001]

---

## Trade-offs

### Trade-off 1: [Description]

**Choice:** [What we chose]

**Benefits:**
- [Benefit 1]
- [Benefit 2]

**Costs:**
- [Cost/limitation 1]
- [Cost/limitation 2]

**Mitigation:**
- [How we mitigate the costs]

---

### Trade-off 2: [Description]

[Repeat structure]

---

## Performance Considerations

**Bottlenecks:**
- [Potential bottleneck 1]: [Mitigation]
- [Potential bottleneck 2]: [Mitigation]

**Optimization Strategies:**
- [Strategy 1]
- [Strategy 2]

**Caching:**
- [What we cache]
- [Cache invalidation strategy]

**Concurrency:**
- [How we handle concurrent requests]
- [Thread safety measures]

---

## Security Architecture

**Security Layers:**
1. Input validation (Adapter layer)
2. Authorization (Use case layer)
3. Data encryption (Infrastructure layer)

**Threat Model:**
- [Threat 1]: [Mitigation]
- [Threat 2]: [Mitigation]

**Security Controls:**
- [Control 1]
- [Control 2]

---

## Scalability

**Current Limits:**
- [Limit 1]: [Value]
- [Limit 2]: [Value]

**Scaling Strategy:**
- Vertical: [How to scale up]
- Horizontal: [How to scale out]

**Future Considerations:**
- [Consideration 1]
- [Consideration 2]

---

## References

- [spec.md](spec.md) - Feature specification
- [data-dictionary.md](data-dictionary.md) - Data structures
- [plan.md](plan.md) - Implementation plan
- [External architecture docs]
