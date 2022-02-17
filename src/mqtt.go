package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	metricsTotalReceivedPing = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mqtt_total_received_ping",
		Help: "Total number of successful ping",
	}, []string{"source", "destination"})

	metricsTotalFailedPing = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mqtt_total_failed_ping",
		Help: "Total number of failed ping",
	}, []string{"source", "destination"})
)

type PingClient struct {
	mqttClient   pahomqtt.Client
	pair         brokerPair
	pingInterval time.Duration
	subCh        chan struct{}
	interruptCh  chan struct{}
	interruptErr error
	interruptMu  sync.Mutex
	readyCh      chan struct{}
}

func NewPingClient(p brokerPair, pingInterval time.Duration) *PingClient {
	client := &PingClient{
		pair:         p,
		pingInterval: pingInterval,
		subCh:        make(chan struct{}),
		interruptCh:  make(chan struct{}),
		readyCh:      make(chan struct{}),
	}

	connOpts := pahomqtt.NewClientOptions().SetClientID(p.clientID).SetCleanSession(false).SetKeepAlive(0).SetConnectTimeout(1 * time.Second).AddBroker(p.source)
	connOpts.OnConnect = client.onConnectHandler

	mqttClient := pahomqtt.NewClient(connOpts)
	client.mqttClient = mqttClient

	metricsTotalReceivedPing.WithLabelValues(client.pair.source, client.pair.destination).Add(0)
	metricsTotalFailedPing.WithLabelValues(client.pair.source, client.pair.destination).Add(0)

	return client
}

func (client *PingClient) Run(ctx context.Context) error {
	token := client.mqttClient.Connect()
	defer client.disconnect(5 * time.Second)

	<-token.Done()
	if token.Error() != nil {
		return token.Error()
	}

	client.ready(ctx)

	client.ping(ctx, client.pingInterval)

	select {
	case <-client.interruptCh:
		return client.interruptErr
	case <-ctx.Done():
		return nil
	}
}

func (client *PingClient) publish() {
	pubToken := client.mqttClient.Publish(client.pair.publishTopic, byte(0), false, "ping")

	<-pubToken.Done()
	if pubToken.Error() != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Ping from source %s to destination %s failed: %v\n", client.pair.source, client.pair.destination, pubToken.Error())
	}
}

func (client *PingClient) incrementReceivedPing() {
	metricsTotalReceivedPing.WithLabelValues(client.pair.source, client.pair.destination).Inc()
}

func (client *PingClient) incrementFailedPing() {
	metricsTotalFailedPing.WithLabelValues(client.pair.source, client.pair.destination).Inc()
}

func (client *PingClient) ping(ctx context.Context, pingInterval time.Duration) {
	tickerInterval := pingInterval * 2
	ticker := time.NewTicker(tickerInterval)

	for {
		select {
		case <-client.interruptCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			client.incrementFailedPing()
		case <-client.subCh:
			client.incrementReceivedPing()
			ticker.Reset(tickerInterval)
		default:
			client.publish()
			time.Sleep(pingInterval)
		}
	}
}

func (client *PingClient) messageHandler(c pahomqtt.Client, m pahomqtt.Message) {
	if string(m.Payload()) != "ping" {
		fmt.Fprintf(os.Stderr, "expected to receive 'ping' as payload but got: %s\n", string(m.Payload()))
		return
	}

	client.subCh <- struct{}{}
}

func (client *PingClient) ready(ctx context.Context) {
	select {
	case <-client.interruptCh:
	case <-ctx.Done():
	case <-client.readyCh:
	}
}

func (client *PingClient) disconnect(timeout time.Duration) {
	disconnectTimeout := time.NewTimer(timeout)
	disconnectCh := make(chan struct{})

	go func() {
		_ = client.mqttClient.Unsubscribe(client.pair.subscriptionTopic)
		client.mqttClient.Disconnect(250)
		close(disconnectCh)
	}()

	select {
	case <-disconnectCh:
	case <-disconnectTimeout.C:
	}
}

func (client *PingClient) interrupt(err error) {
	client.interruptMu.Lock()

	select {
	case <-client.interruptCh:
	default:
		close(client.interruptCh)
	}

	client.interruptErr = err

	client.interruptMu.Unlock()
}

func (client *PingClient) onConnectHandler(c pahomqtt.Client) {
	subToken := c.Subscribe(client.pair.subscriptionTopic, byte(0), client.messageHandler)

	<-subToken.Done()
	if subToken.Error() != nil {
		client.interrupt(subToken.Error())
		return
	}

	allowed := subscriptionAllowed(subToken, client.pair.subscriptionTopic)
	if !allowed {
		client.interrupt(fmt.Errorf("subscription not allowed"))
		return
	}

	select {
	case <-client.readyCh:
	default:
		close(client.readyCh)
	}
}

func subscriptionAllowed(token pahomqtt.Token, topic string) bool {
	subscriptionToken, ok := token.(*pahomqtt.SubscribeToken)
	if !ok {
		return false
	}

	result := subscriptionToken.Result()
	res, found := result[topic]
	if !found {
		return false
	}

	if res >= 128 {
		return false
	}

	return true
}
