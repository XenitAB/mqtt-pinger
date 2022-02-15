package main

import "fmt"

type pair struct {
	source      string
	destination string
}

func getPairs(list []string) ([]pair, error) {
	if len(list) < 2 {
		return nil, fmt.Errorf("received %d item(s) in list but at least 2 are required", len(list))
	}

	var pairs []pair
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
			pairs = append(pairs, pair{source, destination})
		}
	}

	return pairs, nil
}
