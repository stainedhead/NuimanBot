# Bot Tool Ideas

Tools/skills available across the HomeBots example repos. Descriptions are sourced from each skill's SKILL.md frontmatter.

| Tool/Skill | Description | Bots |
|---|---|---|
| `1password` | Set up and use 1Password CLI (op). Use when installing the CLI, enabling desktop app integration, signing in (single or multi-account), or reading/injecting/running secrets via op. | OpenClaw |
| `apple-notes` | Manage Apple Notes via the `memo` CLI on macOS (create, view, edit, delete, search, move, and export notes). Use when a user asks OpenClaw to add a note, list notes, search notes, or manage note folders. | OpenClaw |
| `apple-reminders` | Manage Apple Reminders via the `remindctl` CLI on macOS (list, add, edit, complete, delete). Supports lists, date filters, and JSON/plain output. | OpenClaw |
| `bear-notes` | Create, search, and manage Bear notes via grizzly CLI. | OpenClaw |
| `bird` | X/Twitter CLI for reading, searching, posting, and engagement via cookies. | OpenClaw |
| `blogwatcher` | Monitor blogs and RSS/Atom feeds for updates using the blogwatcher CLI. | OpenClaw |
| `blucli` | BluOS CLI (blu) for discovery, playback, grouping, and volume. | OpenClaw |
| `bluebubbles` | Use when you need to send or manage iMessages via BlueBubbles (recommended iMessage integration). Calls go through the generic message tool with channel="bluebubbles". | OpenClaw |
| `camsnap` | Capture frames or clips from RTSP/ONVIF cameras. | OpenClaw |
| `clawhub` | Use the ClawHub CLI to search, install, update, and publish agent skills from clawhub.com. Use when you need to fetch new skills on the fly, sync installed skills to latest or a specific version, or publish new/updated skill folders with the npm-installed clawhub CLI. | OpenClaw |
| `coding-agent` | Run Codex CLI, Claude Code, OpenCode, or Pi Coding Agent via background process for programmatic control. | OpenClaw |
| `cron` | Schedule reminders and recurring tasks. | NanoBot |
| `discord` | Use when you need to control Discord from OpenClaw via the discord tool: send messages, react, post or upload stickers, upload emojis, run polls, manage threads/pins/search, create/edit/delete channels and categories, fetch permissions or member/role/channel info, set bot presence/activity, or handle moderation actions in Discord DMs or channels. | OpenClaw |
| `eightctl` | Control Eight Sleep pods (status, temperature, alarms, schedules). | OpenClaw |
| `food-order` | Reorder Foodora orders + track ETA/status with ordercli. Never confirm without explicit user approval. Triggers: order food, reorder, track ETA. | OpenClaw |
| `gemini` | Gemini CLI for one-shot Q&A, summaries, and generation. | OpenClaw |
| `gifgrep` | Search GIF providers with CLI/TUI, download results, and extract stills/sheets. | OpenClaw |
| `github` | Interact with GitHub using the `gh` CLI. Use `gh issue`, `gh pr`, `gh run`, and `gh api` for issues, PRs, CI runs, and advanced queries. | NanoBot, OpenClaw |
| `gog` | Google Workspace CLI for Gmail, Calendar, Drive, Contacts, Sheets, and Docs. | OpenClaw |
| `goplaces` | Query Google Places API (New) via the goplaces CLI for text search, place details, resolve, and reviews. Use for human-friendly place lookup or JSON output for scripts. | OpenClaw |
| `healthcheck` | Host security hardening and risk-tolerance configuration for OpenClaw deployments. Use when a user asks for security audits, firewall/SSH/update hardening, risk posture, exposure review, OpenClaw cron scheduling for periodic checks, or version status checks on a machine running OpenClaw (laptop, workstation, Pi, VPS). | OpenClaw |
| `himalaya` | CLI to manage emails via IMAP/SMTP. Use `himalaya` to list, read, write, reply, forward, search, and organize emails from the terminal. Supports multiple accounts and message composition with MML (MIME Meta Language). | OpenClaw |
| `imsg` | iMessage/SMS CLI for listing chats, history, watch, and sending. | OpenClaw |
| `local-places` | Search for places (restaurants, cafes, etc.) via Google Places API proxy on localhost. | OpenClaw |
| `mcporter` | Use the mcporter CLI to list, configure, auth, and call MCP servers/tools directly (HTTP or stdio), including ad-hoc servers, config edits, and CLI/type generation. | OpenClaw |
| `model-usage` | Use CodexBar CLI local cost usage to summarize per-model usage for Codex or Claude, including the current (most recent) model or a full model breakdown. Trigger when asked for model-level usage/cost data from codexbar, or when you need a scriptable per-model summary from codexbar cost JSON. | OpenClaw |
| `nano-banana-pro` | Generate or edit images via Gemini 3 Pro Image (Nano Banana Pro). | OpenClaw |
| `nano-pdf` | Edit PDFs with natural-language instructions using the nano-pdf CLI. | OpenClaw |
| `notion` | Notion API for creating and managing pages, databases, and blocks. | OpenClaw |
| `obsidian` | Work with Obsidian vaults (plain Markdown notes) and automate via obsidian-cli. | OpenClaw |
| `openai-image-gen` | Batch-generate images via OpenAI Images API. Random prompt sampler + `index.html` gallery. | OpenClaw |
| `openai-whisper` | Local speech-to-text with the Whisper CLI (no API key). | OpenClaw |
| `openai-whisper-api` | Transcribe audio via OpenAI Audio Transcriptions API (Whisper). | OpenClaw |
| `openhue` | Control Philips Hue lights/scenes via the OpenHue CLI. | OpenClaw |
| `oracle` | Best practices for using the oracle CLI (prompt + file bundling, engines, sessions, and file attachment patterns). | OpenClaw |
| `ordercli` | Foodora-only CLI for checking past orders and active order status (Deliveroo WIP). | OpenClaw |
| `peekaboo` | Capture and automate macOS UI with the Peekaboo CLI. | OpenClaw |
| `qr-code` | Generate and read QR codes. Use when the user wants to create a QR code from text/URL, or decode/read a QR code from an image file. Supports PNG/JPG output and can read QR codes from screenshots or image files. | OpenClaw |
| `sag` | ElevenLabs text-to-speech with mac-style say UX. | OpenClaw |
| `scheduling` | Create scheduled tasks and one-off reminders. Use for recurring jobs (daily, weekly) or future reminders (in 5 minutes, tomorrow at 3pm). | LettaBot |
| `session-logs` | Search and analyze your own session logs (older/parent conversations) using jq. | OpenClaw |
| `sherpa-onnx-tts` | Local text-to-speech via sherpa-onnx (offline, no cloud) | OpenClaw |
| `skill-creator` | Create or update AgentSkills. Use when designing, structuring, or packaging skills with scripts, references, and assets. | NanoBot, OpenClaw |
| `slack` | Use when you need to control Slack from OpenClaw via the slack tool, including reacting to messages or pinning/unpinning items in Slack channels or DMs. | OpenClaw |
| `songsee` | Generate spectrograms and feature-panel visualizations from audio with the songsee CLI. | OpenClaw |
| `sonoscli` | Control Sonos speakers (discover/status/play/volume/group). | OpenClaw |
| `spotify-player` | Terminal Spotify playback/search via spogo (preferred) or spotify_player. | OpenClaw |
| `summarize` | Summarize or extract text/transcripts from URLs, podcasts, and local files (great fallback for “transcribe this YouTube/video”). | NanoBot, OpenClaw |
| `things-mac` | Manage Things 3 via the `things` CLI on macOS (add/update projects+todos via URL scheme; read/search/list from the local Things database). Use when a user asks OpenClaw to add a task to Things, list inbox/today/upcoming, search tasks, or inspect projects/areas/tags. | OpenClaw |
| `tmux` | Remote-control tmux sessions for interactive CLIs by sending keystrokes and scraping pane output. | NanoBot, OpenClaw |
| `trello` | Manage Trello boards, lists, and cards via the Trello REST API. | OpenClaw |
| `video-frames` | Extract frames or short clips from videos using ffmpeg. | OpenClaw |
| `voice-call` | Start voice calls via the OpenClaw voice-call plugin. | OpenClaw |
| `wacli` | Send WhatsApp messages to other people or search/sync WhatsApp history via the wacli CLI (not for normal user chats). | OpenClaw |
| `weather` | Get current weather and forecasts (no API key required). | NanoBot, OpenClaw |

## Suggested Skills for Research, Architecture, Implementation, and Review

| Tool/Skill | Description | Bots |
|---|---|---|
| `repo-search` | Fast codebase search (ripgrep + symbol lookup) for research and impact analysis. | Proposed |
| `doc-summarize` | Summarize internal docs/external links into concise briefs. | Proposed |
| `architecture-map` | Generate architecture and dependency maps; flag Clean Architecture violations. | Proposed |
| `adr-generator` | Create/update Architecture Decision Records with templates and decision logs. | Proposed |
| `test-runner` | Run targeted Go tests, coverage, and E2E suites. | Proposed |
| `lint-vet` | Orchestrate go fmt, vet, and golangci-lint. | Proposed |
| `pr-review` | Produce diff summaries, risks, and change-impact checklists. | Proposed |
| `dependency-audit` | Run govulncheck and dependency/license audits. | Proposed |
| `sqlite-inspect` | Query notes/users/messages data for debugging and validation. | Proposed |
