package main_test

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ingres/ingres-agent-go/internal/cache"
	"github.com/ingres/ingres-agent-go/internal/handler"
	"github.com/ingres/ingres-agent-go/internal/types"
)

func TestAgentReplayQueries(t *testing.T) {
	// 1. Setup Environment Variables
	os.Setenv("LLM_PROVIDER", "groq")
	os.Setenv("GROQ_BASE_URL", "http://localhost:8081")
	os.Setenv("INGRES_API_BASE", "http://localhost:8082")
	// The key must be set, though playback proxy ignores it
	os.Setenv("GROQ_API_KEY", "dummy_key_for_playback")

	// 2. Start Playback Proxies
	// Use relative paths assuming tests run from servers/ingres-agent/
	proxyScript := "../../recording_proxy.py"
	fixturesDir := "../../tests/fixtures"

	llmProxy := exec.Command("python", proxyScript, "--port", "8081", "--target", "https://api.groq.com/openai/v1", "--mode", "playback", "--dir", fixturesDir)
	if err := llmProxy.Start(); err != nil {
		t.Fatalf("Failed to start LLM proxy: %v", err)
	}
	defer llmProxy.Process.Kill()

	govProxy := exec.Command("python", proxyScript, "--port", "8082", "--target", "https://ingres.iith.ac.in", "--mode", "playback", "--dir", filepath.Join(fixturesDir, "shared", "ingres"))
	if err := govProxy.Start(); err != nil {
		t.Fatalf("Failed to start Gov API proxy: %v", err)
	}
	defer govProxy.Process.Kill()

	// Give proxies a moment to bind to ports
	time.Sleep(2 * time.Second)

	// 3. Initialize Fiber App
	app := fiber.New()
	cacheStore := cache.NewLocalStore(1 * time.Minute)
	app.Post("/agent/chat", handler.HandleAgentChat(cacheStore))

	// 4. Load Golden Queries
	csvFile, err := os.Open("../../tests/golden_queries.csv")
	if err != nil {
		t.Fatalf("Failed to open golden queries: %v", err)
	}
	defer csvFile.Close()

	reader := csv.NewReader(csvFile)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to parse golden queries: %v", err)
	}

	// Skip header
	for i, record := range records {
		if i == 0 {
			continue
		}

		queryID := record[0]
		queryText := record[1]
		// targetDistrict := record[2] (ignoring)
		category := record[3]
		
		t.Run(queryID, func(t *testing.T) {
			// Clear cache for each test to ensure fresh processing
			cacheStore = cache.NewLocalStore(1 * time.Minute)
			app = fiber.New()
			app.Post("/agent/chat", handler.HandleAgentChat(cacheStore))

			reqPayload := types.AgentRequest{
				Question: queryText,
				// Messages is intentionally empty to mimic fresh queries
				Messages: []types.AgentMessage{}, 
			}

			bodyBytes, _ := json.Marshal(reqPayload)
			req := httptest.NewRequest("POST", "/agent/chat", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			// -1 timeout prevents test failures if playback takes a second
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("HTTP request failed: %v", err)
			}

			if resp.StatusCode != 200 {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			respBody, _ := io.ReadAll(resp.Body)
			var agentResp types.AgentResponse
			if err := json.Unmarshal(respBody, &agentResp); err != nil {
				t.Fatalf("Failed to parse agent response: %v", err)
			}

			if agentResp.Answer == "" {
				t.Errorf("Agent answer was empty for category: %s", category)
			}

			// We will add specific category assertions in Step 2
			t.Logf("Query [%s] executed successfully", queryID)
		})
	}
}
