package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/ingres/ingres-agent-go/internal/httpclient"
	"github.com/ingres/ingres-agent-go/internal/prompts"
	apitypes "github.com/ingres/ingres-agent-go/internal/types"
	"github.com/ingres/ingres-agent-go/internal/utils"
)

type GeminiProvider struct {
	apiKey string
}

func NewGeminiProvider() *GeminiProvider {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		key = os.Getenv("OPENAI_API_KEY")
	}
	return &GeminiProvider{apiKey: key}
}

func (p *GeminiProvider) HandleUserQuery(ctx context.Context, userQuery string, previousChats []apitypes.ChatMessage) (string, bool, error) {
	if p.apiKey == "" {
		return fmt.Sprintf("Agent response to: %s", userQuery), false, nil
	}

	resp, err := p.callGeminiAPI(userQuery, previousChats)
	if err != nil {
		slog.Error("gemini api call failed", "error", err, "query", userQuery)
		return fmt.Sprintf("Agent response to: %s", userQuery), false, nil
	}

	return resp, false, nil
}

func (p *GeminiProvider) callGeminiAPI(userQuery string, previousChats []apitypes.ChatMessage) (string, error) {
	baseURL := os.Getenv("GEMINI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	url := fmt.Sprintf("%s/models/gemini-flash-latest:generateContent?key=%s", baseURL, p.apiKey)

	req := GeminiRequest{
		SystemInstruction: struct {
			Parts []GeminiPart `json:"parts"`
		}{
			Parts: []GeminiPart{
				{Text: prompts.SystemInstruction},
			},
		},
		Tools: GetGeminiToolsSchema(),
	}

	req.Contents = make([]struct {
		Role  string       `json:"role"`
		Parts []GeminiPart `json:"parts"`
	}, 0)

	userMessagesTotal := 0
	lastUserMsg := ""

	for _, chat := range previousChats {
		role := "user"
		if chat.Role == "assistant" || chat.Role == "model" {
			role = "model"
		} else {
			userMessagesTotal++
			lastUserMsg = strings.ToLower(strings.TrimSpace(chat.Content))
		}

		req.Contents = append(req.Contents, struct {
			Role  string       `json:"role"`
			Parts []GeminiPart `json:"parts"`
		}{
			Role: role,
			Parts: []GeminiPart{
				{Text: chat.Content},
			},
		})
	}

	req.Contents = append(req.Contents, struct {
		Role  string       `json:"role"`
		Parts []GeminiPart `json:"parts"`
	}{
		Role: "user",
		Parts: []GeminiPart{
			{Text: userQuery},
		},
	})

	queryMatchesPrevious := userMessagesTotal > 0 && lastUserMsg == strings.ToLower(strings.TrimSpace(userQuery))

	for i := 0; i < 5; i++ {
		body, _ := json.Marshal(req)
		httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := httpclient.Default.Do(httpReq)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		var gResp GeminiResponse
		if err := json.Unmarshal(respBody, &gResp); err != nil {
			return "", err
		}

		if len(gResp.Candidates) == 0 || len(gResp.Candidates[0].Content.Parts) == 0 {
			return "", fmt.Errorf("no valid candidate response from gemini: %s", string(respBody))
		}

		part := gResp.Candidates[0].Content.Parts[0]

		if part.FunctionCall != nil && part.FunctionCall.Name == "research" {
			if queryMatchesPrevious {
				slog.Warn("Skipping redundant research (duplicate query detected)", "query", userQuery)
			} else {
				loc, _ := part.FunctionCall.Args["location"].(string)
				slog.Info("Gemini requested research", "location", loc)
				state := utils.IsIndianState(loc)

				// Delegate the massive data fetching logic to orchestrator.go
				researchResult, _ := ExecuteResearchFlow(context.Background(), p, userQuery, loc, state)

				// 1) Append Gemini's tool call back to history
				req.Contents = append(req.Contents, struct {
					Role  string       `json:"role"`
					Parts []GeminiPart `json:"parts"`
				}{
					Role: "model",
					Parts: gResp.Candidates[0].Content.Parts,
				})

				// 2) Append the Go backend's tool response
				req.Contents = append(req.Contents, struct {
					Role  string       `json:"role"`
					Parts []GeminiPart `json:"parts"`
				}{
					Role: "user",
					Parts: []GeminiPart{
						{
							FunctionResponse: &GeminiFunctionResponse{
								Name:     "research",
								Response: researchResult,
							},
						},
					},
				})

				continue // Next iteration sends everything to Gemini again
			}
		}

		return part.Text, nil
	}

	return "", fmt.Errorf("exceeded max tool call iterations")
}
