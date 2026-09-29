package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kulich00/negotiation-arena/backend/internal/llm"
	"github.com/kulich00/negotiation-arena/backend/internal/negotiation"
	"github.com/kulich00/negotiation-arena/backend/internal/repository"
)

type seedDialogue struct {
	id         string
	scenarioID string
	moves      []negotiation.PlayerMove
}

func main() {
	output := flag.String("output", "", "JSONL output path; stdout when empty")
	flag.Parse()
	if err := generate(*output); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(output string) error {
	ctx := context.Background()
	service := negotiation.NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(ctx); err != nil {
		return err
	}
	stableIDs := make(map[string]string)
	for _, dialogue := range seedDialogues() {
		session, err := service.StartSession(ctx, dialogue.scenarioID)
		if err != nil {
			return err
		}
		for _, move := range dialogue.moves {
			if _, err := service.ProcessMove(ctx, session.ID, move); err != nil {
				return fmt.Errorf("%s: %w", dialogue.scenarioID, err)
			}
		}
		stableIDs[session.ID] = dialogue.id
	}
	corpus, err := service.ExportCorpus(ctx, repository.SessionFilter{Limit: 100})
	if err != nil {
		return err
	}
	var target *os.File
	if output == "" {
		target = os.Stdout
	} else {
		target, err = os.Create(output)
		if err != nil {
			return err
		}
		defer target.Close()
	}
	encoder := json.NewEncoder(target)
	encoder.SetEscapeHTML(false)
	for _, dialogue := range corpus.Items {
		dialogue.Origin = "synthetic"
		dialogue.DialogueID = stableIDs[dialogue.DialogueID]
		if err := encoder.Encode(dialogue); err != nil {
			return err
		}
	}
	return nil
}

func seedDialogues() []seedDialogue {
	good := func(proposal *negotiation.MoveProposal) []negotiation.PlayerMove {
		return []negotiation.PlayerMove{
			{Content: "Расскажите, какие приоритеты и ограничения для вас важны?", Intent: negotiation.IntentAskInterest},
			{Content: "Как сейчас устроен процесс принятия решения?", Intent: negotiation.IntentAskSituation},
			{Content: "Что мешает получить нужный результат в текущих условиях?", Intent: negotiation.IntentIdentifyProblem},
			{Content: "К каким потерям приведёт сохранение этой проблемы?", Intent: negotiation.IntentExploreImplication},
			{Content: "Какую пользу даст решение этой проблемы?", Intent: negotiation.IntentClarifyNeedPayoff},
			{Content: "По данным пилота, срок сократился на 18 процентов.", Intent: negotiation.IntentPresentEvidence},
			{Content: "Если мы не договоримся, я рассмотрю сопоставимую альтернативу.", Intent: negotiation.IntentStateBATNA},
			{Content: "Предлагаю зафиксировать взаимовыгодные условия.", Intent: negotiation.IntentPropose, Proposal: proposal},
			{Content: "Подтверждаю согласованные условия.", Intent: negotiation.IntentAccept},
		}
	}
	recovery := func(proposal *negotiation.MoveProposal) []negotiation.PlayerMove {
		return []negotiation.PlayerMove{
			{Content: "Предлагаю сразу принять мои условия.", Intent: negotiation.IntentPropose, Proposal: proposal},
			{Content: "Вы обязаны согласиться сегодня.", Intent: negotiation.IntentPressure},
			{Content: "Давайте вернёмся к конструктиву: что для вас действительно важно?", Intent: negotiation.IntentAskInterest},
			{Content: "Тест на двух этапах показал снижение риска на 12 процентов.", Intent: negotiation.IntentPresentEvidence},
			{Content: "Как сейчас организован процесс?", Intent: negotiation.IntentAskSituation},
			{Content: "Какая проблема сильнее всего влияет на решение?", Intent: negotiation.IntentIdentifyProblem},
			{Content: "Что произойдёт, если проблему не устранить?", Intent: negotiation.IntentExploreImplication},
			{Content: "Какую выгоду даст согласованный вариант?", Intent: negotiation.IntentClarifyNeedPayoff},
			{Content: "Тогда предлагаю скорректированный вариант.", Intent: negotiation.IntentPropose, Proposal: proposal},
			{Content: "Готов подтвердить этот вариант.", Intent: negotiation.IntentAccept},
		}
	}
	harvard := func(proposal *negotiation.MoveProposal) []negotiation.PlayerMove {
		return []negotiation.PlayerMove{
			{Content: "Какие критерии определяют для вас хорошее соглашение?", Intent: negotiation.IntentAskInterest},
			{Content: "Замеры показывают улучшение результата на 15 процентов.", Intent: negotiation.IntentPresentEvidence},
			{Content: "Моя альтернатива — продолжить переговоры с другим вариантом.", Intent: negotiation.IntentStateBATNA},
			{Content: "Предлагаю вариант, который учитывает обозначенные критерии.", Intent: negotiation.IntentPropose, Proposal: proposal},
			{Content: "Согласен, фиксируем договорённость.", Intent: negotiation.IntentAccept},
		}
	}
	spinCorrection := func(proposal *negotiation.MoveProposal) []negotiation.PlayerMove {
		return []negotiation.PlayerMove{
			{Content: "К чему приведёт отсутствие решения?", Intent: negotiation.IntentExploreImplication},
			{Content: "Какую пользу даст изменение?", Intent: negotiation.IntentClarifyNeedPayoff},
			{Content: "Как сейчас устроена работа и какие ресурсы доступны?", Intent: negotiation.IntentAskSituation},
			{Content: "Какое препятствие мешает получить результат?", Intent: negotiation.IntentIdentifyProblem},
			{Content: "Какие издержки создаёт это препятствие?", Intent: negotiation.IntentExploreImplication},
			{Content: "Что изменится после устранения проблемы?", Intent: negotiation.IntentClarifyNeedPayoff},
			{Content: "Данные проверки подтверждают снижение затрат на 11 процентов.", Intent: negotiation.IntentPresentEvidence},
			{Content: "Предлагаю перейти к согласованным условиям.", Intent: negotiation.IntentPropose, Proposal: proposal},
			{Content: "Принимаю итоговый вариант.", Intent: negotiation.IntentAccept},
		}
	}
	type scenarioSeed struct {
		id       string
		proposal *negotiation.MoveProposal
	}
	scenarios := []scenarioSeed{
		{id: "vendor-introduction"},
		{id: "salary-negotiation", proposal: &negotiation.MoveProposal{AlternativeID: "review_in_3_months"}},
		{id: "project-deadline", proposal: &negotiation.MoveProposal{AlternativeID: "phased_delivery"}},
	}
	items := make([]seedDialogue, 0, len(scenarios)*4)
	for _, scenario := range scenarios {
		items = append(items,
			seedDialogue{id: "synthetic-v1-" + scenario.id + "-spin", scenarioID: scenario.id, moves: good(scenario.proposal)},
			seedDialogue{id: "synthetic-v1-" + scenario.id + "-recovery", scenarioID: scenario.id, moves: recovery(scenario.proposal)},
			seedDialogue{id: "synthetic-v1-" + scenario.id + "-harvard", scenarioID: scenario.id, moves: harvard(scenario.proposal)},
			seedDialogue{id: "synthetic-v1-" + scenario.id + "-spin-correction", scenarioID: scenario.id, moves: spinCorrection(scenario.proposal)},
		)
	}
	return items
}
