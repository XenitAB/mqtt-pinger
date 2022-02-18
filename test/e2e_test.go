package test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

func TestE2E(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)
	buildImage(t, pool)
	network := createNetwork(t, pool, "test-mqtt-pinger")
	runVerneMQ(t, pool, network)
	runMqttPinger(t, pool, network)
}

func runMqttPinger(t *testing.T, pool *dockertest.Pool, network *dockertest.Network) {
	t.Helper()

	runMqttPingerContainer(t, pool, network)
	waitForMetrics(t)
	validateMqttPinger(t)
}

func runMqttPingerContainer(t *testing.T, pool *dockertest.Pool, network *dockertest.Network) {
	t.Helper()

	// docker run -d --rm --network test-mqtt-pinger -p 8081:8081 --name test-mqtt-pinger test-mqtt-pinger --ping-interval 1 --brokers test-mqtt-pinger-vmq0:1883 test-mqtt-pinger-vmq1:1883 test-mqtt-pinger-vmq2:1883 1>/dev/null
	_, err := pool.RunWithOptions(&dockertest.RunOptions{
		Name:         "test-mqtt-pinger",
		Repository:   "test-mqtt-pinger",
		Cmd:          []string{"--ping-interval", "1", "--brokers", "test-mqtt-pinger-vmq0:1883", "test-mqtt-pinger-vmq1:1883", "test-mqtt-pinger-vmq2:1883"},
		ExposedPorts: []string{"8081"},
		PortBindings: map[docker.Port][]docker.PortBinding{
			docker.Port("8081"): {{
				HostPort: "8081",
			}},
		},
		NetworkID: network.Network.ID,
	}, func(hc *docker.HostConfig) {
		hc.AutoRemove = true
		hc.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})

	if err != nil {
		t.Fatalf("[ERROR] Unable to start mqtt-pinger: %v", err)
	}
}

func waitForMetrics(t *testing.T) {
	t.Helper()

	for i := 1; i < 10; i++ {
		time.Sleep(1 * time.Second)
		res, err := http.Get("http://127.0.0.1:8081/metrics")
		if err != nil {
			continue
		}

		body := res.Body
		defer body.Close()

		var parser expfmt.TextParser
		mf, err := parser.TextToMetricFamilies(body)
		if err != nil {
			continue
		}

		for k, v := range mf {
			if k == "mqtt_total_received_ping" {
				metrics := v.GetMetric()
				if len(metrics) == 6 {
					t.Logf("[INFO] mqtt-pinger ready")
					return
				}
			}
		}
	}

	t.Fatalf("[ERROR] Unable to get metrics")
}

func validateMqttPinger(t *testing.T) {
	t.Helper()

	for i := 1; i < 10; i++ {
		metrics := getMqttPingerMetrics(t)
		count := 0
		for _, metric := range metrics {
			if metric.Counter.GetValue() > 4 {
				count++
			}
		}

		if count == 6 {
			t.Logf("[INFO] Successfully validated mqtt-pinger")
			return
		}

		time.Sleep(2 * time.Second)
	}

	t.Fatalf("[ERROR] Unable to get metrics")
}

func getMqttPingerMetrics(t *testing.T) []*dto.Metric {
	t.Helper()

	time.Sleep(5 * time.Second)
	res, err := http.Get("http://127.0.0.1:8081/metrics")
	if err != nil {
		t.Fatalf("[ERROR] Unable to request metrics: %v", err)
	}

	body := res.Body
	defer body.Close()

	var parser expfmt.TextParser
	mf, err := parser.TextToMetricFamilies(body)
	if err != nil {
		t.Fatalf("[ERROR] Unable to extract metrics: %v", err)
	}

	var metrics []*dto.Metric
	for k, v := range mf {
		if k == "mqtt_total_received_ping" {
			metrics = v.GetMetric()
		}
	}

	if len(metrics) == 0 {
		t.Fatalf("[ERROR] No metrics found")
	}

	return metrics
}

func newPool(t *testing.T) *dockertest.Pool {
	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatalf("[ERROR] Unable to get dockertest pool: %v", err)
	}

	return pool
}

func buildImage(t *testing.T, pool *dockertest.Pool) {
	t.Helper()

	err := pool.Client.BuildImage(docker.BuildImageOptions{
		Name:         "test-mqtt-pinger",
		Dockerfile:   "./Dockerfile",
		OutputStream: ioutil.Discard,
		ContextDir:   "../",
	})
	if err != nil {
		t.Fatalf("[ERROR] Unable to build image: %v", err)
	}
}

func createNetwork(t *testing.T, pool *dockertest.Pool, networkName string) *dockertest.Network {
	t.Helper()

	network, err := pool.CreateNetwork(networkName, func(config *docker.CreateNetworkOptions) {
		config.Driver = "bridge"
	})
	if err != nil {
		t.Fatalf("[ERROR] Unable to create network %q: %v", networkName, err)
	}

	return network
}

func cleanup(t *testing.T, pool *dockertest.Pool) {
	t.Helper()

	purgeContainer(t, pool, "test-mqtt-pinger-vmq0")
	purgeContainer(t, pool, "test-mqtt-pinger-vmq1")
	purgeContainer(t, pool, "test-mqtt-pinger-vmq2")
	purgeContainer(t, pool, "test-mqtt-pinger")
	removeNetwork(t, pool, "test-mqtt-pinger")
}

func purgeContainer(t *testing.T, pool *dockertest.Pool, containerName string) {
	t.Helper()

	resource, ok := pool.ContainerByName(containerName)
	if ok {
		t.Logf("[INFO] Purging container %q", containerName)
		err := pool.Purge(resource)
		if err != nil {
			t.Logf("[INFO] Unable to purge container %q: %v", containerName, err)
		}
	}
}

func removeNetwork(t *testing.T, pool *dockertest.Pool, networkName string) {
	t.Helper()

	networks, err := pool.Client.ListNetworks()
	if err != nil {
		t.Log("[INFO] Unable to find any networks")
		return
	}

	var networksToRemove []dockertest.Network
	for i := range networks {
		if networks[i].Name == networkName {
			networksToRemove = append(networksToRemove, dockertest.Network{
				Network: &networks[i],
			})
		}
	}

	for i := range networksToRemove {
		t.Logf("[INFO] Removing network %q", networksToRemove[i].Network.Name)
		err := pool.RemoveNetwork(&networksToRemove[i])
		if err != nil {
			t.Logf("[INFO] Unable to remove network %q: %v", networksToRemove[i].Network.Name, err)
		}
	}
}

func runVerneMQ(t *testing.T, pool *dockertest.Pool, network *dockertest.Network) {
	vmq0 := runVerneMQContainer(t, pool, network, "test-mqtt-pinger-vmq0", []string{
		"DOCKER_VERNEMQ_ACCEPT_EULA=yes",
		"DOCKER_VERNEMQ_ALLOW_ANONYMOUS=on",
	}, "1883:1883")

	ip := vmq0.GetIPInNetwork(network)

	_ = runVerneMQContainer(t, pool, network, "test-mqtt-pinger-vmq1", []string{
		"DOCKER_VERNEMQ_ACCEPT_EULA=yes",
		"DOCKER_VERNEMQ_ALLOW_ANONYMOUS=on",
		fmt.Sprintf("DOCKER_VERNEMQ_DISCOVERY_NODE=%s", ip),
	}, "1884:1883")

	_ = runVerneMQContainer(t, pool, network, "test-mqtt-pinger-vmq2", []string{
		"DOCKER_VERNEMQ_ACCEPT_EULA=yes",
		"DOCKER_VERNEMQ_ALLOW_ANONYMOUS=on",
		fmt.Sprintf("DOCKER_VERNEMQ_DISCOVERY_NODE=%s", ip),
	}, "1885:1883")

	waitForVerneMQCluster(t, vmq0)
}

func waitForVerneMQCluster(t *testing.T, resource *dockertest.Resource) {
	t.Helper()

	for i := 1; i < 10; i++ {
		time.Sleep(5 * time.Second)

		var b bytes.Buffer
		output := io.Writer(&b)
		exitCode, err := resource.Exec([]string{"vmq-admin", "cluster", "show"}, dockertest.ExecOptions{
			StdOut: output,
		})
		if err != nil {
			t.Fatalf("[ERROR] Unable run vmq-admin: %v", err)
		}

		if exitCode != 0 {
			continue
		}

		r, err := regexp.Compile("VerneMQ@.*true")
		if err != nil {
			t.Fatalf("[ERROR] Unable to create regex for VerneMQ output")
		}

		outputBytes, err := io.ReadAll(&b)
		if err != nil {
			t.Logf("[INFO] Unable to read output from vmq-admin: %v", err)
		}

		matches := r.FindAllStringSubmatchIndex(string(outputBytes), -1)
		if len(matches) == 3 {
			t.Logf("[INFO] VerneMQ cluster ready")
			return
		}
	}

	t.Fatalf("[ERROR] Unable to start VerneMQ cluster")
}

func runVerneMQContainer(t *testing.T, pool *dockertest.Pool, network *dockertest.Network, containerName string, env []string, port string) *dockertest.Resource {
	t.Helper()

	portParts := strings.Split(port, ":")
	if len(portParts) != 2 {
		t.Fatalf("[ERROR] port should be in format 123:123 (external:internal) but received: %s", port)
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Name:       containerName,
		Repository: "vernemq/vernemq",
		Env:        env,
		NetworkID:  network.Network.ID,
	}, func(hc *docker.HostConfig) {
		hc.AutoRemove = true
		hc.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})

	if err != nil {
		t.Fatalf("[ERROR] Unable to start VerneMQ container %q: %v", containerName, err)
	}

	return resource
}
