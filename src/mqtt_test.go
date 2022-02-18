package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	hmqBroker "github.com/fhmq/hmq/broker"
	"github.com/phayes/freeport"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port, err := freeport.GetFreePort()
	require.NoError(t, err)

	metrics := NewMetricsServer("127.0.0.1", port)
	go func() {
		err := metrics.Start()
		if err != nil {
			fmt.Fprintf(os.Stderr, "unable to start metrics server: %v\n", err)
			cancel()
		}
	}()

	args := []string{""}
	hmqConfig, err := hmqBroker.ConfigureConfig(args)
	require.NoError(t, err)

	mqttBroker, err := hmqBroker.NewBroker(hmqConfig)
	require.NoError(t, err)
	mqttBroker.Start()

	mockBroker := net.JoinHostPort(hmqConfig.Host, hmqConfig.Port)

	// Check that the in-memory broker is started
	for start := time.Now(); time.Since(start) < 5*time.Second; {
		conn, err := net.Dial("tcp", mockBroker)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	p := brokerPair{
		source:            mockBroker,
		destination:       "foobar",
		base64Source:      "foo",
		base64Destination: "bar",
		clientID:          "foobar",
		subscriptionTopic: "baz",
		publishTopic:      "baz",
	}

	g, gCtx := errgroup.WithContext(ctx)
	pinger := NewPingClient(&p, time.Duration(10*time.Millisecond))
	g.Go(func() error {
		return pinger.Run(gCtx)
	})

	time.Sleep(400 * time.Millisecond)

	cancel()

	err = g.Wait()
	require.NoError(t, err)

	successMetrics := getMetrics(t, port, "mqtt_total_received_ping")
	failedMetrics := getMetrics(t, port, "mqtt_total_failed_ping")

	require.Len(t, successMetrics, 1)
	require.Len(t, failedMetrics, 1)

	t.Logf("Success metrics: %v", successMetrics[0].GetCounter().GetValue())
	t.Logf("Failed metrics: %v", failedMetrics[0].GetCounter().GetValue())

	require.GreaterOrEqual(t, successMetrics[0].GetCounter().GetValue(), float64(5))
	require.LessOrEqual(t, failedMetrics[0].GetCounter().GetValue(), float64(1))

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	shutdownErrG, shutdownErrGCtx := errgroup.WithContext(shutdownCtx)
	shutdownErrG.Go(func() error {
		return metrics.Stop(shutdownErrGCtx)
	})

	err = shutdownErrG.Wait()
	require.NoError(t, err)
}

func getMetrics(t *testing.T, port int, metricName string) []*dto.Metric {
	t.Helper()

	time.Sleep(5 * time.Second)
	res, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/metrics", port))
	require.NoError(t, err)

	body := res.Body
	defer body.Close()

	var parser expfmt.TextParser
	mf, err := parser.TextToMetricFamilies(body)
	require.NoError(t, err)

	var metrics []*dto.Metric
	for k, v := range mf {
		if k == metricName {
			metrics = v.GetMetric()
		}
	}

	if len(metrics) == 0 {
		t.Fatalf("[ERROR] No metrics found")
	}

	return metrics
}
