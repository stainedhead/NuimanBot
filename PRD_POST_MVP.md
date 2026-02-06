# NuimanBot Post-MVP Product Requirements Document
 
> Next-phase roadmap for enterprise readiness, advanced skills, and document intelligence
 
**Version:** 1.0  
**Last Updated:** 2026-02-06  
**Status:** Draft  
 
---
 
## Table of Contents
 
1. [Executive Summary](#1-executive-summary)  
2. [Goals and Non-Goals](#2-goals-and-non-goals)  
3. [User Roles and Permissions](#3-user-roles-and-permissions)  
4. [System Architecture (Post-MVP Additions)](#4-system-architecture-post-mvp-additions)  
5. [LLM Provider Expansion](#5-llm-provider-expansion)  
6. [Skills System Expansion](#6-skills-system-expansion)  
7. [Document Intelligence (RAG)](#7-document-intelligence-rag)  
8. [Browser Automation](#8-browser-automation)  
9. [Post-MVP Phases](#9-post-mvp-phases)  
10. [Verification Strategy](#10-verification-strategy)  
11. [Open Questions](#11-open-questions)  
12. [Appendix: Configuration Updates](#12-appendix-configuration-updates)  
 
---
 
## 1. Executive Summary
 
Post-MVP focuses on enterprise-grade model providers, advanced skills for engineering workflows, and document intelligence (RAG) across local, Git, and cloud sources. This phase keeps the existing security posture and Clean Architecture constraints while adding targeted automation and developer tooling.
 
Primary outcomes:
 
- Expand skill coverage for research, architecture planning, implementation, and review.
- Add enterprise-ready LLM provider (AWS Bedrock).
- Deliver RAG indexing/search/retrieval with strong access control and auditability.
 
---
 
## 2. Goals and Non-Goals
 
### Goals
 
| Goal | Metric |
|------|--------|
| Enterprise LLM readiness | AWS Bedrock supported with BYOK and audit controls |
| Research/engineering skill coverage | 8+ new skills available with RBAC gating |
| RAG capability | Index/search/retrieve across local, Git, and cloud sources |
| Security parity | No reduction in input validation, audit logging, or credential protections |
| Clean Architecture compliance | New features follow inward dependency rules |
 
### Non-Goals
 
| Non-Goal | Rationale |
|----------|-----------|
| External skill marketplace | Maintains security posture |
| Auto-updating external binaries | Avoids supply-chain risk |
| Web UI admin panel | CLI-first still preferred |
| Cross-tenant memory sharing | Privacy isolation requirement |
 
---
 
## 3. User Roles and Permissions
 
Existing roles remain (`admin`, `user`). Post-MVP expands permissions for new skill categories:
 
| Permission | Description |
|------------|-------------|
| `network` | External API calls (LLMs, search, cloud storage) |
| `shell` | Local command execution (admin only) |
| `browser` | Browser automation (admin only by default) |
| `documents` | Document indexing and retrieval |
 
RBAC will enforce per-skill allowlists as in the MVP design.
 
---
 
## 4. System Architecture (Post-MVP Additions)
 
Post-MVP adds adapters and infrastructure for document indexing, cloud source connectors, and browser automation while keeping dependencies flowing inward.
 
```
internal/
├── domain/
│   ├── document.go            # Document, Chunk, Source entities
│   └── retrieval.go           # Query and retrieval contracts
├── usecase/
│   ├── document/              # Index/search/retrieve orchestration
│   └── browser/               # Automation orchestration (policies + retries)
├── adapter/
│   ├── repository/
│   │   └── vector/             # Vector store adapter (SQLite/pgvector)
│   └── gateway/
│       └── storage/            # S3/Drive/Git workspace connectors
└── infrastructure/
    ├── llm/bedrock/            # AWS Bedrock client
    ├── browser/                # Playwright/Selenium runners
    └── retrieval/              # Embedding + chunking engine
```
 
---
 
## 5. LLM Provider Expansion
 
### AWS Bedrock
 
**Requirements**
- Support Bedrock runtime (invoke model, streaming where available).
- Credential sources: AWS profile, environment variables, IAM role.
- Model mapping in config (`bedrock/claude-3-5-sonnet`, `bedrock/titan-text`, etc.).
 
**Acceptance**
- Configurable AWS region and credentials.
- Tool calling + streaming support for supported models.
- Audit logs include provider and model metadata.
 
---
 
## 6. Skills System Expansion
 
### 6.1 Skills with Reference Implementations (HomeBots)
 
| Skill | Purpose | Reference |
|-------|---------|-----------|
| `coding_agent` | Orchestrate Codex/Claude Code/OpenCode CLI runs | OpenClaw `coding-agent` |
| `cron` | Scheduled reminders and recurring tasks | NanoBot `cron` |
| `gemini` | One-shot Gemini CLI queries | OpenClaw `gemini` |
| `github` | GitHub operations via `gh` CLI | OpenClaw/NanoBot `github` |
| `sag` | ElevenLabs TTS via `sag` | OpenClaw `sag` |
| `summarize` | Summarize URLs/files/YouTube | OpenClaw `summarize` |
| `repo_search` | Fast codebase search via ripgrep | Internal proposal (BOT_TOOL_IDEAS) |
| `doc_summarize` | Summaries of internal docs and links | Internal proposal (BOT_TOOL_IDEAS) |
| `goog` | Google Workspace CLI (Gmail/Calendar/Drive) | OpenClaw `gog` / go-goog-cli |
 
### 6.2 Skills Requiring New Research/Implementation
 
| Skill | Scope | Research Notes |
|-------|-------|----------------|
| `selenium` | Browser automation for QA/research tasks | Evaluate Selenium WebDriver + Go bindings |
| `playwright` | Headless browser automation | Evaluate Playwright Go vs. Node sidecar |
| `puppeteer` | Node-based browser automation | Evaluate hosted sidecar process |
| `doc_index` | Ingest and embed documents | See RAG section |
| `doc_search` | Query ranked chunks | See RAG section |
| `doc_retrieve` | Fetch full docs/snippets | See RAG section |
 
---
 
## 7. Document Intelligence (RAG)
 
### Scope
- Index and retrieve from local filesystem, Git repos/workspaces, and cloud storage (S3/Drive).
- Support common formats: Markdown, text, PDF, Office, HTML.
 
### Core Skills
 
| Skill | Function |
|-------|----------|
| `doc_index` | Ingest documents, chunk, embed, and store |
| `doc_search` | Query by text and return ranked snippets |
| `doc_retrieve` | Fetch full document or exact chunk by ID |
 
### Functional Requirements
- Configurable sources per user/role with allowlists.
- Embedding model selection via LLM provider abstraction.
- Vector store backend: SQLite for dev, pgvector for production.
- Citations with source metadata (path, repo, URL, timestamp).
 
### Security Requirements
- Per-source RBAC controls and auditing.
- Sensitive file redaction rules.
- Max file size and rate limits for indexing.
 
---
 
## 8. Browser Automation
 
### Requirements
- Headless runs for research/QA tasks.
- Scriptable flows with step limits and timeouts.
- Output artifacts: screenshots, HTML snapshots, and logs.
 
### Constraints
- Admin-only by default (`browser` permission).
- Network allowlist and domain restrictions.
- Explicit user confirmation for destructive actions.
 
---
 
## 9. Post-MVP Phases
 
### Phase 5: Developer Productivity Skills
 
| Task | Description | Status |
|------|-------------|--------|
| Add `github` skill | `gh` CLI integration + RBAC | ☐ |
| Add `repo_search` skill | Ripgrep-based codebase search | ☐ |
| Add `doc_summarize` skill | Summaries for internal docs | ☐ |
| Add `summarize` skill | External URL/file summarization | ☐ |
| Add `coding_agent` skill | Orchestrate external coding agents | ☐ |
 
### Phase 6: Scheduling + Voice
 
| Task | Description | Status |
|------|-------------|--------|
| Add `cron` skill | Reminders and recurring tasks | ☐ |
| Add `sag` skill | ElevenLabs TTS responses | ☐ |
 
### Phase 7: Enterprise Providers
 
| Task | Description | Status |
|------|-------------|--------|
| AWS Bedrock provider | SDK integration + streaming | ☐ |
 
### Phase 8: RAG + Automation
 
| Task | Description | Status |
|------|-------------|--------|
| `doc_index/search/retrieve` | Index + query docs | ☐ |
| Browser automation | Selenium/Playwright/Puppeteer | ☐ |
| `goog` skill | Google Workspace workflows | ☐ |
 
---
 
## 10. Verification Strategy
 
- Unit tests for new skills (schema validation, error handling).
- Integration tests for Bedrock provider adapter.
- E2E RAG tests: index → search → retrieve across all source types.
- Security tests: RBAC denial, allowlist enforcement, audit logging.
 
---
 
## 11. Open Questions
 
- Preferred default embedding model for RAG (Bedrock, OpenAI, or local)?
- Browser automation priority: Playwright vs. Selenium vs. Puppeteer as first-class?
 
---
 
## 12. Appendix: Configuration Updates
 
```yaml
llm:
  providers:
    - id: bedrock-primary
      type: bedrock
      name: AWS Bedrock (prod)
      aws_region: us-east-1
      aws_profile: default
skills:
  entries:
    github:
      enabled: true
    repo_search:
      enabled: true
    doc_index:
      enabled: false
```
