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

	"github.com/ingres/ingres-agent-go/internal/httpclient"
	"github.com/ingres/ingres-agent-go/internal/prompts"
	apitypes "github.com/ingres/ingres-agent-go/internal/types"
	"github.com/ingres/ingres-agent-go/internal/utils"
)

type OpenRouterProvider struct {
	apiKey  string
	model   string
	baseURL string
}

func NewOpenRouterProvider() *OpenRouterProvider {
	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = "meta-llama/llama-3.1-8b-instruct:free"
	}
	baseURL := os.Getenv("OPENROUTER_BASE_URL")
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	
	return &OpenRouterProvider{
		apiKey:  os.Getenv("OPENROUTER_API_KEY"),
		model:   model,
		baseURL: baseURL,
	}
}

func (p *OpenRouterProvider) HandleUserQuery(ctx context.Context, userQuery string, previousChats []apitypes.ChatMessage) (string, bool, error) {
	if p.apiKey == "" {
		return "OpenRouter API key not set", false, nil
	}

	messages := []GroqMessage{
		{Role: "system", Content: prompts.SystemInstruction},
	}

	for _, m := range previousChats {
		role := m.Role
		if role == "BOT" || role == "model" || role == "assistant" {
			role = "assistant"
		} else {
			role = "user"
		}
		messages = append(messages, GroqMessage{Role: role, Content: m.Content})
	}
	messages = append(messages, GroqMessage{Role: "user", Content: userQuery})

	return p.callWithTools(ctx, userQuery, messages)
}

func (p *OpenRouterProvider) callWithTools(ctx context.Context, userQuery string, messages []GroqMessage) (string, bool, error) {
	fullURL := p.baseURL + "/chat/completions"

	for i := 0; i < 5; i++ { // Max 5 iterations for tool calls
		reqBody := GroqRequest{
			Model:    p.model,
			Messages: messages,
			Tools:    GetToolsSchema(),
		}

		body, _ := json.Marshal(reqBody)
		httpReq, _ := http.NewRequest("POST", fullURL, bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
		httpReq.Header.Set("HTTP-Referer", "https://ingres-ai.com") // Required by OpenRouter
		httpReq.Header.Set("X-Title", "Ingres AI Agent")

		resp, err := httpclient.Default.Do(httpReq)
		if err != nil {
			return "", false, err
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			slog.Error("OpenRouter API error", "status", resp.StatusCode, "body", string(respBody))
			return "", false, fmt.Errorf("openrouter api error: %s", string(respBody))
		}

		var gResp GroqResponse
		if err := json.Unmarshal(respBody, &gResp); err != nil {
			slog.Error("OpenRouter parse error", "error", err, "body", string(respBody))
			return "", false, fmt.Errorf("failed to parse openrouter response: %s", string(respBody))
		}

		if len(gResp.Choices) == 0 {
			return "", false, fmt.Errorf("no choice returned from openrouter")
		}

		msg := gResp.Choices[0].Message
		messages = append(messages, msg)

		if gResp.Choices[0].FinishReason == "tool_calls" {
			for _, tc := range msg.ToolCalls {
				var resultStr string
				if tc.Function.Name == "research" {
					var args map[string]interface{}
					json.Unmarshal([]byte(tc.Function.Arguments), &args)
					loc, _ := args["location"].(string)
					slog.Info("OpenRouter requested research", "location", loc)
					state := utils.IsIndianState(loc)
					researchResult, err := ExecuteResearchFlow(ctx, p, userQuery, loc, state)
					if err != nil {
						resultStr = fmt.Sprintf("Error: %v", err)
					} else {
						resultBytes, _ := json.Marshal(researchResult)
						resultStr = string(resultBytes)
					}
				} else {
					result, err := ExecuteTool(tc.Function.Name, tc.Function.Arguments)
					if err != nil {
						resultStr = fmt.Sprintf("Error: %v", err)
					} else {
						resultStr = result
					}
				}

				messages = append(messages, GroqMessage{
					Role:    "tool",
					ToolID:  tc.ID,
					Content: resultStr,
				})
			}
			continue // Go for another round
		}

		return msg.Content, false, nil
	}

	return "", false, fmt.Errorf("too many tool calls")
}

func (p *OpenRouterProvider) GetBusinessDataInterpretation(ctx context.Context, userQuery string) (*apitypes.GetBusinessDataResult, error) {
	return OpenRouterGetBusinessDataInterpretation(ctx, p, userQuery)
}

func (p *OpenRouterProvider) GetMapBusinessDataInterpretation(ctx context.Context, userQuery string) (*apitypes.GetMapBusinessDataInterpretation, error) {
	return OpenRouterGetMapBusinessDataInterpretation(ctx, p, userQuery)
}
