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
