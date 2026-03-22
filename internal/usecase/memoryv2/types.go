package memoryv2

import (
	"time"

	"nuimanbot/internal/domain/memoryv2"
)

// InteractionContext represents a completed interaction to be analyzed for memory extraction
type InteractionContext struct {
	ConversationID string
	UserMessage    string
	AssistantReply string
	ToolOutputs    []string
	MessageIDs     []string // For source attribution
	Timestamp      time.Time
}

// ExtractionRequest represents the LLM request for memory extraction
type ExtractionRequest struct {
	UserMessage    string   `json:"user_message"`
	AssistantReply string   `json:"assistant_reply"`
	ToolOutputs    []string `json:"tool_outputs,omitempty"`
}

// ExtractionResponse represents the LLM response with extracted memory cells
type ExtractionResponse struct {
	Cells []ExtractedCell `json:"cells"`
}

// ExtractedCell represents a memory cell extracted by the LLM
type ExtractedCell struct {
	Scene    string   `json:"scene"`
	CellType string   `json:"cell_type"`
	Salience float64  `json:"salience"`
	Content  string   `json:"content"`
	Source   []string `json:"source"`
}

// CurationResult represents the result of memory curation
type CurationResult struct {
	CellsCreated  int
	CellsSkipped  int // Cells skipped due to deduplication
	ScenesUpdated int
	Errors        []error
}

// RecallRequest represents a request for memory recall
type RecallRequest struct {
	ConversationID string
	Query          string
	MaxTokens      int // Token budget for memory injection
	MaxCells       int // Maximum cells to retrieve
}

// RecallResponse represents retrieved memory for injection into context
type RecallResponse struct {
	Cells           []*memoryv2.MemoryCell
	Scenes          []*memoryv2.MemoryScene
	TotalTokens     int
	FTSMatchCount   int // Number of FTS matches
	FallbackUsed    bool
	RetrievalTimeMs int64
}

// SceneConsolidationRequest represents a request to consolidate a scene summary
type SceneConsolidationRequest struct {
	Scene           string
	Cells           []*memoryv2.MemoryCell
	MaxTokens       int
	ExistingSummary string // Empty if scene is new
}

// SceneConsolidationResponse represents the consolidated scene summary
type SceneConsolidationResponse struct {
	Summary    string
	TokenCount int
}

// CuratorConfig defines configuration for the memory curator service
type CuratorConfig struct {
	Enabled               bool
	ExtractionModel       string // LLM model for extraction (e.g., "claude-3-haiku-20240307")
	ConsolidationModel    string // LLM model for scene consolidation
	MaxCellsPerExtraction int
	RetryOnInvalidJSON    bool
	SceneSummaryMaxTokens int
}

// RecallConfig defines configuration for the memory recall service
type RecallConfig struct {
	FTSResultLimit    int     // Max FTS results to retrieve
	SalienceThreshold float64 // Min salience for fallback retrieval
	FallbackCellLimit int     // Max cells to retrieve via salience fallback
	MaxScenes         int     // Max scenes to include
	TokenBudget       int     // Total token budget for memory injection
}
