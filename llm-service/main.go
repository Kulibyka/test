package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	openrouter "github.com/revrost/go-openrouter"
)

var (
	fullPrompt string
	client     *openrouter.Client
)

func main() {
	systemPromptBytes, err := ioutil.ReadFile("systemprompt.txt")
	if err != nil {
		panic(fmt.Sprintf("Ошибка чтения systemprompt.txt: %v", err))
	}

	orgBytes, err := ioutil.ReadFile("org.json")
	if err != nil {
		panic(fmt.Sprintf("Ошибка чтения org.json: %v", err))
	}

	fullPrompt = strings.Replace(string(systemPromptBytes), "<ORGANIZATION_JSON>", string(orgBytes), 1)

	client = openrouter.NewClient(
		"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",
		openrouter.WithXTitle("My App"),
		openrouter.WithHTTPReferer("https://myapp.com"),
	)

	http.HandleFunc("/process", processHandler)

	fmt.Println("Сервер запущен на :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}

func processHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	userInput := string(body)

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
		http.Error(w, fmt.Sprintf("ChatCompletion error: %v", err), http.StatusInternalServerError)
		return
	}

	raw := resp.Choices[0].Message.Content.Text
	jsonOnly := extractJSON(raw)

	if !isValidJSON(jsonOnly) {
		http.Error(w, "invalid JSON from model", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(jsonOnly))
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
