package analyze

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultModel = "claude-sonnet-4-5"
const anthropicAPI = "https://api.anthropic.com/v1/messages"
const anthropicVersion = "2023-06-01"

// Client calls Anthropic Messages API with the web_search server tool.
type Client struct {
	APIKey string
	Model  string
	HTTP   *http.Client
}

func NewClient(apiKey, model string) *Client {
	if model == "" {
		model = defaultModel
	}
	return &Client{
		APIKey: apiKey,
		Model:  model,
		HTTP:   &http.Client{Timeout: 120 * time.Second},
	}
}

type localContext struct {
	Watched []map[string]any `json:"watched_items,omitempty"`
}

func (c *Client) Analyze(ctx context.Context, query string, watched []map[string]any) (*Result, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY saknas")
	}

	system := `Du är prisanalytiker för den svenska marknaden (Prisläge).
Använd web_search aktivt. Svara ENBART med giltig JSON (ingen markdown, inga kodstaket) enligt schemat:

{
  "product": {"name": string, "brand": string, "variant": string, "gtin"?: string},
  "current_prices": [{"store": string, "price": number, "url": string}],
  "historical_low"?: {"price": number, "date": string, "source": string},
  "predecessor"?: {"name": string, "pattern": string},
  "best_month": string,
  "best_period_note": string,
  "confidence": "low"|"medium"|"high",
  "reasoning": string,
  "sources": [{"title": string, "url": string}]
}

Instruktioner:
1) Identifiera exakt produkt/variant (färg, kapacitet, modellår).
2) Hitta aktuella priser i svenska butiker (Elgiganten, Inet, NetOnNet, Webhallen, Komplett, Kjell, Apple, m.fl.).
3) Hitta publicerad prishistorik (t.ex. lägsta senaste månaderna på Prisjakt/Pricerunner eller liknande).
4) Identifiera föregångarmodellen och hur dess pris utvecklades månad för månad efter lansering.
5) Väga in säsongsmönster för kategorin i Sverige (t.ex. Black Friday, januarireor, back-to-school).
6) Ge en konkret rekommendation: best_month (svenskt månadsnamn) + best_period_note + kort reasoning på svenska.
URL:er måste vara http(s). Priser i SEK som tal. Om data saknas: confidence=low och förklara i reasoning.
Om watched_items ges, använd dem som egen historik när de matchar produkten.`

	userPayload := map[string]any{
		"query": query,
	}
	if len(watched) > 0 {
		userPayload["watched_items"] = watched
	}
	userBytes, _ := json.Marshal(userPayload)

	reqBody := map[string]any{
		"model":      c.Model,
		"max_tokens": 4096,
		"system":     system,
		"messages": []map[string]any{
			{"role": "user", "content": string(userBytes)},
		},
		// Docs: https://platform.claude.com/docs/en/agents-and-tools/tool-use/web-search-tool
		// type web_search_20250305, name web_search (server tool).
		"tools": []map[string]any{
			{
				"type":     "web_search_20250305",
				"name":     "web_search",
				"max_uses": 8,
				"user_location": map[string]any{
					"type":     "approximate",
					"city":     "Stockholm",
					"region":   "Stockholm",
					"country":  "SE",
					"timezone": "Europe/Stockholm",
				},
			},
		},
	}

	raw, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicAPI, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.APIKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	res, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic request: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("anthropic %d: %s", res.StatusCode, truncate(string(body), 500))
	}

	text, err := extractAssistantText(body)
	if err != nil {
		return nil, err
	}
	parsed, err := parseResultJSON(text)
	if err != nil {
		return nil, err
	}
	if err := Validate(parsed); err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	return parsed, nil
}

func extractAssistantText(body []byte) (string, error) {
	var env struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Error      *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return "", fmt.Errorf("decode anthropic response: %w", err)
	}
	if env.Error != nil {
		return "", fmt.Errorf("anthropic error: %s", env.Error.Message)
	}
	var parts []string
	for _, c := range env.Content {
		if c.Type == "text" && strings.TrimSpace(c.Text) != "" {
			parts = append(parts, c.Text)
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("ingen text i Claude-svar (stop_reason=%s)", env.StopReason)
	}
	return strings.Join(parts, "\n"), nil
}

func parseResultJSON(text string) (*Result, error) {
	s := strings.TrimSpace(text)
	if i := strings.Index(s, "```"); i >= 0 {
		s = s[i+3:]
		s = strings.TrimPrefix(s, "json")
		if j := strings.Index(s, "```"); j >= 0 {
			s = s[:j]
		}
		s = strings.TrimSpace(s)
	}
	if i := strings.Index(s, "{"); i >= 0 {
		s = s[i:]
	}
	if j := strings.LastIndex(s, "}"); j >= 0 {
		s = s[:j+1]
	}
	var out Result
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("parse JSON: %w (%s)", err, truncate(s, 200))
	}
	return &out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
