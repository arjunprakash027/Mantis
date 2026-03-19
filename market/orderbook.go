package market

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

func StartOrderBookStream(ctx context.Context, assetIds []string, msgChan chan<- []byte) error {
	wsURL := "wss://ws-subscriptions-clob.polymarket.com/ws/market"

	go func() {

		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {

			if ctx.Err() != nil {
				log.Printf("WebSocket Stream Stopped by Context")
				return
			} else {
				log.Printf("WebSocket Dial Error: %v", err)
				return
			}
		}

		defer conn.Close()

		var mu sync.Mutex

		subMsg := map[string]interface{}{
			"type":       "market",
			"assets_ids": assetIds,
		}

		mu.Lock()
		if err := conn.WriteJSON(subMsg); err != nil {
			log.Printf("WebSocket Sub Error: %v", err)
			return
		}
		mu.Unlock()

		// Pinging the API with PING to let it know we are still listening
		go func() {
			ticker := time.NewTicker(20 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					mu.Lock()
					if err := conn.WriteMessage(websocket.TextMessage, []byte("PING")); err != nil {
						log.Printf("WebSocket Sub Error: %v", err)
						mu.Unlock()
						return
					}
					mu.Unlock()
				}
			}
		}()

		go func() {
			<-ctx.Done()
			conn.Close()
		}()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Printf("WebSocket Closed Gracefully")
				} else {
					log.Printf("WebSocket CRASHED: %v", err)
				}
				return
			}
			msgChan <- message
		}
	}()

	return nil
}
