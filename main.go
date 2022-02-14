package main

import (
	"context"
	"encoding/base64"
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

type broker struct {
	id     int
	self   string
	others []string
}

func run(mainCtx context.Context, cfg config) error {
	ctx, cancel := context.WithCancel(mainCtx)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	metrics := NewMetricsServer(cfg.MetricsAddress, cfg.MetricsPort)
	g.Go(func() error {
		return metrics.Start(gCtx)
	})

	for i := range cfg.Brokers {
		id := i
		self := cfg.Brokers[i]
		g.Go(func() error {
			return pinger(gCtx, cfg.TickerSeconds, broker{
				id:     id,
				self:   self,
				others: otherBrokers(self, cfg.Brokers),
			})
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

	shutdownCh := make(chan struct{}, 0)
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
	g.Go(func() error {
		return metrics.Stop(shutdownCtx)
	})

	err := g.Wait()
	close(shutdownCh)
	return err
}

func pinger(ctx context.Context, tickerSeconds int, b broker) error {
	g, gCtx := errgroup.WithContext(ctx)

	for i := range b.others {
		other := b.others[i]
		g.Go(func() error {
			return ping(gCtx, tickerSeconds, b, other)
		})
	}

	return g.Wait()
}

func ping(ctx context.Context, tickerSeconds int, b broker, other string) error {
	selfB64 := base64.RawURLEncoding.EncodeToString([]byte(b.self))
	otherB64 := base64.RawURLEncoding.EncodeToString([]byte(other))

	mqttClient := NewMqttClient(b.self, fmt.Sprintf("%s-%s", selfB64, otherB64))

	err := mqttClient.Connect()
	if err != nil {
		return err
	}

	subTopic := fmt.Sprintf("mqtt_ping/%s/%s", selfB64, otherB64)
	pubTopic := fmt.Sprintf("mqtt_ping/%s/%s", otherB64, selfB64)

	subCh := make(chan struct{}, 0)
	subFn := func(topic string, payload []byte) {
		if topic != subTopic {
			fmt.Fprintf(os.Stderr, "expected to receive %q as topic but got: %s\n", subTopic, topic)
			return
		}

		if string(payload) != "ping" {
			fmt.Fprintf(os.Stderr, "expected to receive 'ping' as payload but got: %s\n", string(payload))
			return
		}

		subCh <- struct{}{}
	}

	err = mqttClient.Subscribe(subTopic, 1, subFn)
	if err != nil {
		return err
	}

	tickerDuration := time.Duration(tickerSeconds) * time.Second
	ticker := time.NewTicker(tickerDuration)

	metricsTotalFailedPing.WithLabelValues(b.self, other).Add(0)
	metricsTotalReceivedPing.WithLabelValues(b.self, other).Add(0)

	for {
		select {
		case <-ctx.Done():
			go mqttClient.Disconnect()
			return nil
		case <-ticker.C:
			metricsTotalFailedPing.WithLabelValues(b.self, other).Inc()
			ticker.Reset(tickerDuration)
		case <-subCh:
			metricsTotalReceivedPing.WithLabelValues(b.self, other).Inc()
			ticker.Reset(tickerDuration)
		default:
			err := mqttClient.Publish(pubTopic, 1)
			if err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: Ping from %d (%s) to %s failed: %v\n", b.id, b.self, other, err)
			}

			time.Sleep(tickerDuration / 2)
		}
	}
}

func otherBrokers(self string, all []string) []string {
	others := []string{}
	for _, other := range all {
		if other != self {
			others = append(others, other)
		}
	}
	return others
}

type config struct {
	Brokers        []string
	MetricsAddress string
	MetricsPort    int
	TickerSeconds  int
}

func newConfig() (config, error) {
	cfg := config{
		Brokers: []string{
			"10.244.6.11:1883",
			"10.244.7.4:1883",
			"10.244.4.11:1883",
		},
		MetricsAddress: "0.0.0.0",
		MetricsPort:    8081,
		TickerSeconds:  10,
	}
	return cfg, nil
}
