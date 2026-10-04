package data

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"github.com/ingres/ingres-agent-go/internal/httpclient"
)

type FetchMapBusinessDataInput struct {
	Location string
	Year     *string
}

func FetchMapBusinessData(input FetchMapBusinessDataInput) (interface{}, error) {
	currentYear := time.Now().Year()
	startYear := currentYear - 1
	if input.Year != nil && *input.Year != "" {
		if y, err := strconv.Atoi(*input.Year); err == nil {
			startYear = y
		}
	}
	yearString := fmt.Sprintf("%d-%d", startYear, startYear+1)

	stateUUIDs := []string{
		"f38e6de8-396e-47b4-af18-32c333eddccc", "bd81d570-0e97-4dad-9bcd-219a05f56f2f",
		"d6da1adf-2a9c-4908-a356-e7668d4ab108", "a1588b13-5700-450d-b51d-b782ed565801",
		"609c5df4-6414-4bbd-a22d-ff5fbdad6836", "ee1789e0-3831-4cdb-b0b8-77231309cd7e",
		"648a95f6-9249-4c92-8ae4-a9d93eb7c898", "12f0823e-2c16-4765-8137-585b3d5123ac",
		"6ad10263-20ad-4001-86c0-738be00ec03e", "e7b3f02d-2497-4bcd-9e20-baa4b621822b",
		"b967d23e-68c8-492c-a372-9623441f7d24", "94360caf-ebf0-4303-8c9e-3509bb0cded2",
		"68dad067-a4cc-4397-a6fd-fc10ef7cc933", "f318279f-537d-4373-b9dc-ac347cdef82f",
		"ae12d8f1-e36c-445f-8d32-80f958766b4e", "f5c761d1-d9c5-4e5f-af48-2cdfc1c92ce8",
		"a1ac5d18-8c9a-4047-8fdd-4d7d9deaa34e", "7f615d2f-0be6-42bf-891f-7239e101e487",
		"8fd29251-6e20-4f33-9a96-f47cab45eb13", "b8c2ccc0-b638-468a-84e7-3512ece9b3a5",
		"496bae22-c752-43d6-9bda-7798f9d3b32f", "eaec6bbb-a219-415f-bdba-991c42586352",
		"ca25704a-43d0-42ad-bcf4-b2b60270594c", "5f2a2b2b-cd02-4a00-94ec-3fc97d9f0d19",
		"0ee3cc5b-1b67-465c-9528-46540aea9cb7", "812072c8-d651-485c-88e8-7dd29d8c183b",
		"37572702-1c31-4723-8328-0a1757f6b4e1", "d19a5290-2e40-494a-83d2-98f4c845b1f1",
		"fd163bec-156c-4633-a7b7-5dfee5fcdf57", "5ff5533e-36c5-4c24-abff-1cbbea6f2bdf",
		"785cc6f0-e9d0-4961-9578-08ed2f24377a", "19903c2c-ed18-4782-a679-dc10e8aa71ed",
		"e98cd5b7-6556-4c0f-a778-3429e1c14a6b", "40320284-8d82-4f7e-80d7-00a00bb0e5b0",
		"edce8ca7-bf15-4b5e-b4c5-b10c543acd83", "200030a1-6d27-4dff-988a-e2104ff62ab8",
		"68ecabb4-0ea5-4909-b8e3-20bbaa7b91e8",
	}

	payload := map[string]interface{}{
		"verificationStatus": 1,
		"approvalLevel":      1,
		"computationType":    "normal",
		"locuuid":            "ffce954d-24e1-494b-ba7e-0931d8ad6085",
		"parentuuid":         "ffce954d-24e1-494b-ba7e-0931d8ad6085",
		"period":             "annual",
		"stateuuid":          "ffce954d-24e1-494b-ba7e-0931d8ad6085",
		"view":               "admin",
		"year":               yearString,
		"areaType":           "total",
		"layerName":          "gec:indgec_mandal_all",
		"component":          "category",
		"mapUsage":           false,
		"stateUUID":          stateUUIDs,
	}

	slog.Info("Fetching MAP data", "location", input.Location, "year", yearString)

	baseURL := os.Getenv("INGRES_API_BASE")
	if baseURL == "" {
		baseURL = "https://ingres.iith.ac.in"
	}
	apiURL := baseURL + "/api/gec/mapBusinessData"

	payloadBytes, _ := json.Marshal(payload)
	resp, err := httpclient.Default.Post(apiURL, "application/json", bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("error fetching map business data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	var dataList []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&dataList); err != nil {
		return nil, fmt.Errorf("failed to decode map response: %v", err)
	}

	searchLoc := strings.ToLower(strings.TrimSpace(input.Location))
	for _, item := range dataList {
		locName, ok := item["locationName"].(string)
		if ok && strings.ToLower(locName) == searchLoc {
			return item, nil
		}
	}

	return map[string]interface{}{
		"message": fmt.Sprintf("No map data found for %s", input.Location),
		"data":    nil,
	}, nil
}
