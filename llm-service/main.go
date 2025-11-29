package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	openrouter "github.com/revrost/go-openrouter"
)

var (
	fullPrompt   string
	client       *openrouter.Client
	stubResponse json.RawMessage
)

func main() {
	apiKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	rawStub := strings.TrimSpace(os.Getenv("LLM_STUB_RESPONSE"))

	if rawStub == "" {
		rawStub = `{"classification":"general","model_answer":{"summary":"Demo response","priority":"normal","next_steps":["Follow up with the sender","Schedule the requested call"]}}`
	}

	if !isValidJSON(rawStub) {
		log.Fatalf("LLM_STUB_RESPONSE is not valid JSON")
	}

	stubResponse = json.RawMessage(rawStub)

	if apiKey == "" {
		log.Print("OPENROUTER_API_KEY not provided — running in stub mode")
	} else {
		client = openrouter.NewClient(
			apiKey,
			openrouter.WithXTitle("My App"),
			openrouter.WithHTTPReferer("https://myapp.com"),
		)
	}
	systemPromptBytes, err := os.ReadFile("systemprompt.txt")
	if err != nil {
		log.Fatalf("Ошибка чтения systemprompt.txt: %v", err)
	}

	orgBytes, err := os.ReadFile("org.json")
	if err != nil {
		log.Fatalf("Ошибка чтения org.json: %v", err)
	}

	fullPrompt = strings.Replace(string(systemPromptBytes), "<ORGANIZATION_JSON>", string(orgBytes), 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/process", processHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := ":" + strings.TrimPrefix(port, ":")
	log.Printf("Сервер запущен на %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}

func processHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	userInput := string(body)

	if client == nil {
		writeStub(w)
		return
	}

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openrouter.ChatCompletionRequest{
			Model: "openai/gpt-4o",
			Messages: []openrouter.ChatCompletionMessage{
				openrouter.SystemMessage(fullPrompt),
				openrouter.UserMessage(userInput),
			},
			MaxTokens: 1500,
		},
	)
	if err != nil {
		log.Printf("ChatCompletion error, falling back to stub: %v", err)
		writeStub(w)
		return
	}

	raw := resp.Choices[0].Message.Content.Text
	jsonOnly := extractJSON(raw)

	if !isValidJSON(jsonOnly) {
		http.Error(w, "invalid JSON from model", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(jsonOnly))
}

func writeStub(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-LLM-Source", "stub")
	_, _ = w.Write(stubResponse)
}

func extractJSON(text string) string {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		return text[start : end+1]
	}
	return ""
}

func isValidJSON(text string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(text), &js) == nil
}
