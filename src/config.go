package main

type config struct {
	Brokers        []string
	MetricsAddress string
	MetricsPort    int
	TickerSeconds  int
}

func newConfig() (config, error) {
	cfg := config{
		Brokers: []string{
			"127.0.0.1:1883",
			"127.0.0.1:1884",
			"127.0.0.1:1885",
		},
		MetricsAddress: "0.0.0.0",
		MetricsPort:    8081,
		TickerSeconds:  10,
	}
	return cfg, nil
}
