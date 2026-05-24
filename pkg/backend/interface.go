package backend

import (
	"context"
	"github.com/arjunprakash027/Mantis/market"
)

type StreamerBackend interface {
	PublishStream(ctx context.Context, namespace string, identifier string, data []byte) error
	RegisterMetadata(ctx context.Context, slug string, tokens []market.Token) error
}

type ExecutorBackend interface {
	ExecuteTrade(ctx context.Context, action string, asset string, amount float64, price float64, strategyID string) (bool, string, error)
	PublishSignalResult(ctx context.Context, stratergyID string, resultPayload []byte) error
	SubscribeInboundSignals(ctx context.Context, handler func(msgID string, payload []byte)) error
	AcknowledgeSignal(ctx context.Context, msgID string) error
}

