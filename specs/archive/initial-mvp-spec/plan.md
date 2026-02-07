# NuimanBot Initial MVP Implementation Plan (Sub-Agent Driven)

## 1. Introduction

This document outlines a detailed implementation plan for the NuimanBot Initial Minimum Viable Product (MVP), leveraging a "sub-agent" or "worker thread" model for parallel development. The plan is based on the comprehensive `PRODUCT_REQUIREMENT_DOC.md` (which now serves as `specs/initial-mvp-spec/spec.md`) and strictly adheres to the Clean Architecture and Test-Driven Development (TDD) principles outlined in `AGENTS.md`. A key focus is on defining clear responsibilities for each sub-agent and establishing robust mechanisms for dependency management to avoid conflicts and ensure seamless integration.

## 2. Overall Strategy: Sub-Agent Collaboration on Clean Architecture

The implementation will be decomposed into tasks assigned to specialized sub-agents, each responsible for a distinct part of the codebase, primarily aligned with Clean Architecture layers and specific functional domains.

**Core Principles:**
*   **Strict Layer Adherence**: Each sub-agent will operate within the boundaries of Clean Architecture layers (`domain`, `usecase`, `adapter`, `infrastructure`).
*   **Interface-Driven Development**: All interactions between components, especially across layers, will be defined via Go interfaces. Inner layers define interfaces; outer layers implement them. This is crucial for decoupling and managing dependencies.
*   **TDD First**: Every sub-agent will follow the Red-Green-Refactor cycle for their assigned tasks.
*   **Centralized Integration**: A designated "Integration Lead" (Architect Agent) will be responsible for final assembly, dependency injection, and resolving cross-cutting concerns.
*   **Version Control**: Atomic commits and frequent merges to a central repository (e.g., `main` branch) are essential. Feature branches for each major sub-agent task will be used.

## 3. Sub-Agent Roles and Responsibilities (MVP Focus)

The following sub-agents will be responsible for specific areas to implement the MVP:

### 3.1. Architect Agent (Human/Orchestrator) (Status: PENDING)

*   **Responsibility**: Defines overall system contracts, major interfaces, and ensures adherence to Clean Architecture. Resolves inter-agent conflicts and oversees integration points. This agent defines the initial `NuimanBotConfig` structure and entry points.
*   **Key Tasks**:
    *   Define initial `go.mod` and project structure. [Status: PENDING]
    *   Establish initial `main.go` in `cmd/nuimanbot`. [Status: PENDING]
    *   Define the global `NuimanBotConfig` struct in `internal/config`. [Status: PENDING]
    *   Oversee interface definitions between layers. [Status: PENDING]
    *   Integrate final components via dependency injection. [Status: PENDING]
### 3.2. Domain Agent (Status: PENDING)

*   **Responsibility**: Implements all core business entities, value objects, and domain-level interfaces within `internal/domain/`.
*   **Key Deliverables (MVP)**:
    *   `internal/domain/user.go`: `User`, `Role`, `Platform` types. [Status: PENDING]
    *   `internal/domain/message.go`: `IncomingMessage`, `OutgoingMessage`, `Conversation` types. [Status: PENDING]
    *   `internal/domain/skill.go`: `Skill` interface, `SkillConfig`, `SkillResult`, `Permission` types. [Status: PENDING]
    *   `internal/domain/llm.go`: `LLMProvider`, `LLMRequest`, `LLMResponse`, `StreamChunk` types. [Status: PENDING]
    *   `internal/domain/error.go`: Custom domain error types. [Status: PENDING]*   **Dependencies**: No outbound dependencies. Collaborates with other agents by providing defined interfaces.

### 3.3. Security & Crypto Agent (Status: PENDING)

*   **Responsibility**: Implements all security-related infrastructure and use cases, focusing on credential management, encryption, and input validation.
*   **Key Deliverables (MVP)**:
    *   `internal/infrastructure/crypto/aes.go`: AES-256-GCM encryption/decryption functions. [Status: PENDING]
    *   `internal/infrastructure/crypto/vault.go`: Implementation of `CredentialVault` interface, using `aes.go`. [Status: PENDING]
    *   `internal/usecase/security/service.go`: Implementation of `SecurityService` interface (`Encrypt`, `Decrypt`, `ValidateInput`, `Audit`). [Status: PENDING]
    *   `internal/usecase/security/input_validation.go`: Input sanitization and prompt/command injection detection logic. [Status: PENDING]
    *   `internal/domain/security.go`: `SecureString` type, `AuditEvent` structure. [Status: PENDING]*   **Dependencies**: Depends on `internal/domain`. Provides `SecurityService` and `CredentialVault` interfaces to `usecase` layer.

### 3.4. CLI Gateway Agent (Status: IN PROGRESS)

*   **Responsibility**: Develops the interactive command-line interface, acting as the primary user interaction point for the MVP.
*   **Key Deliverables (MVP)**:
    *   `internal/adapter/gateway/cli/gateway.go`: Implementation of `Gateway` interface for CLI. [Status: COMPLETE]
    *   REPL logic, command parsing, output formatting. [Status: COMPLETE]
    *   Integration with a `ChatService` (from `usecase` layer) and `UserService` for basic user interactions. [Status: IN PROGRESS]
    *   `internal/config/cli.go`: Definition and parsing of `CLIConfig`. [Status: COMPLETE]*   **Dependencies**: Depends on `internal/domain` (for `Message` types, `Platform`), and `internal/usecase/chat` (for `ChatService` interface).

### 3.5. LLM Abstraction & Anthropic Agent (Status: COMPLETE)

*   **Responsibility**: Implements the generic LLM service interface and the concrete Anthropic provider.
*   **Key Deliverables (MVP)**:
    *   `internal/usecase/llm/service.go`: `LLMService` interface definition (if not in domain) and its basic orchestrating service. [Status: COMPLETE]
    *   `internal/infrastructure/llm/anthropic/client.go`: Concrete implementation of `LLMService` for Anthropic, using `github.com/anthropics/anthropic-sdk-go`. [Status: COMPLETE]
    *   `internal/config/llm.go`: Definition and parsing of `LLMConfig` (including Anthropic-specific fields). [Status: COMPLETE]*   **Dependencies**: Depends on `internal/domain` (for `LLMRequest`/`Response` types), and external `anthropic-sdk-go` library. Provides `LLMService` to `usecase` layer.

### 3.6. Skills Core & Built-in Skills Agent (Status: IN PROGRESS)

*   **Responsibility**: Builds the core skills execution system and implements the initial `calculator` and `datetime` skills.
*   **Key Deliverables (MVP)**:
    *   `internal/usecase/skill/service.go`: `SkillExecutionService` (or similar) to manage skill loading, execution, and permission checks. [Status: COMPLETE]
    *   `internal/usecase/skill/registry.go`: `SkillRegistry` implementation. [Status: COMPLETE]
    *   `internal/skills/calculator/calculator.go`: Implementation of the `calculator` skill. [Status: PENDING]
    *   `internal/skills/datetime/datetime.go`: Implementation of the `datetime` skill. [Status: PENDING]
    *   `internal/config/skills.go`: Definition and parsing of `SkillsSystemConfig`. [Status: COMPLETE]*   **Dependencies**: Depends on `internal/domain` (for `Skill` interface, `Permission`), and `internal/usecase/security` (for permission checks).

### 3.7. Memory & SQLite Agent (Status: COMPLETE)

*   **Responsibility**: Implements the memory persistence layer, specifically using SQLite for the MVP.
*   **Key Deliverables (MVP)**:
    *   `internal/usecase/memory/repository.go`: `MemoryRepository` interface. [Status: COMPLETE]
    *   `internal/adapter/repository/sqlite/user.go`: SQLite implementation for `UserRepository` (saving/retrieving users). [Status: COMPLETE]
    *   `internal/adapter/repository/sqlite/message.go`: SQLite implementation for `MessageRepository` (saving/retrieving messages, conversations). [Status: COMPLETE]
    *   `internal/config/storage.go`: Definition and parsing of `Storage` configuration. [Status: COMPLETE]*   **Dependencies**: Depends on `internal/domain` (for `User`, `Message`, `Conversation` types). Uses `github.com/mattn/go-sqlite3`.

### 3.8. Configuration Agent (Status: PENDING)

*   **Responsibility**: Manages the loading, parsing, and provision of the global `NuimanBotConfig` to all parts of the application.
*   **Key Deliverables (MVP)**:
    *   `internal/config/config.go`: Central `NuimanBotConfig` struct. [Status: PENDING]
    *   `internal/config/loader.go`: Logic for loading configuration from YAML and environment variables (e.g., using Viper). [Status: PENDING]
    *   Ensuring `SecureString` types are handled correctly (e.g., decrypted on load). [Status: PENDING]*   **Dependencies**: None. Provides configuration to all other components.

### 3.9. Quality Assurance (QA) Agent (Cross-Cutting) (Status: PENDING)

*   **Responsibility**: Ensures all code adheres to TDD, maintains high test coverage, and passes all quality gates.
*   **Key Deliverables (MVP)**:
    *   Develops comprehensive test suites for all implemented components. [Status: PENDING]
    *   Sets up and enforces `golangci-lint` configuration. [Status: PENDING]
    *   Monitors and reports test coverage metrics. [Status: PENDING]
    *   Performs integration tests for major feature flows (e.g., CLI input -> LLM -> Skill -> CLI output). [Status: PENDING]
    *   Performs security tests (e.g., input validation attacks). [Status: PENDING]*   **Dependencies**: Works across all layers and components.

## 4. Inter-Agent Communication and Dependency Management

To avoid dependency conflicts and ensure smooth integration:

*   **Interface Definitions First**: The Architect Agent (or a collaborative effort with relevant sub-agents) will prioritize defining Go interfaces in the `internal/domain/` and `internal/usecase/` layers. These interfaces act as contracts.
*   **Mocking for Parallelism**: Sub-agents can use mock implementations of interfaces not yet completed by other agents to enable parallel development. For example, the `CLI Gateway Agent` can mock the `ChatService` interface until the `Chat Service Agent` provides a concrete implementation.
*   **Dependency Injection**: All inter-component dependencies will be managed through dependency injection in `cmd/nuimanbot/main.go`. Sub-agents will focus on implementing services and adapters, not on directly instantiating their dependencies.
*   **Feature Branches**: Each sub-agent will work on dedicated feature branches, merging frequently into `main` after passing local tests.
*   **Code Reviews**: Cross-agent code reviews will be conducted to ensure interface adherence, architectural compliance, and consistency.
*   **Shared Utilities**: A common `internal/utils` package (carefully managed to avoid becoming a catch-all) can be used for truly generic helper functions.

### Example Dependency Flow (High-Level)

1.  **Domain Agent**: Defines `domain.User`, `domain.Skill`, `domain.Message`, `domain.LLMRequest`, `domain.LLMService` interface.
2.  **Security & Crypto Agent**: Implements `infrastructure.crypto.AES`, `infrastructure.crypto.Vault`, `usecase.security.Service`.
3.  **LLM Abstraction & Anthropic Agent**: Implements `infrastructure.llm.AnthropicClient` (implements `domain.LLMService`), `usecase.llm.Service`.
4.  **Skills Core Agent**: Implements `usecase.skill.Service` (which uses `domain.LLMService` to execute skills). Develops `internal/skills/calculator`, `internal/skills/datetime`.
5.  **Memory & SQLite Agent**: Implements `adapter.repository.sqlite.UserRepository`, `adapter.repository.sqlite.MessageRepository` (implements `usecase.memory.Repository`).
6.  **CLI Gateway Agent**: Implements `adapter.gateway.cli.Gateway`. This `Gateway` will receive `usecase.chat.Service` (which uses `usecase.skill.Service`, `usecase.memory.Repository`, `usecase.security.Service`).
7.  **Architect Agent**: In `cmd/nuimanbot/main.go`, orchestrates dependency injection, creating instances of infrastructure components, passing them to adapter implementations, and then to use case services, finally assembling the core application logic and starting the `CLI Gateway`.

## 5. Implementation Workflow (Per Sub-Agent Iteration)

1.  **Understand**: Review assigned section of `spec.md` (and overall PRD), `AGENTS.md` (architecture, TDD).
2.  **Plan**: Break down the specific task into smaller, testable units. Identify required interfaces and dependencies.
3.  **Test First (Red)**: Write unit tests for a small piece of functionality.
4.  **Implement (Green)**: Write minimal code to make the test pass.
5.  **Refactor**: Improve code quality, readability, and adherence to Go idioms/Clean Architecture. Ensure tests remain green.
6.  **Verify**: Run all local tests, linter (`golangci-lint`), and build.
7.  **Document**: Update internal documentation, add comments where necessary.
8.  **Integrate**: Merge to feature branch, then propose merge to `main` (after Architect Agent approval/review).

## 6. Integration and Finalization

*   **Continuous Integration**: Automated builds and tests on every `main` branch commit.
*   **Staging Environment**: Deploy integrated MVP to a staging environment for end-to-end testing.
*   **User Acceptance Testing (UAT)**: Engage stakeholders for feedback on core CLI functionality and initial skills.
*   **Documentation Review**: Final review of all generated documentation (PRD, `spec.md`, `plan.md`).

This detailed plan, with its emphasis on sub-agent collaboration and interface-driven development, provides a robust framework for building the NuimanBot MVP efficiently while managing complexity and dependencies.