package llm

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

type countingObserver struct {
	operation string
	result    string
}

func (observer *countingObserver) ObserveLLM(operation, result string) {
	observer.operation, observer.result = operation, result
}

func TestArenaModelInterpreterUsesRemotePrediction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/interpret" || request.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"intent":"ask_interest","proposalValue":0,"alternativeId":"","relevant":true,"confidence":0.91,"modelVersion":"test"}`))
	}))
	defer server.Close()
	observer := &countingObserver{}
	interpreter, err := NewArenaModelInterpreter(ArenaModelConfig{BaseURL: server.URL, Client: server.Client(), MinConfidence: 0.6, Observer: observer})
	if err != nil {
		t.Fatal(err)
	}
	result, err := interpreter.InterpretMove(context.Background(), InterpretationRequest{Message: "Какие параметры отбора имеют значение?"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Intent != "ask_interest" || result.Source != "arena_model" || result.Relevant == nil || !*result.Relevant {
		t.Fatalf("unexpected interpretation: %+v", result)
	}
	if observer.operation != "arena_model_interpretation" || observer.result != "success" {
		t.Fatalf("unexpected metric: %+v", observer)
	}
}

func TestArenaModelInterpreterFallsBackOnLowConfidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"intent":"pressure","proposalValue":0,"alternativeId":"","relevant":true,"confidence":0.2,"modelVersion":"test"}`))
	}))
	defer server.Close()
	observer := &countingObserver{}
	interpreter, err := NewArenaModelInterpreter(ArenaModelConfig{BaseURL: server.URL, Client: server.Client(), MinConfidence: 0.6, Observer: observer})
	if err != nil {
		t.Fatal(err)
	}
	result, err := interpreter.InterpretMove(context.Background(), InterpretationRequest{Message: "Назовите параметры отбора"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Intent != "neutral" || result.Source != "local" {
		t.Fatalf("fallback was not used: %+v", result)
	}
	if observer.result != "fallback" {
		t.Fatalf("fallback metric was not recorded: %+v", observer)
	}
}

func TestArenaModelInterpreterRejectsInvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"intent":"invented","proposalValue":0,"alternativeId":"","relevant":true,"confidence":1,"modelVersion":"test"}`))
	}))
	defer server.Close()
	interpreter, err := NewArenaModelInterpreter(ArenaModelConfig{BaseURL: server.URL, Client: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	result, err := interpreter.InterpretMove(context.Background(), InterpretationRequest{Message: "Здравствуйте"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Intent != "neutral" || result.Source != "local" {
		t.Fatalf("invalid response did not fall back: %+v", result)
	}
}

func TestNewArenaModelInterpreterValidatesConfiguration(t *testing.T) {
	if _, err := NewArenaModelInterpreter(ArenaModelConfig{BaseURL: "file:///tmp/model"}); err == nil {
		t.Fatal("invalid URL was accepted")
	}
	if _, err := NewArenaModelInterpreter(ArenaModelConfig{BaseURL: "http://localhost:8090", MinConfidence: 1.1}); err == nil {
		t.Fatal("invalid confidence was accepted")
	}
	if _, err := NewArenaModelInterpreter(ArenaModelConfig{BaseURL: "http://localhost:8090", MinConfidence: math.NaN()}); err == nil {
		t.Fatal("non-finite confidence was accepted")
	}
}
