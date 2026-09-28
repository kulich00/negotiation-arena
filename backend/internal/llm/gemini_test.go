package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGeminiGeneratorGenerateReply(t *testing.T) {
	var received geminiRequest
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("method = %s", request.Method)
		}
		if request.URL.Path != "/models/gemini-2.5-flash:generateContent" {
			t.Errorf("path = %s", request.URL.Path)
		}
		if request.Header.Get("x-goog-api-key") != "test-key" {
			t.Error("missing API key header")
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Errorf("decode request: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"  Готов обсудить этот вариант.  "}]}}]}`))
	}))
	defer server.Close()

	generator, err := NewGeminiGenerator(GeminiConfig{
		APIKey: "test-key", BaseURL: server.URL, Client: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	reply, err := generator.GenerateReply(context.Background(), ReplyRequest{
		BaseReply: "Базовый ответ", PlayerMessage: "Предложение игрока",
		OpponentRole: "Заказчик", OpponentGoal: "Снизить риск",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply != "Готов обсудить этот вариант." {
		t.Fatalf("reply = %q", reply)
	}
	if len(received.Contents) != 1 || !strings.Contains(received.Contents[0].Parts[0].Text, "Базовый ответ") {
		t.Fatalf("deterministic reply was not sent: %+v", received)
	}
	if len(received.SystemInstruction.Parts) != 1 || received.GenerationConfig.MaxOutputTokens != 256 {
		t.Fatalf("unexpected Gemini request: %+v", received)
	}
	if received.GenerationConfig.ThinkingConfig == nil || received.GenerationConfig.ThinkingConfig.ThinkingBudget == nil || *received.GenerationConfig.ThinkingConfig.ThinkingBudget != 0 {
		t.Fatalf("thinking must be disabled for short Gemini 2.5 replies: %+v", received.GenerationConfig)
	}
}

func TestGeminiGeneratorRejectsFailedAndEmptyResponses(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{name: "http error", statusCode: http.StatusTooManyRequests, body: `{"error":{"message":"quota"}}`},
		{name: "empty candidates", statusCode: http.StatusOK, body: `{"candidates":[]}`},
		{name: "blocked", statusCode: http.StatusOK, body: `{"promptFeedback":{"blockReason":"SAFETY"}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.statusCode)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()
			generator, err := NewGeminiGenerator(GeminiConfig{APIKey: "secret-key", BaseURL: server.URL, Client: server.Client()})
			if err != nil {
				t.Fatal(err)
			}
			_, err = generator.GenerateReply(context.Background(), ReplyRequest{BaseReply: "reply"})
			if err == nil {
				t.Fatal("expected error")
			}
			if strings.Contains(err.Error(), "secret-key") {
				t.Fatal("error exposed API key")
			}
		})
	}
}

func TestGeminiGeneratorInterpretsMoveAsStructuredJSON(t *testing.T) {
	var received geminiRequest
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Errorf("decode request: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"{\"intent\":\"pressure\",\"proposalValue\":0,\"alternativeId\":\"\"}"}]}}]}`))
	}))
	defer server.Close()
	generator, err := NewGeminiGenerator(GeminiConfig{APIKey: "test-key", BaseURL: server.URL, Client: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	interpretation, err := generator.InterpretMove(context.Background(), InterpretationRequest{
		Message: "Сделайте как я сказал, иначе пожалеете", ScenarioTopic: "Сроки",
	})
	if err != nil {
		t.Fatal(err)
	}
	if interpretation.Intent != "pressure" || interpretation.ProposalValue != 0 || interpretation.AlternativeID != "" {
		t.Fatalf("unexpected interpretation: %+v", interpretation)
	}
	config := received.GenerationConfig
	if config.ResponseMIMEType != "application/json" || config.ResponseSchema == nil {
		t.Fatalf("structured output was not requested: %+v", config)
	}
	if config.ThinkingConfig == nil || config.ThinkingConfig.ThinkingBudget == nil || *config.ThinkingConfig.ThinkingBudget != 0 {
		t.Fatalf("thinking must be disabled for classification: %+v", config)
	}
}

func TestGeminiGeneratorRotatesKeysBetweenRequests(t *testing.T) {
	var receivedKeys []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedKeys = append(receivedKeys, request.Header.Get("x-goog-api-key"))
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"reply"}]}}]}`))
	}))
	defer server.Close()

	generator, err := NewGeminiGenerator(GeminiConfig{
		APIKeys: []string{"key-1", "key-2", "key-3"},
		BaseURL: server.URL,
		Client:  server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	for range 4 {
		if _, err := generator.GenerateReply(context.Background(), ReplyRequest{BaseReply: "reply"}); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"key-1", "key-2", "key-3", "key-1"}
	if strings.Join(receivedKeys, ",") != strings.Join(want, ",") {
		t.Fatalf("key order = %v, want %v", receivedKeys, want)
	}
}

func TestGeminiGeneratorTriesNextKeyAfterQuotaError(t *testing.T) {
	var receivedKeys []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		key := request.Header.Get("x-goog-api-key")
		receivedKeys = append(receivedKeys, key)
		if key == "exhausted-key" {
			writer.WriteHeader(http.StatusTooManyRequests)
			_, _ = writer.Write([]byte(`{"error":{"message":"quota exhausted"}}`))
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"reply"}]}}]}`))
	}))
	defer server.Close()

	generator, err := NewGeminiGenerator(GeminiConfig{
		APIKeys: []string{"exhausted-key", "working-key"},
		BaseURL: server.URL,
		Client:  server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := generator.GenerateReply(context.Background(), ReplyRequest{BaseReply: "reply"}); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"exhausted-key", "working-key", "working-key"}
	if strings.Join(receivedKeys, ",") != strings.Join(want, ",") {
		t.Fatalf("key attempts = %v, want %v", receivedKeys, want)
	}
}

func TestGeminiGeneratorSkipsKeysDuringQuotaCooldown(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requestCount++
		writer.Header().Set("Retry-After", "60")
		writer.WriteHeader(http.StatusTooManyRequests)
		_, _ = writer.Write([]byte(`{"error":{"message":"quota exhausted"}}`))
	}))
	defer server.Close()

	generator, err := NewGeminiGenerator(GeminiConfig{
		APIKeys: []string{"key-1", "key-2"}, BaseURL: server.URL, Client: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := generator.GenerateReply(context.Background(), ReplyRequest{BaseReply: "reply"}); err == nil {
		t.Fatal("expected quota error")
	}
	if requestCount != 2 {
		t.Fatalf("initial request count = %d, want 2", requestCount)
	}
	_, err = generator.GenerateReply(context.Background(), ReplyRequest{BaseReply: "reply"})
	if err == nil || !strings.Contains(err.Error(), "temporarily unavailable") {
		t.Fatalf("expected cooldown error, got %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("cooldown made extra provider requests: %d", requestCount)
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, time.September, 28, 12, 0, 0, 0, time.UTC)
	if got := parseRetryAfter("12", now); got != 12*time.Second {
		t.Fatalf("seconds Retry-After = %s", got)
	}
	if got := parseRetryAfter(now.Add(30*time.Second).Format(http.TimeFormat), now); got != 30*time.Second {
		t.Fatalf("date Retry-After = %s", got)
	}
	if got := parseRetryAfter("invalid", now); got != 0 {
		t.Fatalf("invalid Retry-After = %s", got)
	}
}

func TestGeminiGeneratorRotatesKeysSafelyAcrossConcurrentRequests(t *testing.T) {
	keys := []string{"key-1", "key-2", "key-3", "key-4"}
	counts := make(map[string]int, len(keys))
	var countsMu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		countsMu.Lock()
		counts[request.Header.Get("x-goog-api-key")]++
		countsMu.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"reply"}]}}]}`))
	}))
	defer server.Close()

	generator, err := NewGeminiGenerator(GeminiConfig{
		APIKeys: keys,
		BaseURL: server.URL,
		Client:  server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}

	const requestCount = 40
	errorsChannel := make(chan error, requestCount)
	var requests sync.WaitGroup
	requests.Add(requestCount)
	for range requestCount {
		go func() {
			defer requests.Done()
			_, requestErr := generator.GenerateReply(context.Background(), ReplyRequest{BaseReply: "reply"})
			errorsChannel <- requestErr
		}()
	}
	requests.Wait()
	close(errorsChannel)
	for requestErr := range errorsChannel {
		if requestErr != nil {
			t.Fatal(requestErr)
		}
	}
	for _, key := range keys {
		if counts[key] != requestCount/len(keys) {
			t.Fatalf("requests with %s = %d, want %d", key, counts[key], requestCount/len(keys))
		}
	}
}

func TestNewGeminiGeneratorNormalizesAndLimitsKeyPool(t *testing.T) {
	generator, err := NewGeminiGenerator(GeminiConfig{
		APIKeys: []string{" key-1 ", "", "key-1", "key-2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if generator.APIKeyCount() != 2 {
		t.Fatalf("APIKeyCount = %d, want 2", generator.APIKeyCount())
	}

	tooMany := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}
	if _, err := NewGeminiGenerator(GeminiConfig{APIKeys: tooMany}); err == nil {
		t.Fatal("expected an error for more than 8 unique API keys")
	}
}

func TestFallbackReplyGeneratorUsesDeterministicReply(t *testing.T) {
	generator := NewFallbackReplyGenerator(
		replyGeneratorFunc(func(context.Context, ReplyRequest) (string, error) {
			return "", errors.New("provider unavailable")
		}),
		NewPassthroughReplyGenerator(), nil,
	)
	reply, err := generator.GenerateReply(context.Background(), ReplyRequest{BaseReply: "deterministic"})
	if err != nil {
		t.Fatal(err)
	}
	if reply != "deterministic" {
		t.Fatalf("reply = %q", reply)
	}
}

func TestNewGeminiGeneratorRequiresAPIKey(t *testing.T) {
	if _, err := NewGeminiGenerator(GeminiConfig{}); err == nil {
		t.Fatal("expected missing API key error")
	}
}

type replyGeneratorFunc func(context.Context, ReplyRequest) (string, error)

func (function replyGeneratorFunc) GenerateReply(ctx context.Context, request ReplyRequest) (string, error) {
	return function(ctx, request)
}
