package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

func main() {
	cfg, err := newConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config generation returned an error: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	err = run(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "application returned an error: %v\n", err)
		os.Exit(1)
	}
}

func run(mainCtx context.Context, cfg config) error {
	pairs, err := generateBrokerPairs(cfg.Brokers)
	if err != nil {
		return err
	}

	pingers := make(map[*brokerPair]*MqttClient)
	for i := range pairs {
		p := &pairs[i]
		pingers[p] = NewClient(*p)
	}

	ctx, cancel := context.WithCancel(mainCtx)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	metrics := NewMetricsServer(cfg.MetricsAddress, cfg.MetricsPort)
	g.Go(func() error {
		return metrics.Start(gCtx)
	})

	for i := range pairs {
		p := &pairs[i]
		g.Go(func() error {
			return pingers[p].Start(gCtx)
		})
	}

	for i := range pairs {
		p := &pairs[i]
		go startPing(gCtx, cfg.TickerSeconds, *p, pingers[p])
	}

	stopChan := make(chan os.Signal, 2)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGPIPE)

	var doneMsg string
	select {
	case sig := <-stopChan:
		doneMsg = fmt.Sprintf("os.Signal (%s)", sig)
	case <-gCtx.Done():
		doneMsg = "context"
	}

	shutdownCh := make(chan struct{})
	go func() {
		select {
		case <-stopChan:
			fmt.Fprint(os.Stderr, "forcefully stopped the application\n")
			os.Exit(1)
		case <-shutdownCh:
			return
		}
	}()

	cancel()

	fmt.Printf("server shutdown initiated by: %s\n", doneMsg)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	for i := range pairs {
		p := &pairs[i]
		g.Go(func() error {
			return pingers[p].Stop(shutdownCtx)
		})
	}

	g.Go(func() error {
		return metrics.Stop(shutdownCtx)
	})

	err = g.Wait()
	close(shutdownCh)
	return err
}
