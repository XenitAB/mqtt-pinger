package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
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

func generateBrokerPairs(list []string, clientIdPrefix string) ([]brokerPair, error) {
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

			randomString, err := generateRandomString(8)
			if err != nil {
				return nil, err
			}
			clientID := fmt.Sprintf("%s-%s", clientIdPrefix, randomString)

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

func generateRandomString(n int) (string, error) {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", err
		}
		ret[i] = letters[num.Int64()]
	}

	return string(ret), nil
}
