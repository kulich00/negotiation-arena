package negotiation

import (
	"context"
	"testing"

	"github.com/kulich00/negotiation-arena/backend/internal/llm"
	"github.com/kulich00/negotiation-arena/backend/internal/repository"
)

func TestNegotiationFlow(t *testing.T) {
	service := NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	service.SeedDefaults()
	session, err := service.StartSession("salary-negotiation")
	if err != nil {
		t.Fatal(err)
	}
	turn, err := service.ProcessMessage(context.Background(), session.ID, "Предлагаю компромисс. Какие интересы для вас важны?")
	if err != nil {
		t.Fatal(err)
	}
	if turn.Session.TrustScore <= 50 {
		t.Fatalf("expected trust to grow, got %d", turn.Session.TrustScore)
	}
	result, err := service.Finish(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.FinalScore == 0 {
		t.Fatal("expected non-zero score")
	}
}
