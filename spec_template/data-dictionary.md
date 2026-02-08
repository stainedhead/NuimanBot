# [Feature Name] - Data Dictionary

**Created:** [YYYY-MM-DD]
**Version:** 1.0
**Status:** [Draft | Complete]

---

## Overview

This document defines all data structures, types, interfaces, and constants for the [Feature Name] implementation. Organized by Clean Architecture layers.

**Purpose:**
- Single source of truth for all data types
- Ensure consistency across layers
- Document validation rules and constraints
- Specify database schemas

---

## Table of Contents

1. [Domain Layer](#domain-layer)
2. [Use Case Layer](#use-case-layer)
3. [Infrastructure Layer](#infrastructure-layer)
4. [Adapter Layer](#adapter-layer)
5. [Configuration](#configuration)
6. [Type Aliases & Enums](#type-aliases--enums)
7. [Database Schema](#database-schema)
8. [API Types](#api-types)

---

## Domain Layer

Location: `internal/domain/`

### 1. [EntityName] (Entity)

**File:** `[entity].go`

```go
// [EntityName] represents [description of what this entity models]
type [EntityName] struct {
    // Unique identifier
    ID string

    // [Field description]
    FieldName1 string

    // [Field description]
    FieldName2 int

    // [Field description]
    FieldName3 time.Time

    // [Field description with business rules]
    FieldName4 [Type]
}
```

**Methods:**
```go
// [MethodName] [description of what method does]
func (e *[EntityName]) [MethodName]() [ReturnType]

// Validate checks if entity is valid according to business rules
func (e *[EntityName]) Validate() error

// String returns human-readable representation
func (e *[EntityName]) String() string
```

**Validation Rules:**
- `ID` must be non-empty, <= 64 chars
- `FieldName1` must match pattern `[a-z-]+`, <= 128 chars
- `FieldName2` must be >= 0, <= 1000
- `FieldName3` must not be zero value
- [Custom validation rule]

**Business Rules:**
- [Business rule 1]
- [Business rule 2]

**Example:**
```go
entity := &[EntityName]{
    ID:         "example-123",
    FieldName1: "example-value",
    FieldName2: 42,
    FieldName3: time.Now(),
}

if err := entity.Validate(); err != nil {
    // Handle validation error
}
```

---

### 2. [ValueObject] (Value Object)

**File:** `[entity].go`

```go
// [ValueObject] represents [description]
type [ValueObject] struct {
    Field1 string `yaml:"field_1"`
    Field2 bool   `yaml:"field_2,omitempty"`
    Field3 []string `yaml:"field_3,omitempty"`
}
```

**Methods:**
```go
// IsValid checks if value object is valid
func (vo *[ValueObject]) IsValid() bool

// Equals compares two value objects
func (vo *[ValueObject]) Equals(other *[ValueObject]) bool
```

**Validation Rules:**
- [Rule 1]
- [Rule 2]

**Immutability:**
- [Is this value object immutable? If yes, document that changes create new instances]

---

### 3. [InterfaceName] (Repository/Service Interface)

**File:** `[entity].go`

```go
// [InterfaceName] defines operations for [description]
type [InterfaceName] interface {
    // Create [description]
    Create(ctx context.Context, entity *[EntityName]) error

    // Get [description]
    Get(ctx context.Context, id string) (*[EntityName], error)

    // Update [description]
    Update(ctx context.Context, entity *[EntityName]) error

    // Delete [description]
    Delete(ctx context.Context, id string) error

    // List [description]
    List(ctx context.Context, filter [FilterType]) ([]*[EntityName], error)
}
```

**Expected Behavior:**
- `Create`: [Behavior description, error conditions]
- `Get`: [Behavior description, error conditions]
- `Update`: [Behavior description, error conditions]
- `Delete`: [Behavior description, error conditions]
- `List`: [Behavior description, error conditions]

**Error Conditions:**
- Returns `ErrNotFound` if entity doesn't exist
- Returns `ErrAlreadyExists` if duplicate detected
- Returns `ErrInvalidInput` if validation fails
- [Other error conditions]

---

## Use Case Layer

Location: `internal/usecase/[feature]/`

### 1. [ServiceName] (Service)

**File:** `service.go`

```go
// [ServiceName] orchestrates [description of business logic]
type [ServiceName] struct {
    repo      domain.[InterfaceName]
    validator domain.[ValidatorInterface]
    // Other dependencies
}

// New[ServiceName] creates a new service instance
func New[ServiceName](
    repo domain.[InterfaceName],
    validator domain.[ValidatorInterface],
) *[ServiceName] {
    return &[ServiceName]{
        repo:      repo,
        validator: validator,
    }
}
```

**Methods:**
```go
// [MethodName] [description of use case]
func (s *[ServiceName]) [MethodName](
    ctx context.Context,
    input [InputType],
) ([OutputType], error)
```

**Use Cases:**
- [Use case 1]: [Description]
- [Use case 2]: [Description]

---

### 2. [InputType] (Use Case Input)

**File:** `types.go`

```go
// [InputType] represents input for [use case name]
type [InputType] struct {
    Field1 string
    Field2 int
    // Other fields
}

// Validate checks if input is valid
func (i *[InputType]) Validate() error
```

**Validation Rules:**
- [Rule 1]
- [Rule 2]

---

### 3. [OutputType] (Use Case Output)

**File:** `types.go`

```go
// [OutputType] represents output from [use case name]
type [OutputType] struct {
    Result  [Type]
    Metadata map[string]any
}
```

---

## Infrastructure Layer

Location: `internal/infrastructure/[feature]/`

### 1. [ImplementationName] (Implementation)

**File:** `[impl].go`

```go
// [ImplementationName] implements domain.[InterfaceName]
type [ImplementationName] struct {
    db     *sql.DB
    config [ConfigType]
    // Other dependencies
}

// New[ImplementationName] creates a new implementation
func New[ImplementationName](
    db *sql.DB,
    config [ConfigType],
) *[ImplementationName] {
    return &[ImplementationName]{
        db:     db,
        config: config,
    }
}
```

**Implements:** `domain.[InterfaceName]`

**Dependencies:**
- [Dependency 1]
- [Dependency 2]

---

### 2. [ConfigType] (Configuration)

**File:** `config.go`

```go
// [ConfigType] holds configuration for [component]
type [ConfigType] struct {
    Option1 string
    Option2 int
    Option3 time.Duration
}

// Validate checks if configuration is valid
func (c *[ConfigType]) Validate() error
```

**Default Values:**
```go
var Default[ConfigType] = [ConfigType]{
    Option1: "default-value",
    Option2: 100,
    Option3: 30 * time.Second,
}
```

---

## Adapter Layer

Location: `internal/adapter/[type]/`

### 1. [AdapterName] (Adapter)

**File:** `[adapter].go`

```go
// [AdapterName] adapts [external system] to [internal interface]
type [AdapterName] struct {
    service domain.[ServiceInterface]
    // Other dependencies
}
```

**Purpose:** [What this adapter does]

**Adapts:** [External interface] → [Internal interface]

---

## Configuration

Location: `internal/config/`

### Feature Configuration

**File:** `config.go`

```yaml
# Configuration structure (YAML format)
[feature]:
  enabled: true
  option_1: value
  option_2: value
  sub_config:
    nested_option: value
```

**Go Struct:**
```go
type [FeatureConfig] struct {
    Enabled bool              `yaml:"enabled"`
    Option1 string            `yaml:"option_1"`
    Option2 int               `yaml:"option_2"`
    SubConfig [SubConfigType] `yaml:"sub_config"`
}
```

**Environment Variable Overrides:**
- `NUIMANBOT_[FEATURE]_ENABLED` → `Enabled`
- `NUIMANBOT_[FEATURE]_OPTION1` → `Option1`
- `NUIMANBOT_[FEATURE]_OPTION2` → `Option2`

---

## Type Aliases & Enums

### Enumerations

#### [EnumName]

```go
// [EnumName] represents [description]
type [EnumName] int

const (
    [EnumValue1] [EnumName] = iota // [Description]
    [EnumValue2]                   // [Description]
    [EnumValue3]                   // [Description]
)

// String returns the string representation
func (e [EnumName]) String() string {
    return [...]string{"value1", "value2", "value3"}[e]
}

// IsValid checks if enum value is valid
func (e [EnumName]) IsValid() bool {
    return e >= [EnumValue1] && e <= [EnumValue3]
}
```

**Valid Values:**
- `[EnumValue1]`: [Description of when to use]
- `[EnumValue2]`: [Description of when to use]
- `[EnumValue3]`: [Description of when to use]

---

### Type Aliases

```go
// [AliasName] is an alias for [description]
type [AliasName] string

// Validation rules
func (a [AliasName]) Validate() error
```

---

## Database Schema

### Table: [table_name]

**Purpose:** [What this table stores]

**Schema:**
```sql
CREATE TABLE [table_name] (
    id TEXT PRIMARY KEY,
    field_1 TEXT NOT NULL,
    field_2 INTEGER NOT NULL,
    field_3 TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    -- Constraints
    CONSTRAINT [constraint_name] CHECK ([condition])
);

-- Indexes
CREATE INDEX idx_[table]_[field] ON [table_name]([field]);
CREATE UNIQUE INDEX idx_[table]_[field]_unique ON [table_name]([field]);
```

**Columns:**
| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| `id` | TEXT | No | Primary key, unique identifier |
| `field_1` | TEXT | No | [Description] |
| `field_2` | INTEGER | No | [Description] |
| `field_3` | TEXT | Yes | [Description] |
| `created_at` | TIMESTAMP | No | Record creation time |
| `updated_at` | TIMESTAMP | No | Last update time |

**Indexes:**
- `idx_[table]_[field]`: [Purpose of index]
- `idx_[table]_[field]_unique`: [Purpose of unique index]

**Relationships:**
- Foreign key to `[other_table].[field]`
- References `[table].[field]`

**Constraints:**
- [Constraint description]

---

## API Types

### Request Types

#### [RequestType]

**Endpoint:** `POST /api/[endpoint]`

**Content-Type:** `application/json`

```go
// [RequestType] represents API request for [operation]
type [RequestType] struct {
    Field1 string `json:"field_1"`
    Field2 int    `json:"field_2"`
}
```

**Example:**
```json
{
  "field_1": "value",
  "field_2": 123
}
```

**Validation:**
- `field_1`: Required, max length 256
- `field_2`: Required, range 0-1000

---

### Response Types

#### [ResponseType]

**Content-Type:** `application/json`

```go
// [ResponseType] represents API response for [operation]
type [ResponseType] struct {
    Success bool              `json:"success"`
    Data    [DataType]        `json:"data,omitempty"`
    Error   string            `json:"error,omitempty"`
}
```

**Example (Success):**
```json
{
  "success": true,
  "data": {
    "field": "value"
  }
}
```

**Example (Error):**
```json
{
  "success": false,
  "error": "Error message"
}
```

---

## Constants

### Error Messages

```go
const (
    ErrMsg[ErrorType] = "[error message template]"
    // Additional error messages
)
```

### Limits and Thresholds

```go
const (
    Max[LimitName] = 1000
    Default[LimitName] = 100
    Min[LimitName] = 1
)
```

### Timeouts and Durations

```go
const (
    Default[Operation]Timeout = 30 * time.Second
    Max[Operation]Timeout     = 5 * time.Minute
)
```

---

## Type Mapping Reference

**Domain → Database:**
| Domain Type | Database Type | Conversion |
|-------------|---------------|------------|
| `string` | `TEXT` | Direct |
| `int` | `INTEGER` | Direct |
| `time.Time` | `TIMESTAMP` | RFC3339 |
| `[]string` | `TEXT` | JSON array |
| `map[string]any` | `TEXT` | JSON object |

**Domain → API:**
| Domain Type | JSON Type | Conversion |
|-------------|-----------|------------|
| `string` | `string` | Direct |
| `int` | `number` | Direct |
| `time.Time` | `string` | RFC3339 |
| `[]string` | `array` | Direct |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | [YYYY-MM-DD] | Initial version |
