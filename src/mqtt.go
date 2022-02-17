package main

import (
	"context"
	"fmt"
	"os"
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

type MqttClient struct {
	mqttClient pahomqtt.Client
	ctxCancel  context.CancelFunc
	ctxError   error
	pair       brokerPair
	subCh      chan struct{}
	readyCh    chan struct{}
}

func NewClient(p brokerPair) *MqttClient {
	client := &MqttClient{
		pair:    p,
		subCh:   make(chan struct{}),
		readyCh: make(chan struct{}),
	}

	connOpts := pahomqtt.NewClientOptions().SetClientID(p.clientID).SetCleanSession(false).SetKeepAlive(0).SetConnectTimeout(1 * time.Second).AddBroker(p.source)
	connOpts.OnConnect = client.onConnectHandler

	mqttClient := pahomqtt.NewClient(connOpts)
	client.mqttClient = mqttClient

	metricsTotalReceivedPing.WithLabelValues(client.pair.source, client.pair.destination).Add(0)
	metricsTotalFailedPing.WithLabelValues(client.pair.source, client.pair.destination).Add(0)

	return client
}

func (client *MqttClient) Publish() {
	pubToken := client.mqttClient.Publish(client.pair.publishTopic, byte(0), false, "ping")

	<-pubToken.Done()
	if pubToken.Error() != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Ping from source %s to destination %s failed: %v\n", client.pair.source, client.pair.destination, pubToken.Error())
	}
}

func (client *MqttClient) Ready(ctx context.Context) {
	select {
	case <-ctx.Done():
	case <-client.readyCh:
	}
}

func (client *MqttClient) IncrementReceivedPing() {
	metricsTotalReceivedPing.WithLabelValues(client.pair.source, client.pair.destination).Inc()
}

func (client *MqttClient) IncrementFailedPing() {
	metricsTotalFailedPing.WithLabelValues(client.pair.source, client.pair.destination).Inc()
}

func (client *MqttClient) SubCh() <-chan struct{} {
	return client.subCh
}

func (client *MqttClient) Stop(ctx context.Context) error {
	c := make(chan struct{})
	go func() {
		defer close(c)

		unsubToken := client.mqttClient.Unsubscribe(client.pair.subscriptionTopic)

		if unsubToken.Error() != nil {
			fmt.Fprintf(os.Stderr, "Unable to gracefully unsubscribe from topic %s: %v\n", client.pair.subscriptionTopic, unsubToken.Error())
		} else {
			fmt.Printf("Unsubscribed from topic: %s\n", client.pair.subscriptionTopic)
		}

		client.mqttClient.Disconnect(250)
		fmt.Println("Disconnected from mqtt broker, stopping client")
	}()

	var err error
	select {
	case <-c:
		err = nil
	case <-ctx.Done():
		err = ctx.Err()
	}

	return err
}

func (client *MqttClient) setContext(ctx context.Context) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	client.ctxCancel = cancel
	return ctx
}

func (client *MqttClient) cancel(err error) {
	client.ctxError = err
	client.ctxCancel()
}

func (client *MqttClient) Start(ctx context.Context) error {
	ctx = client.setContext(ctx)
	token := client.mqttClient.Connect()

	<-token.Done()
	if token.Error() != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to mqtt broker: %v\n", token.Error())
		return token.Error()
	}

	<-ctx.Done()

	return client.ctxError
}

func (client *MqttClient) messageHandler(c pahomqtt.Client, m pahomqtt.Message) {
	if string(m.Payload()) != "ping" {
		fmt.Fprintf(os.Stderr, "expected to receive 'ping' as payload but got: %s\n", string(m.Payload()))
		return
	}

	client.subCh <- struct{}{}
}

func (client *MqttClient) onConnectHandler(c pahomqtt.Client) {
	fmt.Println("Connected to mqtt broker")

	subToken := c.Subscribe(client.pair.subscriptionTopic, byte(0), client.messageHandler)

	<-subToken.Done()
	if subToken.Error() != nil {
		fmt.Fprintf(os.Stderr, "Unable to subscribe to topic %s: %v\n", client.pair.subscriptionTopic, subToken.Error())
		client.cancel(subToken.Error())
		return
	}

	allowed := subscriptionAllowed(subToken, client.pair.subscriptionTopic)
	if !allowed {
		err := fmt.Errorf("subscription not allowed")
		fmt.Fprintf(os.Stderr, "Subscription not allowed to topic %s: %v\n", client.pair.subscriptionTopic, err)
		client.cancel(err)
		return
	}

	select {
	case <-client.readyCh:
	default:
		close(client.readyCh)
	}

	fmt.Printf("Subscription started to topic: %s\n", client.pair.subscriptionTopic)
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
