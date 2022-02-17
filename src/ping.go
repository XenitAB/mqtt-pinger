package main

import (
	"context"
	"time"
)

type Pinger interface {
	SubCh() <-chan struct{}
	IncrementFailedPing()
	IncrementReceivedPing()
	Publish()
	Ready(ctx context.Context)
}

func startPing(ctx context.Context, tickerSeconds int, p brokerPair, pinger Pinger) {
	tickerDuration := time.Duration(tickerSeconds) * time.Second
	ticker := time.NewTicker(tickerDuration)

	pinger.Ready(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pinger.IncrementFailedPing()
		case <-pinger.SubCh():
			pinger.IncrementReceivedPing()
			ticker.Reset(tickerDuration)
		default:
			pinger.Publish()
			time.Sleep(tickerDuration / 2)
		}
	}
}
