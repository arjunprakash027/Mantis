package market

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Token struct {
	TokenID string `json:"token_id"`
	Outcome string `json:"outcome"`
	Market  string `json:"market"`
}

type MarketInfo struct {
	Tokens   []Token `json:"tokens"`
	Question string  `json:"question"`
}

func GetTokens(slug string) ([]Token, string, error) {
	MarketUrl := fmt.Sprintf("https://gamma-api.polymarket.com/markets/slug/%s", slug)
	resp, err := http.Get(MarketUrl)
	if err == nil && resp.StatusCode == http.StatusOK {

		var m struct {
			Question     string `json:"question"`
			ClobTokenIds string `json:"clobTokenIds"`
			Outcomes     string `json:"outcomes"`
		}

		json.NewDecoder(resp.Body).Decode(&m)
		resp.Body.Close()

		if m.ClobTokenIds != "" && m.ClobTokenIds != "[]" {
			return parseTokens(m.ClobTokenIds, m.Outcomes, m.Question), m.Question, nil
		}
	}

	eventURL := fmt.Sprintf("https://gamma-api.polymarket.com/events/slug/%s", slug)
	resp, err = http.Get(eventURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("event not found")
	}

	defer resp.Body.Close()

	var event struct {
		Title   string `json:"title"`
		Markets []struct {
			Question     string `json:"question"`
			ClobTokenIds string `json:"clobTokenIds"`
			Outcomes     string `json:"outcomes"`
		} `json:"markets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&event); err != nil {
		return nil, "", err
	}

	var allTokens []Token
	for _, m := range event.Markets {
		tokens := parseTokens(m.ClobTokenIds, m.Outcomes, m.Question)
		allTokens = append(allTokens, tokens...)
	}

	return allTokens, event.Title, nil
}

func parseTokens(idsStr, nameStr string, marketName string) []Token {
	var ids, names []string
	json.Unmarshal([]byte(idsStr), &ids)
	json.Unmarshal([]byte(nameStr), &names)

	tokens := make([]Token, 0, len(ids))
	for i, id := range ids {
		name := "Unknown"
		if i < len(names) {
			name = names[i]
		}
		tokens = append(tokens, Token{
			TokenID: id,
			Outcome: name,
			Market:  marketName,
		})
	}
	return tokens
}
