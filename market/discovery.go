// Go code to fetch all current markets (discovery)
package market

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Simplified Market struct using only what we need
type DiscoveryMarket struct {
	ID        string  `json:"id"`
	Question  string  `json:"question"`
	Slug      string  `json:"slug"`
	Liquidity float64 `json:"liquidity,string"`
	Volume    float64 `json:"volume,string"`

	// Timing (For finding "New" markets)
	StartDate string `json:"startDateIso"`
	EndDate   string `json:"endDateIso"`

	// Prices & Analytics (Numbers in API)
	LastPrice float64 `json:"lastTradePrice"`
	Change24h float64 `json:"oneDayPriceChange"`
	Change1h  float64 `json:"oneHourPriceChange"`
	Spread    float64 `json:"spread"`

	// Metadata & Routing
	Category     string `json:"category"`
	ClobTokenIds string `json:"clobTokenIds"`
}

// StartDiscoveryStream periodically fetches ALL active markets and pushes them to Redis
func StartDiscoveryStream(ch chan<- []byte) error {
	base := "https://gamma-api.polymarket.com/markets?active=true&closed=false&limit=100&order=startDate&ascending=false"
	fmt.Printf("Discovery Stream Started..")

	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		for {
			var allMarkets []DiscoveryMarket
			offset := 0

			for {
				url := fmt.Sprintf("%s&offset=%d", base, offset)
				resp, err := http.Get(url)
				if err != nil {
					fmt.Printf("Discovery Network Error: %v\n", err)
					break
				}

				if resp.StatusCode != http.StatusOK {
					fmt.Printf("Discovery API Error: status %d for offset %d\n", resp.StatusCode, offset)
					resp.Body.Close()
					break
				}

				var batch []DiscoveryMarket
				decoder := json.NewDecoder(resp.Body)
				if err := decoder.Decode(&batch); err != nil {
					fmt.Printf("Discovery Decode Error at offset %d: %v\n", offset, err)
					resp.Body.Close()
					break
				}
				if len(batch) == 0 {
					resp.Body.Close()
					break
				}
				allMarkets = append(allMarkets, batch...)
				resp.Body.Close()
				offset += 100
			}

			if len(allMarkets) > 0 {
				fmt.Printf("🔍 Discovery: Found %d active markets. Pushing to Redis...\n", len(allMarkets))
				if data, err := json.Marshal(allMarkets); err == nil {
					ch <- data
				}
			}
			<-ticker.C
		}
	}()

	return nil
}
