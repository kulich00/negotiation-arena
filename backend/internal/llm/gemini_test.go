package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
