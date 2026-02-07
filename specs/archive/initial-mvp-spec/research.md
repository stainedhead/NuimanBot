# NuimanBot Initial MVP Research Findings

This document summarizes key research findings and external references influencing the NuimanBot Initial MVP, based on the `PRODUCT_REQUIREMENT_DOC.md` (`specs/initial-mvp-spec/spec.md`) and the planned implementation strategy.

## 1. Security-First Approach Justification

The core motivation for NuimanBot is to address critical security vulnerabilities prevalent in existing AI agent frameworks.

*   **Community Skill Vulnerabilities**: The PRD highlights that "26% of community skills in similar platforms contain security vulnerabilities including credential leakage, prompt injection enabling RCE, and supply chain attacks." This statistic underscores the absolute necessity of NuimanBot's "Custom skills only: No external skill imports" policy and rigorous security controls.
*   **Threat Model & Mitigations**: The detailed threat model in the PRD (Credential leakage, Prompt injection, Malicious skills, Session hijacking, Privilege escalation, Supply chain attacks) and their corresponding mitigations (AES-256-GCM encryption, Input sanitization, Output sandboxing, Custom skills only, RBAC, Minimal dependencies, Audit logging) form the foundation of our security research. Each of these mitigations requires careful implementation and validation.

## 2. Architectural Principles (Clean Architecture & TDD)

*   **Clean Architecture**: The project strictly adheres to Clean Architecture as detailed in `AGENTS.md` and `specs/initial-mvp-spec/plan.md`. This choice is fundamental to maintain separation of concerns, testability, and long-term maintainability, which indirectly contributes to security by reducing complexity.
    *   **Reference**: `AGENTS.md`
*   **Test-Driven Development (TDD)**: As mandated by `AGENTS.md`, TDD will be the primary development methodology. This ensures that features are correctly implemented from the outset and provides a safety net for refactoring, crucial for security-sensitive components.

## 3. Go-Specific Implementation Considerations

*   **Go Plugin System (`plugin` package) for Skills**:
    *   **Research Finding**: The PRD states, "NuimanBot will leverage Go's plugin system (`plugin` package) to dynamically load skills from compiled shared objects (`.so` files)."
    *   **Implications**: The `plugin` package in Go has specific platform limitations (only Linux, FreeBSD, macOS, and Solaris are supported) and requires plugins to be compiled with the same Go compiler version and flags as the main application. This will impact deployment and potentially development environments, necessitating clear documentation and build processes for skills. This approach, while adding build complexity, enhances security by ensuring skills are compiled and linked within a controlled environment, preventing easy injection of arbitrary code.
*   **Dependency Management**: Go Modules (`go.mod`, `go.sum`) for consistent and reproducible builds.

## 4. Key External Libraries and Protocols

### 4.1. Messaging Gateways

*   **CLI**: Standard Go `os` package for input/output and `bufio` for reading lines. `chzyer/readline` or similar for REPL features (command history, autocompletion).
*   **Telegram**: `github.com/go-telegram/bot`. This library will be used for Telegram integration (Phase 2), supporting long-polling and webhooks.
*   **Slack**: `github.com/slack-go/slack`. Used for Slack integration (Phase 2), specifically Socket Mode to avoid exposing public endpoints.

### 4.2. LLM Providers

*   **Anthropic**: `github.com/anthropics/anthropic-sdk-go`. The official Go SDK for Claude models.
*   **OpenAI**: `github.com/openai/openai-go`. The official Go SDK for GPT models.
*   **Ollama**: Direct HTTP API interaction using Go's `net/http` package. Requires understanding Ollama's local API endpoints and request/response formats.
*   **Bedrock**: `github.com/aws/aws-sdk-go` (specifically the Bedrock runtime and model packages). This is a newer addition mentioned in the PRD's `LLMConfig`. Secure integration will require managing AWS credentials and region configurations.

### 4.3. Data Persistence

*   **SQLite**: `github.com/mattn/go-sqlite3`. Selected for MVP for its simplicity and embedded nature, suitable for development and single-server deployments.

### 4.4. Model Context Protocol (MCP)

*   **Reference**: The PRD specifies compliance with the "[2025-11-25 specification](https://modelcontextprotocol.io/specification/2025-11-25)". This external specification is critical for MCP Server and Client modes. Research into the specific details of this protocol is required for accurate implementation.

## 5. Configuration Management

*   **Approach**: The plan includes using a library like `spf13/viper` for loading configuration from YAML files and environment variables, providing robust and flexible configuration handling.
*   **Security Integration**: `SecureString` types within the configuration will require integration with the `SecurityService` for decryption upon loading.

## 6. Testing Strategy

*   **Unit/Integration/E2E**: The PRD outlines a comprehensive testing strategy covering unit, integration, and end-to-end tests, including specific security tests. This research confirms the need for a multi-faceted testing approach.
*   **Coverage**: Target coverage metrics are clearly defined (e.g., 90% domain, 80% overall), necessitating continuous monitoring and enforcement.

This research document serves as a guide for understanding the underlying technical decisions and external dependencies for the NuimanBot MVP.
