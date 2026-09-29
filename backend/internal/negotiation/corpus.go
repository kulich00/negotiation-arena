package negotiation

import (
	"context"
	"fmt"
	"time"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
	"github.com/kulich00/negotiation-arena/backend/internal/repository"
)

func (s *Service) ExportCorpus(ctx context.Context, filter repository.SessionFilter) (domain.CorpusPage, error) {
	page, err := s.repo.ListSessions(ctx, filter)
	if err != nil {
		return domain.CorpusPage{}, err
	}
	export := domain.CorpusPage{
		SchemaVersion: domain.CorpusSchemaVersion,
		GeneratedAt:   time.Now().UTC(),
		Items:         make([]domain.CorpusDialogue, 0, len(page.Items)),
		Total:         page.Total,
		Limit:         page.Limit,
		Offset:        page.Offset,
	}
	for _, summary := range page.Items {
		detail, err := s.SessionDetail(ctx, summary.ID)
		if err != nil {
			return domain.CorpusPage{}, err
		}
		dialogue := buildCorpusDialogue(detail)
		export.Items = append(export.Items, dialogue)
	}
	return export, nil
}

func buildCorpusDialogue(detail domain.SessionDetail) domain.CorpusDialogue {
	checkpoints := make(map[int]domain.TurnCheckpoint, len(detail.Checkpoints))
	for _, checkpoint := range detail.Checkpoints {
		checkpoints[checkpoint.Turn] = checkpoint
	}
	dialogue := domain.CorpusDialogue{
		SchemaVersion: domain.CorpusSchemaVersion, Origin: "runtime", DialogueID: detail.Session.ID,
		ParentDialogueID: detail.Session.ParentSessionID, ForkedFromTurn: detail.Session.ForkedFromTurn,
		Status: detail.Session.Status, StartedAt: detail.Session.StartedAt, FinishedAt: detail.Session.FinishedAt,
		Scenario: detail.Scenario, Result: detail.Result, Turns: []domain.CorpusTurn{},
		Quality: domain.CorpusQuality{Complete: true, Issues: []string{}},
	}
	startIndex := 1
	if len(detail.Messages) == 0 || detail.Messages[0].Sender != "opponent" {
		dialogue.InitialMessage = domain.CorpusMessage{Role: "opponent", Text: detail.Session.InitialMessage, CreatedAt: detail.Session.StartedAt}
		dialogue.Quality = addCorpusIssue(dialogue.Quality, "missing_initial_message")
		startIndex = 0
	} else {
		dialogue.InitialMessage = corpusMessage(detail.Messages[0])
	}
	remaining := len(detail.Messages) - startIndex
	if remaining%2 != 0 {
		dialogue.Quality = addCorpusIssue(dialogue.Quality, "incomplete_final_turn")
		remaining--
	}
	for offset := 0; offset < remaining; offset += 2 {
		index := startIndex + offset
		turn := (index + 1) / 2
		if startIndex == 0 {
			turn = offset/2 + 1
		}
		player, opponent := detail.Messages[index], detail.Messages[index+1]
		if player.Sender != "player" || opponent.Sender != "opponent" {
			dialogue.Quality = addCorpusIssue(dialogue.Quality, fmt.Sprintf("unexpected_roles_turn_%d", turn))
		}
		before, beforeOK := checkpoints[turn-1]
		after, afterOK := checkpoints[turn]
		var beforeSnapshot, afterSnapshot *domain.CorpusSnapshot
		if beforeOK {
			snapshot := corpusSnapshot(before)
			beforeSnapshot = &snapshot
		} else {
			dialogue.Quality = addCorpusIssue(dialogue.Quality, fmt.Sprintf("missing_before_checkpoint_turn_%d", turn))
		}
		if afterOK {
			snapshot := corpusSnapshot(after)
			afterSnapshot = &snapshot
		} else {
			dialogue.Quality = addCorpusIssue(dialogue.Quality, fmt.Sprintf("missing_after_checkpoint_turn_%d", turn))
		}
		analysis := domain.TurnAnalysis{
			Intent: "unknown", Technique: "unknown", InterpretationSource: "unknown", ReplySource: "unknown",
			Strengths: []string{}, Risks: []string{}, Errors: []domain.NegotiationError{},
		}
		if player.Analysis != nil {
			analysis = *player.Analysis
		} else {
			dialogue.Quality = addCorpusIssue(dialogue.Quality, fmt.Sprintf("missing_analysis_turn_%d", turn))
		}
		if analysis.InterpretationSource == "" {
			analysis.InterpretationSource = "unknown"
			dialogue.Quality = addCorpusIssue(dialogue.Quality, fmt.Sprintf("missing_interpretation_source_turn_%d", turn))
		}
		if analysis.ReplySource == "" {
			analysis.ReplySource = "unknown"
			dialogue.Quality = addCorpusIssue(dialogue.Quality, fmt.Sprintf("missing_reply_source_turn_%d", turn))
		}
		if analysis.Strengths == nil {
			analysis.Strengths = []string{}
		}
		if analysis.Risks == nil {
			analysis.Risks = []string{}
		}
		if analysis.Errors == nil {
			analysis.Errors = []domain.NegotiationError{}
		}
		dialogue.Turns = append(dialogue.Turns, domain.CorpusTurn{
			Turn: turn, Player: corpusMessage(player), Opponent: corpusMessage(opponent),
			Analysis: analysis, Before: beforeSnapshot, After: afterSnapshot,
		})
	}
	if len(dialogue.Turns) == 0 {
		dialogue.Quality = addCorpusIssue(dialogue.Quality, "no_player_turns")
	}
	return dialogue
}

func addCorpusIssue(quality domain.CorpusQuality, issue string) domain.CorpusQuality {
	quality.Complete = false
	quality.Issues = append(quality.Issues, issue)
	return quality
}

func corpusMessage(message domain.Message) domain.CorpusMessage {
	return domain.CorpusMessage{Role: message.Sender, Text: message.Content, CreatedAt: message.CreatedAt}
}

func corpusSnapshot(checkpoint domain.TurnCheckpoint) domain.CorpusSnapshot {
	return domain.CorpusSnapshot{
		TrustScore: checkpoint.TrustScore, ArgumentScore: checkpoint.ArgumentScore,
		PressureScore: checkpoint.PressureScore, State: checkpoint.State,
	}
}
