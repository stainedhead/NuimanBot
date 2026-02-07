# NuimanBot Initial MVP Tasks

This document outlines the detailed task breakdown for implementing the NuimanBot Initial MVP, structured around the sub-agent collaboration model defined in `specs/initial-mvp-spec/plan.md`. Tasks are organized by sub-agent, adhering to Clean Architecture layers and focusing on Phase 1 objectives from the `PRODUCT_REQUIREMENT_DOC.md`.

## 1. Architect Agent Tasks (Orchestration & Setup)

*   **Task 1.1: Project Initialization** [Status: COMPLETE]
    *   Initialize Go module: `go mod init nuimanbot`.
    *   Create `cmd/nuimanbot/main.go` with a basic `main` function.
    *   Establish top-level directories: `internal/domain`, `internal/usecase`, `internal/adapter`, `internal/infrastructure`.
    *   Configure initial `.gitignore` and `golangci-lint` (as per `AGENTS.md`).
*   **Task 1.2: Global Configuration Struct Definition** [Status: COMPLETE]
    *   Define the `NuimanBotConfig` struct in `internal/config/config.go` with placeholder sub-structs for Server, Security, LLM, Gateways, MCP, Storage, Skills, Memory, ExternalAPI, Tools.
*   **Task 1.3: Dependency Injection Setup (Initial)** [Status: PENDING]
    *   Set up a basic dependency injection container/pattern in `cmd/nuimanbot/main.go` to handle core components (e.g., config loader).
*   **Task 1.4: CI Pipeline Setup (Basic)** [Status: PENDING]
    *   Implement a basic CI configuration (e.g., GitHub Actions workflow) to run `go fmt`, `go vet`, `golangci-lint`, `go test`, `go build`.

## 2. Domain Agent Tasks (internal/domain/)

*   **Task 2.1: User & Role Entities** [Status: COMPLETE]
    *   Define `Role` enum and `User` struct in `internal/domain/user.go`.
*   **Task 2.2: Message & Conversation Entities** [Status: COMPLETE]
    *   Define `Platform` enum, `IncomingMessage`, `OutgoingMessage`, `StoredMessage`, `Conversation` structs and related types in `internal/domain/message.go`.
*   **Task 2.3: Skill Interfaces & Types** [Status: COMPLETE]
    *   Define `Skill` interface, `SkillConfig`, `SkillResult`, `Permission` enum in `internal/domain/skill.go`.
*   **Task 2.4: LLM Interfaces & Types** [Status: COMPLETE]
    *   Define `LLMProvider` enum, `LLMRequest`, `LLMResponse`, `StreamChunk` structs, `ToolDefinition`, `ToolCall`, `TokenUsage` in `internal/domain/llm.go`.
*   **Task 2.5: Security Types** [Status: COMPLETE]
    *   Define `SecureString` and `AuditEvent` structs in `internal/domain/security.go`.
*   **Task 2.6: Generic Error Types** [Status: COMPLETE]
    *   Define custom domain error types in `internal/domain/errors.go`.
*   **Task 2.7: ChatService Implementation** [Status: COMPLETE]
    *   Implement the core ChatService logic in `internal/usecase/chat/service.go`.

## 3. Security & Crypto Agent Tasks (internal/usecase/security/, internal/infrastructure/crypto/)

*   **Task 3.1: AES-256-GCM Implementation** [Status: PENDING]
    *   Implement `Encrypt` and `Decrypt` functions using AES-256-GCM in `internal/infrastructure/crypto/aes.go`.
    *   Write unit tests for encryption/decryption.
*   **Task 3.2: Credential Vault** [Status: COMPLETE]
    *   Define `CredentialVault` interface in `internal/usecase/security/vault.go`.
    *   Implement `internal/infrastructure/crypto/vault.go` using `aes.go` for secure storage (e.g., file-based encrypted storage for MVP).
    *   Implement `Store`, `Retrieve`, `Delete`, `RotateKey`, `List` methods.
    *   Write unit and integration tests for the vault.
*   **Task 3.3: Security Service** [Status: COMPLETE]
    *   Define `SecurityService` interface in `internal/usecase/security/service.go`.
    *   Implement `SecurityService` with `Encrypt`, `Decrypt` methods (delegating to `CredentialVault`).
    *   Implement basic `ValidateInput` (max length, null bytes, UTF-8) in `internal/usecase/security/input_validation.go`.
    *   Implement `Audit` logging function in `internal/usecase/security/service.go`.
    *   Write unit tests for `SecurityService` and `ValidateInput`.

## 4. CLI Gateway Agent Tasks (internal/adapter/gateway/cli/)

*   **Task 4.1: CLI Gateway Interface Implementation** [Status: COMPLETE]
    *   Implement `Gateway` interface for CLI in `internal/adapter/gateway/cli/gateway.go`.
    *   Implement `Start` method to set up an interactive REPL loop.
    *   Implement `Send` method to display outgoing messages to the console.
    *   Implement `OnMessage` to register a handler for parsed CLI commands.
    *   Write unit tests for basic CLI input/output.
*   **Task 4.2: Command Parsing & Dispatch** [Status: COMPLETE]
    *   Develop a basic command parser within the CLI gateway to translate user input into `IncomingMessage` suitable for the `ChatService`.
*   **Task 4.3: CLI-specific Configuration** [Status: COMPLETE]
    *   Define `CLIConfig` in `internal/config/cli.go` and integrate into `NuimanBotConfig`.

## 5. LLM Abstraction & Anthropic Agent Tasks (internal/usecase/llm/, internal/infrastructure/llm/anthropic/)

*   **Task 5.1: LLM Service Orchestration** [Status: COMPLETE]
    *   Implement `LLMService` interface orchestration logic in `internal/usecase/llm/service.go` (e.g., routing requests to specific providers based on config).
    *   Write unit tests for service orchestration.
*   **Task 5.2: Anthropic Client Implementation** [Status: COMPLETE]
    *   Implement `LLMService` for Anthropic in `internal/infrastructure/llm/anthropic/client.go` using `github.com/anthropics/anthropic-sdk-go`.
    *   Implement `Complete`, `Stream`, `ListModels` methods.
    *   Write integration tests for Anthropic API calls (using mocks for external API).
*   **Task 5.3: LLM Configuration** [Status: COMPLETE]
    *   Define `LLMProviderConfig`, `LLMConfig` structs in `internal/config/llm.go`.

## 6. Skills Core & Built-in Skills Agent Tasks (internal/usecase/skill/, internal/skills/)

*   **Task 6.1: Skill Registry & Execution Service** [Status: COMPLETE]
    *   Define `SkillRegistry` interface in `internal/usecase/skill/registry.go`. (Definition: COMPLETE)
    *   Implement a `SkillExecutionService` in `internal/usecase/skill/service.go` to handle skill discovery, registration, and execution, including permission checks and timeouts. (Implementation: COMPLETE)
    *   Write unit tests for skill execution flow. (Tests: COMPLETE)
*   **Task 6.2: Calculator Skill** [Status: PENDING]
    *   Create `internal/skills/calculator/calculator.go`.
    *   Implement `Skill` interface for a simple calculator (e.g., supporting `add`, `subtract`).
    *   Write unit tests for the calculator skill.
*   **Task 6.3: Datetime Skill** [Status: PENDING]
    *   Create `internal/skills/datetime/datetime.go`.
    *   Implement `Skill` interface for current date/time retrieval and basic formatting.
    *   Write unit tests for the datetime skill.
*   **Task 6.4: Skills System Configuration** [Status: COMPLETE]
    *   Define `SkillsSystemConfig` in `internal/config/skills.go` and integrate into `NuimanBotConfig`.

## 7. Memory & SQLite Agent Tasks (internal/usecase/memory/, internal/adapter/repository/sqlite/)

*   **Task 7.1: Memory Repository Interface** [Status: COMPLETE]
    *   Define `MemoryRepository` interface in `internal/usecase/memory/repository.go` with methods like `SaveMessage`, `GetConversation`, `DeleteConversation`.
*   **Task 7.2: SQLite User Repository** [Status: COMPLETE]
    *   Implement `UserRepository` interface (if defined) using SQLite in `internal/adapter/repository/sqlite/user.go`.
    *   Implement `CreateUser`, `GetUserByID`, `UpdateUser` methods.
    *   Write integration tests for SQLite user operations.
*   **Task 7.3: SQLite Message & Conversation Repository** [Status: COMPLETE]
    *   Implement `MessageRepository` (or part of `MemoryRepository`) using SQLite in `internal/adapter/repository/sqlite/message.go`.
    *   Implement `SaveMessage`, `GetConversation`, `GetRecentMessages` methods.
    *   Write integration tests for SQLite message/conversation operations.
*   **Task 7.4: Storage Configuration** [Status: COMPLETE]
    *   Define `StorageConfig` in `internal/config/storage.go` and integrate into `NuimanBotConfig`.

## 8. Configuration Agent Tasks (internal/config/)

*   **Task 8.1: Configuration Loader** [Status: PENDING]
    *   Implement a configuration loading service in `internal/config/loader.go` (e.g., using `viper`) to read `config.yaml` and environment variables.
    *   Handle `SecureString` types by decrypting values on load using `SecurityService`.
    *   Provide methods to retrieve specific configuration sections.
    *   Write unit tests for config loading and decryption.
*   **Task 8.2: Integrate all Configs** [Status: PENDING]
    *   Ensure all individual `*Config` structs (`ServerConfig`, `SecurityConfig`, `LLMConfig`, `GatewaysConfig`, `MCPConfig`, `StorageConfig`, `SkillsSystemConfig`, `MemoryConfig`, `ExternalAPIConfig`, `ToolsConfig`) are correctly defined and integrated into `NuimanBotConfig`.

## 9. Quality Assurance Agent Tasks (Cross-Cutting)

*   **Task 9.1: Test Coverage Enforcement** [Status: PENDING]
    *   Set up tooling to measure and enforce test coverage targets (90% domain, 85% usecase, 80% adapter, 75% infrastructure, 80% overall).
    *   Integrate coverage checks into CI.
*   **Task 9.2: End-to-End Test (CLI -> LLM -> Skill -> CLI)** [Status: PENDING]
    *   Develop an E2E test to simulate a full interaction: User input via CLI -> `ChatService` -> `LLMService` -> `SkillExecutionService` (e.g., calculator) -> `LLMService` response -> CLI output.
*   **Task 9.3: Security Test Scenarios** [Status: PENDING]
    *   Create tests for input validation, credential handling, and audit logging to ensure security mitigations are effective.

## 10. Integration Lead / Architect Agent (Final Assembly)

*   **Task 10.1: Main Application Assembly** [Status: PENDING]
    *   In `cmd/nuimanbot/main.go`, assemble all implemented components using dependency injection.
    *   Initialize configuration, security services, memory, LLM service, skill service, and CLI gateway.
    *   Start the CLI gateway.
*   **Task 10.2: Error Handling & Graceful Shutdown** [Status: PENDING]
    *   Implement robust error handling and graceful shutdown mechanisms.

This detailed task breakdown, combined with the sub-agent collaboration model and strict adherence to architectural principles, aims to streamline development and minimize integration challenges.
