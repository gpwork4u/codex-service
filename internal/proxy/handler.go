package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"codex-service/internal/auth"
)

const codexEndpoint = "https://chatgpt.com/backend-api/codex/responses"

var codexModels = []string{
	"gpt-5.1-codex",
	"gpt-5.1-codex-max",
	"gpt-5.1-codex-mini",
	"gpt-5.2",
	"gpt-5.2-codex",
	"gpt-5.3-codex",
	"gpt-5.4",
	"gpt-5.4-mini",
}

type Handler struct {
	tm     *auth.TokenManager
	client *http.Client
}

func NewHandler(tm *auth.TokenManager) *Handler {
	return &Handler{
		tm: tm,
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// POST /v1/responses — passthrough proxy (force stream=true)
func (h *Handler) HandleResponses(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	// Force stream=true for Codex API
	var raw map[string]json.RawMessage
	json.Unmarshal(body, &raw)
	raw["stream"] = json.RawMessage("true")
	body, _ = json.Marshal(raw)

	var peek struct {
		Stream bool `json:"stream"`
	}
	json.Unmarshal(body, &peek)

	resp, err := h.doProxy(body)
	if err != nil {
		log.Printf("proxy error: %v", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
		return
	}

	relaySSEPassthrough(w, resp.Body)
}

// POST /v1/chat/completions — translate and proxy
func (h *Handler) HandleChatCompletions(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	// Parse to check stream flag and get model
	var peek struct {
		Stream bool   `json:"stream"`
		Model  string `json:"model"`
	}
	json.Unmarshal(body, &peek)

	// Transform chat completions → responses format
	transformed, err := chatCompletionsToResponses(body)
	if err != nil {
		log.Printf("transform error: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := h.doProxy(transformed)
	if err != nil {
		log.Printf("proxy error: %v", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
		return
	}

	if peek.Stream {
		transformer := makeSSETransformer(peek.Model)
		relaySSE(w, resp.Body, transformer)
		if f, ok := w.(http.Flusher); ok {
			fmt.Fprint(w, "data: [DONE]\n\n")
			f.Flush()
		}
	} else {
		// Codex API always streams, so collect SSE into a single response
		content, usage := collectSSE(resp.Body)
		chatResp := buildChatCompletionsResponse(peek.Model, content, usage)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResp)
	}
}

// GET /v1/models
func (h *Handler) HandleModels(w http.ResponseWriter, r *http.Request) {
	type model struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	}
	type modelsResponse struct {
		Object string  `json:"object"`
		Data   []model `json:"data"`
	}

	models := make([]model, len(codexModels))
	for i, id := range codexModels {
		models[i] = model{
			ID:      id,
			Object:  "model",
			Created: 1700000000,
			OwnedBy: "openai",
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(modelsResponse{
		Object: "list",
		Data:   models,
	})
}

func (h *Handler) doProxy(body []byte) (*http.Response, error) {
	accessToken, accountID, err := h.tm.GetToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	req, err := http.NewRequest("POST", codexEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	if accountID != "" {
		req.Header.Set("ChatGPT-Account-Id", accountID)
	}
	req.Header.Set("User-Agent", "codex-cli/1.0")
	req.Header.Set("originator", "codex")

	return h.client.Do(req)
}

