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
}

func startPing(ctx context.Context, tickerSeconds int, p pair, pinger Pinger) {
	tickerDuration := time.Duration(tickerSeconds) * time.Second
	ticker := time.NewTicker(tickerDuration)

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
