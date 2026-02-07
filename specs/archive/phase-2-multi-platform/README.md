# Phase 2: Multi-Platform - Specification Documents

**Phase:** 2 (Multi-Platform)
**Status:** 📋 Planning Complete - Ready for Implementation
**Started:** 2026-02-06
**Target:** 4-5 weeks

---

## Overview

Phase 2 extends NuimanBot from a single-platform CLI tool to a multi-platform conversational agent with:

- **3 Messaging Gateways:** CLI, Telegram, Slack
- **3 LLM Providers:** Anthropic, OpenAI, Ollama
- **5 Skills:** calculator, datetime, weather, web_search, notes
- **Full RBAC:** Role-based skill access control
- **User Management:** Admin commands for user lifecycle

---

## Document Overview

This directory contains all planning documents for Phase 2, following the progressive documentation workflow from AGENTS.md.

### 1. [spec.md](./spec.md) - Feature Specification ✅
**Purpose:** Defines what Phase 2 will deliver

**Contents:**
- Feature requirements and user stories
- Acceptance criteria for each feature
- Architecture changes
- Configuration changes
- Success metrics

**Read this first** to understand the scope and goals.

---

### 2. [research.md](./research.md) - Research Findings ✅
**Purpose:** Gathers information about libraries and APIs

**Contents:**
- Telegram Bot API integration guide
- Slack API Socket Mode guide
- OpenAI SDK usage examples
- Ollama API documentation
- OpenWeatherMap API examples
- DuckDuckGo search API
- Database schema extensions
- RBAC permission matrix
- Tool calling format mapping

**Read this** when implementing specific components to understand external dependencies.

---

### 3. [data-dictionary.md](./data-dictionary.md) - Data Structures ✅
**Purpose:** Defines all data types and schemas

**Contents:**
- Extended User entity with AllowedSkills
- Note entity definition
- Configuration structs for new gateways
- Configuration structs for new LLM providers
- Configuration structs for new skills
- Repository interfaces
- Service interfaces
- Database schemas
- Error types

**Reference this** when writing code to ensure consistent data structures.

---

### 4. [plan.md](./plan.md) - Implementation Plan ✅
**Purpose:** Outlines implementation approach and strategy

**Contents:**
- Development order and priorities
- Sub-agent assignments and responsibilities
- Detailed implementation plans for each component
- Testing strategy
- Database migrations
- Risk mitigation
- Quality gates
- Timeline (4-5 weeks)

**Read this** to understand how Phase 2 will be built and in what order.

---

### 5. [tasks.md](./tasks.md) - Task Breakdown ✅
**Purpose:** Breaks work into concrete, testable tasks

**Contents:**
- 63 detailed tasks organized by sub-agent
- Acceptance criteria for each task
- Test requirements for each task
- Time estimates
- Dependency tracking
- Progress tracking (☐/▸/✅/❌)

**Use this** as your implementation checklist. Mark tasks complete as you go.

---

### 6. [implementation-notes.md](./implementation-notes.md) - Living Document 📝
**Purpose:** Records decisions, gotchas, and lessons learned

**Contents:**
- Implementation decisions with rationale
- Gotchas and edge cases encountered
- Performance notes
- API quirks discovered
- Testing insights
- Refactoring opportunities

**Update this** during implementation as you learn and discover issues.

---

## Reading Order

### For Planning & Understanding:
1. **spec.md** - Understand what we're building
2. **plan.md** - Understand how we'll build it
3. **tasks.md** - See the detailed task list

### For Implementation:
1. **tasks.md** - Pick the next task
2. **data-dictionary.md** - Reference data structures
3. **research.md** - Reference API documentation
4. **implementation-notes.md** - Update with findings

---

## Quick Reference

### Phase 2 Priorities

**Priority 1 (Week 1):** Security & Foundation
- RBAC enforcement
- User management

**Priority 2 (Week 2):** Provider Expansion
- OpenAI provider
- Ollama provider

**Priority 3 (Week 3-4):** Gateway Expansion
- Telegram gateway
- Slack gateway

**Priority 4 (Week 5):** Skill Expansion
- Weather skill
- Web search skill
- Notes skill

---

## Implementation Workflow

### For Each Task:

1. **Read task from tasks.md**
   - Note acceptance criteria
   - Note test requirements

2. **Review relevant docs**
   - Check data-dictionary.md for types
   - Check research.md for API details
   - Check plan.md for implementation approach

3. **Follow TDD: Red-Green-Refactor**
   - Write failing test (Red)
   - Implement minimal code (Green)
   - Refactor for quality (Refactor) - **REQUIRED!**

4. **Run quality gates**
   - go fmt ./...
   - go vet ./...
   - golangci-lint run
   - go test ./...

5. **Update documentation**
   - Mark task ✅ in tasks.md
   - Add notes to implementation-notes.md
   - Update README.md if needed

---

## Success Criteria

Phase 2 is complete when:

- [ ] All 63 tasks in tasks.md are marked ✅
- [ ] All 3 gateways operational (CLI, Telegram, Slack)
- [ ] All 3 LLM providers operational (Anthropic, OpenAI, Ollama)
- [ ] All 5 skills operational (calculator, datetime, weather, web_search, notes)
- [ ] RBAC prevents unauthorized access (tested)
- [ ] User management CRUD works (tested)
- [ ] All quality gates pass
- [ ] Test coverage ≥75%
- [ ] Manual E2E testing successful
- [ ] Documentation updated

---

## Current Status

**Planning:** ✅ Complete (2026-02-06)
**Implementation:** ⏳ Not Started
**Testing:** ⏳ Not Started
**Documentation:** ⏳ Not Started

---

## Next Steps

1. ✅ Complete all planning documents
2. ⏳ **Start Task 1.1** - Define RBAC Permission Matrix
3. ⏳ Follow tasks.md in order
4. ⏳ Update implementation-notes.md as you go
5. ⏳ Mark tasks complete in tasks.md

---

## File Structure

```
specs/phase-2-multi-platform/
├── README.md                    # This file - overview and guide
├── spec.md                      # Feature specification
├── research.md                  # API and library research
├── data-dictionary.md           # Data structures and schemas
├── plan.md                      # Implementation plan
├── tasks.md                     # Detailed task breakdown (63 tasks)
└── implementation-notes.md      # Living log of decisions and learnings
```

---

## Key Metrics

- **Total Tasks:** 63
- **Estimated Time:** 18-25 working days (4-5 weeks)
- **New Files:** ~25+ (gateways, providers, skills, configs)
- **Modified Files:** ~15+ (main.go, configs, repos, etc.)
- **New Tests:** ~150+ test cases
- **New Dependencies:** 2 (go-telegram/bot, slack-go/slack, sashabaranov/go-openai)

---

## Dependencies

### External Libraries
- `github.com/go-telegram/bot` - Telegram Bot API
- `github.com/slack-go/slack` - Slack SDK
- `github.com/sashabaranov/go-openai` - OpenAI SDK

### External APIs
- OpenWeatherMap API (weather skill)
- DuckDuckGo Instant Answer API (web search skill)
- Telegram Bot API (telegram gateway)
- Slack API (slack gateway)
- OpenAI API (openai provider)
- Ollama API (ollama provider - local)

---

## References

- **Phase 1 Spec:** `specs/initial-mvp-spec/spec.md`
- **Architecture Guide:** `AGENTS.md`
- **Product Requirements:** `PRODUCT_REQUIREMENT_DOC.md`
- **Project README:** `README.md`
- **Project Status:** `STATUS.md`

---

## Questions or Issues?

1. Check implementation-notes.md for known issues
2. Review research.md for API documentation
3. Review plan.md for architectural decisions
4. Consult AGENTS.md for development guidelines

---

**Ready to begin Phase 2 implementation!** 🚀

Start with Task 1.1 in tasks.md: Define RBAC Permission Matrix
