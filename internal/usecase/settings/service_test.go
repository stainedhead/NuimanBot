package settings

import "testing"

type fakePool struct {
	concurrency  int
	setCallCount int
}

func (f *fakePool) SetConcurrency(n int) {
	f.setCallCount++
	f.concurrency = n
}

func (f *fakePool) Concurrency() int { return f.concurrency }

type fakeSkills struct {
	names []string
}

func (f *fakeSkills) SkillNames() []string { return f.names }

func TestService_WorkerPoolSize(t *testing.T) {
	pool := &fakePool{concurrency: 3}
	s := NewService(pool, &fakeSkills{}, RetentionDefaults{})
	if got := s.WorkerPoolSize(); got != 3 {
		t.Fatalf("expected 3, got %d", got)
	}
}

func TestService_SetWorkerPoolSize_Valid(t *testing.T) {
	pool := &fakePool{concurrency: 3}
	s := NewService(pool, &fakeSkills{}, RetentionDefaults{})
	if err := s.SetWorkerPoolSize(10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pool.concurrency != 10 {
		t.Fatalf("expected pool concurrency updated to 10, got %d", pool.concurrency)
	}
	if pool.setCallCount != 1 {
		t.Fatalf("expected SetConcurrency called once, got %d", pool.setCallCount)
	}
}

func TestService_SetWorkerPoolSize_NonPositiveRejected(t *testing.T) {
	pool := &fakePool{concurrency: 3}
	s := NewService(pool, &fakeSkills{}, RetentionDefaults{})
	for _, n := range []int{0, -1, -100} {
		if err := s.SetWorkerPoolSize(n); err == nil {
			t.Errorf("expected error for n=%d", n)
		}
	}
	if pool.setCallCount != 0 {
		t.Fatalf("expected no SetConcurrency calls for invalid input, got %d", pool.setCallCount)
	}
}

func TestService_SkillNames(t *testing.T) {
	s := NewService(&fakePool{}, &fakeSkills{names: []string{"a", "b"}}, RetentionDefaults{})
	got := s.SkillNames()
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unexpected skill names: %v", got)
	}
}

func TestService_SkillNames_NilLister(t *testing.T) {
	s := NewService(&fakePool{}, nil, RetentionDefaults{})
	if got := s.SkillNames(); got != nil {
		t.Fatalf("expected nil for a nil SkillsLister, got %v", got)
	}
}

func TestService_RetentionDefaults(t *testing.T) {
	s := NewService(&fakePool{}, &fakeSkills{}, RetentionDefaults{ChatDays: 90, ProjectDays: 180, HistoryDays: 90})
	chat, project, history := s.RetentionDefaults()
	if chat != 90 || project != 180 || history != 90 {
		t.Fatalf("unexpected retention defaults: %d, %d, %d", chat, project, history)
	}
}
