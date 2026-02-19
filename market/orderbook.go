package market

import (
	"context"
	"log"

	"github.com/gorilla/websocket"
)

func StartOrderBookStream(ctx context.Context, assetIds []string, msgChan chan<- []byte) error {
	wsURL := "wss://ws-subscriptions-clob.polymarket.com/ws/market"

	go func() {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			log.Printf("❌ WebSocket Dial Error: %v", err)
			return
		}
		defer conn.Close()

		subMsg := map[string]interface{}{
			"type":       "market",
			"assets_ids": assetIds,
		}
		if err := conn.WriteJSON(subMsg); err != nil {
			log.Printf("❌ WebSocket Sub Error: %v", err)
			return
		}

		go func() {
			<-ctx.Done()
			conn.Close()
		}()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Printf("💡 WebSocket Closed Gracefully")
				} else {
					log.Printf("‼️ WebSocket CRASHED: %v", err)
				}
				return
			}
			msgChan <- message
		}
	}()

	return nil
}
