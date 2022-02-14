package main

import (
	"fmt"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type MqttClient struct {
	mqttClient mqtt.Client
}

type pingEvent struct {
	Topic   string
	Payload []byte
}

type Subscription struct {
	Topic string
	QoS   int
}

func NewMqttClient(addr string, clientID string) *MqttClient {
	mqttOpts := mqtt.NewClientOptions().AddBroker(addr)
	mqttOpts.ClientID = clientID
	mqttClient := mqtt.NewClient(mqttOpts)

	return &MqttClient{
		mqttClient: mqttClient,
	}
}

func (m *MqttClient) Connect() error {
	token := m.mqttClient.Connect()

	<-token.Done()
	return token.Error()
}

func (m *MqttClient) Disconnect() {
	m.mqttClient.Disconnect(250)
}

func (m *MqttClient) Subscribe(topic string, qos int, fn func(topic string, payload []byte)) error {
	subToken := m.mqttClient.Subscribe(topic, byte(qos), func(c mqtt.Client, m mqtt.Message) {
		fn(m.Topic(), m.Payload())
	})

	<-subToken.Done()
	if subToken.Error() != nil {
		return subToken.Error()
	}

	allowed := subscriptionAllowed(subToken, topic)
	if !allowed {
		return fmt.Errorf("subscription not allowed")
	}

	return nil
}

func (m *MqttClient) Publish(topic string, qos int) error {
	pubToken := m.mqttClient.Publish(topic, byte(qos), false, "ping")

	<-pubToken.Done()
	return pubToken.Error()
}

func subscriptionAllowed(token mqtt.Token, topic string) bool {
	subscriptionToken, ok := token.(*mqtt.SubscribeToken)
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
