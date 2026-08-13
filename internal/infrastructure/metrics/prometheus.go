package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP Metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// LLM Metrics
	LLMRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_requests_total",
			Help: "Total number of LLM requests",
		},
		[]string{"provider", "model", "status"},
	)

	LLMRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "llm_request_duration_seconds",
			Help:    "LLM request duration in seconds",
			Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0, 60.0},
		},
		[]string{"provider", "model"},
	)

	LLMTokensUsed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_tokens_used_total",
			Help: "Total LLM tokens used",
		},
		[]string{"provider", "model", "type"},
	)

	LLMCostUSD = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_cost_usd_total",
			Help: "Total LLM cost in USD",
		},
		[]string{"provider", "model"},
	)

	// Skill Metrics
	SkillExecutionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skill_executions_total",
			Help: "Total number of skill executions",
		},
		[]string{"skill", "status"},
	)

	SkillExecutionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "skill_execution_duration_seconds",
			Help:    "Skill execution duration in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1.0, 2.0, 5.0},
		},
		[]string{"skill"},
	)

	// Cache Metrics
	CacheHitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache_type"},
	)

	CacheMissesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache_type"},
	)

	CacheEvictionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_evictions_total",
			Help: "Total number of cache evictions",
		},
		[]string{"cache_type"},
	)

	// Database Metrics
	DBQueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_queries_total",
			Help: "Total number of database queries",
		},
		[]string{"operation", "status"},
	)

	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
		},
		[]string{"operation"},
	)

	DBConnectionsOpen = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_open",
			Help: "Number of open database connections",
		},
	)

	DBConnectionsIdle = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_idle",
			Help: "Number of idle database connections",
		},
	)

	// Memory Metrics
	MemoryExtractionTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "memory_extraction_total",
			Help: "Total number of memory extractions",
		},
		[]string{"status"},
	)

	MemoryExtractionDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "memory_extraction_duration_seconds",
			Help:    "Memory extraction duration in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1.0, 2.0, 5.0, 10.0},
		},
	)

	MemoryCellsCreatedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "memory_cells_created_total",
			Help: "Total number of memory cells created",
		},
	)

	MemoryConsolidationTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "memory_consolidation_total",
			Help: "Total number of scene consolidations",
		},
		[]string{"status"},
	)

	MemoryConsolidationDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "memory_consolidation_duration_seconds",
			Help:    "Scene consolidation duration in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1.0, 2.0, 5.0, 10.0},
		},
	)

	MemoryRecallTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "memory_recall_total",
			Help: "Total number of memory recalls",
		},
		[]string{"status", "query_type"},
	)

	MemoryRecallDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "memory_recall_duration_seconds",
			Help:    "Memory recall duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
		},
	)

	MemoryRecallCellsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "memory_recall_cells_total",
			Help: "Total number of memory cells recalled",
		},
	)

	MemoryFTSQueryDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "memory_fts_query_duration_seconds",
			Help:    "FTS query duration in seconds",
			Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1},
		},
	)

	// MemoryRecallExpiredCellsSkipped counts cells skipped during recall because
	// their expires_at has passed. A steadily increasing value indicates expired cells
	// are accumulating in the Ingatan backend (see R-08 and support_docs/ingatan-operations.md).
	MemoryRecallExpiredCellsSkipped = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "memory_recall_expired_cells_skipped_total",
			Help: "Total number of memory cells skipped during recall because they are expired",
		},
	)

	// Rate Limiting Metrics
	RateLimitExceeded = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rate_limit_exceeded_total",
			Help: "Total number of rate limit violations",
		},
		[]string{"user_id", "action"},
	)

	// Security Metrics
	SecurityValidationFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "security_validation_failures_total",
			Help: "Total number of input validation failures",
		},
		[]string{"reason"},
	)

	AuditEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "audit_events_total",
			Help: "Total number of audit events",
		},
		[]string{"action", "outcome"},
	)

	// Buzz Gateway Metrics
	//
	// BuzzRelayConnections was originally declared as a GaugeVec labeled
	// relay_url/status, but the adapter layer (per FR-004's decision to set
	// it from buzz.Gateway via nostr.Client.ConnectedRelayCount(), not from
	// internal/infrastructure/nostr directly) only has an aggregate
	// currently-connected-relay count available, not per-relay connect
	// state. Re-scoped to a plain Gauge of that count rather than carrying
	// unused labels.
	BuzzRelayConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "buzz_relay_connections",
			Help: "Current number of connected Buzz relays",
		},
	)

	// BuzzEventsReceivedTotal's help text originally described "verified Buzz
	// events received" generically, but it is only ever incremented in
	// processChannelMessage for kind:9 channel messages — not for the
	// kind:9000/kind:10100 agent-status events that also flow through the
	// same verified pipeline in processEvent (FR-013). Re-scoped the help
	// text to make that explicit, rather than adding a companion counter for
	// a volume nothing currently needs to alert on.
	BuzzEventsReceivedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "buzz_events_received_total",
			Help: "Total number of verified Buzz kind:9 channel messages received (excludes kind:9000/kind:10100 agent-status events)",
		},
		[]string{"channel_id", "sender_is_agent"},
	)

	BuzzEventsPublishedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "buzz_events_published_total",
			Help: "Total number of Buzz events published",
		},
		[]string{"status"},
	)

	BuzzSignatureVerificationFailuresTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "buzz_signature_verification_failures_total",
			Help: "Total number of Buzz events dropped for failing signature verification",
		},
	)

	// Web Chats Environment Metrics — recorded in
	// internal/adapter/web/chats_handler.go around chatsService.SendMessage,
	// mirroring the Buzz gateway's dedicated per-integration counter/
	// histogram pattern above (a generic HTTP-level metric wouldn't
	// distinguish an agent turn that failed from one that simply wasn't
	// sent, or separate LLM-processing latency from ordinary page-render
	// latency).
	WebChatTurnsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "web_chat_turns_total",
			Help: "Total number of web Chats environment agent turns processed",
		},
		[]string{"status"}, // "success" | "error"
	)

	WebChatTurnDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "web_chat_turn_duration_seconds",
			Help:    "Duration of web Chats environment agent turns (SendMessage call to reply)",
			Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0, 60.0},
		},
	)
)
