package domain

import "testing"

func TestProject_AgentsFilePath(t *testing.T) {
	p := &Project{OutputDirectory: "/home/user/projects/widget"}
	got := p.AgentsFilePath()
	want := "/home/user/projects/widget/AGENTS.md"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestProject_AgentsFilePath_EmptyOutputDirectory(t *testing.T) {
	p := &Project{}
	if got := p.AgentsFilePath(); got != "" {
		t.Fatalf("expected empty path for empty OutputDirectory, got %q", got)
	}
}
