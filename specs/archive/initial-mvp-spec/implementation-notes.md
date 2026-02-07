# NuimanBot Initial MVP Implementation Notes

This document captures anticipated implementation details, potential "gotchas," critical decisions, and best practices for the NuimanBot Initial MVP. It serves as a living document to guide the development sub-agents and ensure consistency with the `PRODUCT_REQUIREMENT_DOC.md` and the detailed `plan.md`.

## 1. Cross-Cutting Concerns & Architectural Adherence

### 1.1. Clean Architecture Strictness

*   **Note**: Strictly adhere to the Clean Architecture boundaries. Avoid direct imports from outer layers to inner layers. Enforce interfaces for all cross-layer communication.
*   **Gotcha**: Accidental direct dependencies between layers (e.g., `usecase` importing `adapter`) can easily occur. Rigorous code reviews and linting (`golangci-lint` configured for dependency checks) are essential.
*   **Decision**: Use dependency injection extensively from `cmd/nuimanbot/main.go` to construct the application graph. Avoid global variables for services.

### 1.2. Go Idiomatic Practices

*   **Note**: Prioritize idiomatic Go. Small interfaces, explicit error handling (`fmt.Errorf("context: %w", err)`), clear naming, and structured concurrency.
*   **Gotcha**: Over-engineering interfaces or creating large, generic interfaces (`interface{}`) that defeat the purpose of strong typing.
*   **Decision**: Keep interfaces minimal and define them where they are *used*, not where they are implemented.

### 1.3. Concurrency and `context.Context`

*   **Note**: Go's concurrency model (`goroutines` and `channels`) will be central to handling multiple messaging gateways, LLM calls, and skill executions. `context.Context` must be propagated through all API calls, service methods, and repository operations to manage deadlines, cancellations, and request-scoped values (e.g., `userID` for audit logs).
*   **Gotcha**: Forgetting to pass `context.Context` or ignoring its cancellation can lead to resource leaks or unresponsive services.
*   **Decision**: All service and repository methods must accept `context.Context` as their first argument.

## 2. Security Layer Implementation

### 2.1. `SecureString` Handling

*   **Note**: The `SecureString` type is crucial for handling sensitive data (API keys, tokens). Its `Zero()` method for memory zeroing should be called diligently when the sensitive data is no longer needed.
*   **Gotcha**: Unintentional copying of `SecureString` values, leading to sensitive data lingering in memory.
*   **Decision**: Encapsulate `SecureString` creation and usage, ensuring it's always handled safely (e.g., direct string conversion should be avoided, instead use `Value()` method when needed).

### 2.2. AES-256-GCM Key Management

*   **Note**: The `NUIMANBOT_ENCRYPTION_KEY` is a critical secret. It must be a cryptographically secure 32-byte key.
*   **Gotcha**: Hardcoding the key, using a weak key, or improper generation/distribution of the key.
*   **Decision**: The key will be loaded from an environment variable. Consider a robust secret management solution (e.g., HashiCorp Vault) for production beyond MVP, but for MVP, environment variable is acceptable.

### 2.3. Input Sanitization & Output Sandboxing

*   **Note**: The `SecurityService.ValidateInput` is essential for mitigating prompt injection and command injection.
*   **Gotcha**: Incomplete sanitization, leading to bypasses. Output sandboxing might involve escaping content before displaying to a user or passing to another system.
*   **Decision**: Implement a whitelist-based approach where possible for input validation. For output, use context-aware escaping mechanisms (e.g., HTML escaping for web contexts, JSON escaping for API responses).

## 3. Skills System Implementation

### 3.1. Go `plugin` Package Limitations

*   **Note**: The `plugin` package for dynamic skill loading is highly platform-dependent (Linux, macOS, FreeBSD, Solaris). It also requires the main application and plugins to be built with the exact same Go toolchain, including minor version and build flags. This is a significant constraint for cross-platform deployment.
*   **Gotcha**: Build failures due to mismatched Go versions or flags between the main executable and skill plugins. Not supported on Windows.
*   **Decision**: For MVP, acknowledge this limitation. For future phases, evaluate alternatives for broader platform support (e.g., WebAssembly, gRPC microservices for skills) if the `plugin` package becomes a blocker. Clear documentation on skill compilation is critical.

### 3.2. Skill Configuration & API Keys

*   **Note**: Skills can have their own `APIKey` and `Env` variables within `SkillConfig`. These must be loaded and managed securely via the `CredentialVault` and passed to the skill's execution context.
*   **Gotcha**: Leakage of skill-specific API keys or environment variables.
*   **Decision**: The `SkillExecutionService` will be responsible for securely injecting configuration and credentials into the skill's execution environment.

## 4. LLM Provider Abstraction

### 4.1. Unified `LLMService` Interface

*   **Note**: The `LLMService` interface is key to abstracting different LLM providers. Ensure `LLMRequest` and `LLMResponse` are comprehensive enough to capture common features (messages, tools, streaming) across providers.
*   **Gotcha**: Provider-specific features that don't fit the generic interface, leading to "leaky abstractions."
*   **Decision**: Initially, prioritize common features. For unique provider features, consider extending the `LLMRequest` with a `map[string]any` for `ProviderParams` or creating provider-specific request/response types if necessary, handled internally by the concrete client implementations.

### 4.2. Ollama Integration

*   **Note**: Ollama typically runs locally, often via an HTTP API. Direct `net/http` client integration will be required.
*   **Gotcha**: Network connectivity issues with the local Ollama instance, differing API versions.
*   **Decision**: Include robust error handling and configurability for Ollama's `BaseURL`.

## 5. Configuration Management

### 5.1. Dynamic Configuration Reloading

*   **Note**: While not explicitly MVP, the `external_api` section includes `POST /api/v1/config/reload`. This implies the configuration system should support dynamic reloading without restarting the application.
*   **Gotcha**: State management issues during reload, non-atomic updates, or incorrect application of new settings.
*   **Decision**: For MVP, focus on initial load. Future work for dynamic reload will require careful design to propagate changes safely to active services.

## 6. Testing Strategy

### 6.1. E2E Test Setup

*   **Note**: The MVP demands a comprehensive E2E test to validate the full flow (CLI input -> LLM -> Skill -> CLI output). This test will be complex.
*   **Gotcha**: Difficulty in mocking external services (LLMs) reliably for E2E tests.
*   **Decision**: Use integration test doubles (test fakes or stubs) for external LLM calls in E2E tests to ensure determinism and speed. Actual external calls can be covered in specific integration tests or dedicated external E2E suites.

## 7. Performance Considerations

*   **Note**: Golang's performance characteristics are generally good, but LLM calls and encryption can be computationally intensive.
*   **Gotcha**: Blocking operations within gateways or use cases leading to unresponsiveness.
*   **Decision**: Use `goroutines` and channels for asynchronous operations where appropriate. Implement timeouts for all external calls (LLM, network skills, disk I/O). Profile early if performance becomes a concern.

These notes highlight critical areas that sub-agents should pay close attention to during their implementation phases to ensure the NuimanBot MVP is robust, secure, and adheres to the defined architectural and quality standards.
