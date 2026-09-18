package llm

import (
	"context"
	"strings"
)

type MockProvider struct{}

func NewMockProvider() MockProvider { return MockProvider{} }

func (MockProvider) Analyze(_ context.Context, request AnalysisRequest) (AnalysisResult, error) {
	message := strings.ToLower(request.Message)
	result := AnalysisResult{Reply: "Поясните, какую выгоду ваше предложение даёт обеим сторонам?"}

	if strings.Contains(message, "?") {
		result.TrustDelta++
		result.DetectedStrengths = append(result.DetectedStrengths, "Использованы вопросы")
	}
	if containsAny(message, "понимаю", "интерес", "компромисс", "предлагаю") {
		result.TrustDelta += 2
		result.Reply = "Готов обсудить компромисс. Какие условия вы считаете справедливыми?"
		result.DetectedStrengths = append(result.DetectedStrengths, "Поиск взаимной выгоды")
	}
	if containsAny(message, "результат", "показатель", "срок", "данные", "эффект") {
		result.ArgumentDelta += 2
		result.Reply = "Аргументы звучат конкретнее. Как вы предлагаете измерить результат?"
		result.DetectedStrengths = append(result.DetectedStrengths, "Конкретная аргументация")
	}
	if containsAny(message, "должны", "обязаны", "иначе", "требую") {
		result.PressureDelta += 2
		result.TrustDelta--
		result.Reply = "Давление не помогает договориться. Давайте вернёмся к фактам."
	}
	return result, nil
}

func containsAny(value string, variants ...string) bool {
	for _, variant := range variants {
		if strings.Contains(value, variant) {
			return true
		}
	}
	return false
}
