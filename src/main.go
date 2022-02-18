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
	cfg, err := loadConfig(os.Args[1:])
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
	pairs, err := generateBrokerPairs(cfg.Brokers, cfg.ClientIDPrefix)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(mainCtx)
	defer cancel()

	metrics := NewMetricsServer(cfg.MetricsAddress, cfg.MetricsPort)
	go func() {
		err := metrics.Start()
		if err != nil {
			fmt.Fprintf(os.Stderr, "unable to start metrics server: %v\n", err)
			cancel()
		}
	}()

	g, gCtx := errgroup.WithContext(ctx)
	for i := range pairs {
		pinger := NewPingClient(&pairs[i], time.Duration(cfg.PingInterval)*time.Second)
		g.Go(func() error {
			return pinger.Run(gCtx)
		})
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
	defer close(shutdownCh)
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

	err = g.Wait()
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to gracefully shutdown mqtt client: %v\n", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	shutdownErrG, shutdownErrGCtx := errgroup.WithContext(shutdownCtx)
	shutdownErrG.Go(func() error {
		return metrics.Stop(shutdownErrGCtx)
	})

	err = shutdownErrG.Wait()
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to gracefully shutdown metrics server: %v\n", err)
	}

	return nil
}
