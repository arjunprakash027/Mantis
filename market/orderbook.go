package market

import (
	"fmt"
	"log"

	"github.com/gorilla/websocket"
)

func StartOrderBookStream(assetIds []string, msgChan chan<- []byte) error {
	wsURL := "wss://ws-subscriptions-clob.polymarket.com/ws/market"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to websocket: %w", err)
	}

	subMsg := map[string]interface{}{
		"type":       "market",
		"assets_ids": assetIds,
	}

	if err := conn.WriteJSON(subMsg); err != nil {
		conn.Close()
		return fmt.Errorf("failed to subscribe to market: %w", err)
	}

	go func() {
		defer conn.Close()
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("read error: %v", err)
				close(msgChan)
				return
			}
			msgChan <- message
		}
	}()

	return nil
}
