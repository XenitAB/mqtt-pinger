package main

import "github.com/alexflint/go-arg"

type config struct {
	Brokers        []string `arg:"--brokers,env:BROKERS" help:"the brokers to send pings between"`
	MetricsAddress string   `arg:"--metrics-address,env:METRICS_ADDRESS" default:"0.0.0.0" help:"the address to use for the metrics http listener"`
	MetricsPort    int      `arg:"--metrics-port,env:METRICS_PORT" default:"8081" help:"the metrics port to use for the http listener"`
	PingInterval   int      `arg:"--ping-interval,env:PING_INTERVAL" default:"10" help:"the interval sleeping after publishing ping messages"`
}

func loadConfig(args []string) (config, error) {
	argCfg := arg.Config{
		Program:   "mqtt-pinger",
		IgnoreEnv: false,
	}

	var cfg config
	parser, err := arg.NewParser(argCfg, &cfg)
	if err != nil {
		return config{}, err
	}

	err = parser.Parse(args)
	if err != nil {
		return config{}, err
	}

	return cfg, nil
}
