package main

import (
	"context"
	"encoding/base64"
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

	metricsConnectionState = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mqtt_client_connection_state",
		Help: "Connection state of the MQTT client",
	}, []string{"source", "destination"})

	metricsCurrentReconnectAttempts = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mqtt_client_current_reconnect_attempts",
		Help: "Current number of reconnect attempts by the MQTT client",
	}, []string{"source", "destination"})

	metricsTotalReconnectAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mqtt_client_total_reconnect_attempts",
		Help: "Total number of reconnect attempts by the MQTT client",
	}, []string{"source", "destination"})
)

type MqttClient struct {
	subTopic       string
	pubTopic       string
	connected      bool
	reconnectCount int
	reconnectMu    sync.Mutex
	mqttClient     pahomqtt.Client
	ctxCancel      context.CancelFunc
	ctxError       error
	pair           pair
	b64Source      string
	b64Destination string
	subCh          chan struct{}
}

func NewClient(p pair) *MqttClient {
	b64Source := base64.RawURLEncoding.EncodeToString([]byte(p.source))
	b64Destination := base64.RawURLEncoding.EncodeToString([]byte(p.destination))
	clientID := fmt.Sprintf("%s-%s", b64Source, b64Destination)

	client := &MqttClient{
		subTopic:       fmt.Sprintf("mqtt_ping/%s/%s", b64Source, b64Destination),
		pubTopic:       fmt.Sprintf("mqtt_ping/%s/%s", b64Destination, b64Source),
		pair:           p,
		b64Source:      b64Source,
		b64Destination: b64Destination,
		subCh:          make(chan struct{}, 0),
	}

	connOpts := pahomqtt.NewClientOptions().SetClientID(clientID).SetCleanSession(false).SetKeepAlive(0).SetConnectTimeout(1 * time.Second).AddBroker(p.source)
	connOpts.OnConnect = client.onConnectHandler
	connOpts.OnConnectionLost = client.connectionLostHandler
	connOpts.OnReconnecting = client.reconnectHandler

	mqttClient := pahomqtt.NewClient(connOpts)
	client.mqttClient = mqttClient

	metricsTotalReceivedPing.WithLabelValues(client.pair.source, client.pair.destination).Add(0)
	metricsTotalFailedPing.WithLabelValues(client.pair.source, client.pair.destination).Add(0)

	return client
}

func (client *MqttClient) Publish() {
	topic := fmt.Sprintf("mqtt_ping/%s/%s", client.b64Destination, client.b64Source)
	pubToken := client.mqttClient.Publish(topic, byte(0), false, "ping")

	<-pubToken.Done()
	if pubToken.Error() != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Ping from source %s to destination %s failed: %v\n", client.pair.source, client.pair.destination, pubToken.Error())
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

func (client *MqttClient) Connected() bool {
	return client.connected
}

func (client *MqttClient) setConnected() {
	metricsConnectionState.WithLabelValues(client.pair.source, client.pair.destination).Set(1)
	client.connected = true
}

func (client *MqttClient) setDisconnected() {
	metricsConnectionState.WithLabelValues(client.pair.source, client.pair.destination).Set(0)
	client.connected = false
}

func (client *MqttClient) incReconnectAttempt() {
	client.reconnectMu.Lock()
	client.reconnectCount++
	metricsCurrentReconnectAttempts.WithLabelValues(client.pair.source, client.pair.destination).Set(float64(client.reconnectCount))
	metricsTotalReconnectAttempts.WithLabelValues(client.pair.source, client.pair.destination).Inc()
	client.reconnectMu.Unlock()
}

func (client *MqttClient) resetReconnectAttempt() {
	client.reconnectMu.Lock()
	client.reconnectCount = 0
	metricsCurrentReconnectAttempts.WithLabelValues(client.pair.source, client.pair.destination).Set(0)
	client.reconnectMu.Unlock()
}

func (client *MqttClient) Stop(ctx context.Context) error {
	c := make(chan struct{})
	go func() {
		defer close(c)

		unsubToken := client.mqttClient.Unsubscribe(client.subTopic)

		if unsubToken.Error() != nil {
			fmt.Fprintf(os.Stderr, "Unable to gracefully unsubscribe from topic %s: %v\n", client.subTopic, unsubToken.Error())
		} else {
			fmt.Printf("Unsubscribed from topic: %s\n", client.subTopic)
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

	subToken := c.Subscribe(client.subTopic, byte(0), client.messageHandler)

	<-subToken.Done()
	if subToken.Error() != nil {
		fmt.Fprintf(os.Stderr, "Unable to subscribe to topic %s: %v\n", client.subTopic, subToken.Error())
		client.cancel(subToken.Error())
		return
	}

	allowed := subscriptionAllowed(subToken, client.subTopic)
	if !allowed {
		err := fmt.Errorf("subscription not allowed")
		fmt.Fprintf(os.Stderr, "Subscription not allowed to topic %s: %v\n", client.subTopic, err)
		client.cancel(err)
		return
	}

	fmt.Printf("Subscription started to topic: %s\n", client.subTopic)

	client.setConnected()
	if client.reconnectCount > 0 {
		client.resetReconnectAttempt()
	}
}

func (client *MqttClient) connectionLostHandler(c pahomqtt.Client, e error) {
	client.setDisconnected()
	fmt.Fprintf(os.Stderr, "Connection lost to mqtt broker: %v\n", e)
}

func (client *MqttClient) reconnectHandler(c pahomqtt.Client, co *pahomqtt.ClientOptions) {
	// setDisconnected() isn't needed here as OnReconnecting is called as the same time as OnConnectionLost: https://github.com/eclipse/paho.mqtt.golang/blob/master/client.go#L491
	client.incReconnectAttempt()
	fmt.Fprintf(os.Stderr, "Reconnecting to mqtt broker, attempt: %d\n", client.reconnectCount)
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
