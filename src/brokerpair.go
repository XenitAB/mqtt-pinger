package main

import "fmt"

type brokerPair struct {
	source      string
	destination string
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
			pairs = append(pairs, brokerPair{source, destination})
		}
	}

	return pairs, nil
}
