package main

import (
	"encoding/base64"
	"fmt"
)

type brokerPair struct {
	source            string
	destination       string
	base64Source      string
	base64Destination string
	clientID          string
	subscriptionTopic string
	publishTopic      string
}

func generateBrokerPairs(list []string) ([]brokerPair, error) {
	if len(list) < 2 {
		return nil, fmt.Errorf("received %d item(s) in list but at least 2 are required", len(list))
	}

	var pairs []brokerPair
	others := func(self string, all []string) []string {
		others := []string{}
		for _, other := range all {
			if other != self {
				others = append(others, other)
			}
		}
		return others
	}

	for _, source := range list {
		for _, destination := range others(source, list) {
			base64Source := base64.RawURLEncoding.EncodeToString([]byte(source))
			base64Destination := base64.RawURLEncoding.EncodeToString([]byte(destination))
			clientID := fmt.Sprintf("%s-%s", base64Source, base64Destination)
			subscriptionTopic := fmt.Sprintf("mqtt_ping/%s/%s", base64Source, base64Destination)
			publishTopic := fmt.Sprintf("mqtt_ping/%s/%s", base64Destination, base64Source)

			pair := brokerPair{
				source:            source,
				destination:       destination,
				base64Source:      base64Source,
				base64Destination: base64Destination,
				clientID:          clientID,
				subscriptionTopic: subscriptionTopic,
				publishTopic:      publishTopic,
			}
			pairs = append(pairs, pair)
		}
	}

	return pairs, nil
}
