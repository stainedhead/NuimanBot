package cli_test

import (
	"bytes"
	"testing"

	"nuimanbot/internal/adapter/gateway/cli"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

func TestGateway_SetAdminHandler(t *testing.T) {
	cfg := &config.CLIConfig{}
	g := cli.NewGateway(cfg)

	// Should not panic
	g.SetAdminHandler(nil)
}

func TestGateway_SetProfileHandler(t *testing.T) {
	cfg := &config.CLIConfig{}
	g := cli.NewGateway(cfg)

	g.SetProfileHandler(nil)
}

func TestGateway_SetBotHandler(t *testing.T) {
	cfg := &config.CLIConfig{}
	g := cli.NewGateway(cfg)

	g.SetBotHandler(nil)
}

func TestGateway_SetCurrentUser(t *testing.T) {
	cfg := &config.CLIConfig{}
	g := cli.NewGateway(cfg)

	user := &domain.User{ID: "user-1", Role: domain.RoleAdmin}
	g.SetCurrentUser(user)
}

func TestGateway_SetMemoryHandler(t *testing.T) {
	cfg := &config.CLIConfig{}
	g := cli.NewGateway(cfg)

	g.SetMemoryHandler(nil)
}

func TestGateway_SetConfigHandler(t *testing.T) {
	cfg := &config.CLIConfig{}
	g := cli.NewGateway(cfg)

	g.SetConfigHandler(nil)
}

func TestGateway_SetSkillHandler(t *testing.T) {
	cfg := &config.CLIConfig{}
	output := new(bytes.Buffer)
	g := cli.NewGateway(cfg)

	mockExec := &MockSkillExecutor{}
	h := cli.NewSkillHandler(mockExec, output)
	g.SetSkillHandler(h)
}
