package service

import (
	"context"
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/domain"
)

type mockRepo struct {
	domain.Repository
	choreCreated bool
	chore        *domain.Chore
}

func (m *mockRepo) CreateChore(ctx context.Context, chore *domain.Chore) error {
	m.choreCreated = true
	m.chore = chore
	return nil
}

func TestChoreService_CreateChore(t *testing.T) {
	repo := &mockRepo{}
	service := NewChoreService(repo)

	desc := "test chore"
	dur := 1 * time.Hour

	chore, err := service.CreateChore(context.Background(), desc, dur)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if chore == nil {
		t.Fatal("expected chore to be returned")
	}

	if !repo.choreCreated {
		t.Error("expected chore to be created in repo")
	}

	if repo.chore.Description != desc {
		t.Errorf("expected description %s, got %s", desc, repo.chore.Description)
	}
}

func TestChoreService_AssignChore(t *testing.T) {
	repo := &mockRepo{}
	service := NewChoreService(repo)

	err := service.AssignChore(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
