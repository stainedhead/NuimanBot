package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	cliadapter "nuimanbot/internal/adapter/cli"
	"nuimanbot/internal/domain/memoryv2"
)

// MemoryCommandHandler handles memory CLI commands via the gateway.
type MemoryCommandHandler struct {
	cmd *cliadapter.MemoryCommand
}

// NewMemoryCommandHandler creates a new memory command handler.
func NewMemoryCommandHandler(cmd *cliadapter.MemoryCommand) *MemoryCommandHandler {
	return &MemoryCommandHandler{cmd: cmd}
}

// IsMemoryCommand checks if the input is a memory command.
func IsMemoryCommand(input string) bool {
	return strings.HasPrefix(input, "/memory ")
}

// HandleMemoryCommand processes a memory command.
func (h *MemoryCommandHandler) HandleMemoryCommand(ctx context.Context, input string) error {
	parts := strings.Fields(input)
	if len(parts) < 2 {
		return h.showHelp()
	}

	// Skip "/memory"
	subcommand := parts[1]
	args := parts[2:]

	switch subcommand {
	case "list":
		return h.handleList(ctx, args)
	case "get":
		return h.handleGet(ctx, args)
	case "search":
		return h.handleSearch(ctx, args)
	case "delete":
		return h.handleDelete(ctx, args)
	case "scenes":
		return h.handleScenes(ctx, args)
	case "prune":
		return h.cmd.Prune(ctx)
	case "help":
		return h.showHelp()
	default:
		return fmt.Errorf("unknown memory command: %s (try /memory help)", subcommand)
	}
}

func (h *MemoryCommandHandler) handleList(ctx context.Context, args []string) error {
	filter := memoryv2.MemoryCellFilter{}
	format := "table"

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--scene":
			if i+1 < len(args) {
				i++
				filter.Scene = args[i]
			}
		case "--conversation":
			if i+1 < len(args) {
				i++
				filter.ConversationID = args[i]
			}
		case "--type":
			if i+1 < len(args) {
				i++
				cellType, err := memoryv2.ParseCellType(args[i])
				if err != nil {
					return fmt.Errorf("invalid cell type: %s", args[i])
				}
				filter.CellType = &cellType
			}
		case "--limit":
			if i+1 < len(args) {
				i++
				limit, err := strconv.Atoi(args[i])
				if err != nil {
					return fmt.Errorf("invalid limit: %s", args[i])
				}
				filter.Limit = limit
			}
		case "--format":
			if i+1 < len(args) {
				i++
				format = args[i]
			}
		}
	}

	return h.cmd.List(ctx, filter, format)
}

func (h *MemoryCommandHandler) handleGet(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: /memory get <id> [--format json|table]")
	}

	id := args[0]
	format := "table"

	for i := 1; i < len(args); i++ {
		if args[i] == "--format" && i+1 < len(args) {
			i++
			format = args[i]
		}
	}

	return h.cmd.Get(ctx, id, format)
}

func (h *MemoryCommandHandler) handleSearch(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: /memory search <query> [--limit N] [--format json|table]")
	}

	// Collect query words (everything that isn't a flag)
	var queryParts []string
	limit := 20
	format := "table"

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--limit":
			if i+1 < len(args) {
				i++
				l, err := strconv.Atoi(args[i])
				if err != nil {
					return fmt.Errorf("invalid limit: %s", args[i])
				}
				limit = l
			}
		case "--format":
			if i+1 < len(args) {
				i++
				format = args[i]
			}
		default:
			queryParts = append(queryParts, args[i])
		}
	}

	query := strings.Join(queryParts, " ")
	if query == "" {
		return fmt.Errorf("search query cannot be empty")
	}

	return h.cmd.Search(ctx, query, limit, format)
}

func (h *MemoryCommandHandler) handleDelete(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: /memory delete <id>")
	}

	return h.cmd.Delete(ctx, args[0])
}

func (h *MemoryCommandHandler) handleScenes(ctx context.Context, args []string) error {
	format := "table"

	for i := 0; i < len(args); i++ {
		if args[i] == "--format" && i+1 < len(args) {
			i++
			format = args[i]
		}
	}

	return h.cmd.Scenes(ctx, format)
}

func (h *MemoryCommandHandler) showHelp() error {
	// The MemoryCommand already writes to its configured output,
	// so we just print help directly.
	fmt.Println("Memory commands:")
	fmt.Println("  /memory list [--scene S] [--conversation C] [--type T] [--limit N] [--format json|table]")
	fmt.Println("  /memory get <id> [--format json|table]")
	fmt.Println("  /memory search <query> [--limit N] [--format json|table]")
	fmt.Println("  /memory delete <id>")
	fmt.Println("  /memory scenes [--format json|table]")
	fmt.Println("  /memory prune")
	fmt.Println("  /memory help")
	return nil
}
