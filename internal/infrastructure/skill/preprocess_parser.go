package skill

import (
	"bufio"
	"strings"
	"time"

	"nuimanbot/internal/domain"
)

// CommandBlock represents a command block with its position in the source
type CommandBlock struct {
	Command  domain.PreprocessCommand
	StartPos int
	EndPos   int
}

// PreprocessParser parses !command blocks from skill markdown
type PreprocessParser struct {
	defaultTimeout time.Duration
}

// NewPreprocessParser creates a new preprocessing parser
func NewPreprocessParser() *PreprocessParser {
	return &PreprocessParser{
		defaultTimeout: domain.MaxCommandTimeout,
	}
}

// Parse extracts preprocessing commands from skill markdown
func (p *PreprocessParser) Parse(content string) ([]domain.PreprocessCommand, error) {
	blocks, err := p.ParseWithPositions(content)
	if err != nil {
		return nil, err
	}

	commands := make([]domain.PreprocessCommand, len(blocks))
	for i, block := range blocks {
		commands[i] = block.Command
	}

	return commands, nil
}

// ParseWithPositions extracts commands with their positions in the source
func (p *PreprocessParser) ParseWithPositions(content string) ([]CommandBlock, error) {
	var blocks []CommandBlock

	scanner := bufio.NewScanner(strings.NewReader(content))
	var currentCommand strings.Builder
	var inCommandBlock bool
	startPos := 0
	currentPos := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineLength := len(line) + 1 // +1 for newline

		// Check if this is a !command line
		if strings.TrimSpace(line) == "!command" {
			// Save any previous command
			if inCommandBlock && currentCommand.Len() > 0 {
				blocks = append(blocks, CommandBlock{
					Command: domain.PreprocessCommand{
						Command: strings.TrimSpace(currentCommand.String()),
						Timeout: p.defaultTimeout,
					},
					StartPos: startPos,
					EndPos:   currentPos,
				})
				currentCommand.Reset()
			}

			// Start new command block
			inCommandBlock = true
			startPos = currentPos
			currentPos += lineLength
			continue
		}

		if inCommandBlock {
			// Empty line ends the command block
			if strings.TrimSpace(line) == "" {
				if currentCommand.Len() > 0 {
					blocks = append(blocks, CommandBlock{
						Command: domain.PreprocessCommand{
							Command: strings.TrimSpace(currentCommand.String()),
							Timeout: p.defaultTimeout,
						},
						StartPos: startPos,
						EndPos:   currentPos,
					})
					currentCommand.Reset()
				}
				inCommandBlock = false
			} else {
				// Add line to current command
				if currentCommand.Len() > 0 {
					currentCommand.WriteString("\n")
				}
				currentCommand.WriteString(line)
			}
		}

		currentPos += lineLength
	}

	// Handle command block at end of file
	if inCommandBlock && currentCommand.Len() > 0 {
		blocks = append(blocks, CommandBlock{
			Command: domain.PreprocessCommand{
				Command: strings.TrimSpace(currentCommand.String()),
				Timeout: p.defaultTimeout,
			},
			StartPos: startPos,
			EndPos:   currentPos,
		})
	}

	return blocks, scanner.Err()
}
