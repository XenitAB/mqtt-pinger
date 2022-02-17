#!/bin/bash
set -ex

timestamp() {
  date +"%T"
}

cleanup() {
  set +e
  docker kill $(docker ps -f name=test-mqtt-pinger-vmq0 -q) 1>/dev/null 2>&1
  docker kill $(docker ps -f name=test-mqtt-pinger-vmq1 -q) 1>/dev/null 2>&1
  docker kill $(docker ps -f name=test-mqtt-pinger-vmq2 -q) 1>/dev/null 2>&1
  docker kill $(docker ps -f name=test-mqtt-pinger -q) 1>/dev/null 2>&1
  docker network rm test-mqtt-pinger 1>/dev/null 2>&1
  set -e
}

wait_for_cluster() {
  for delay in 5 5 5 5 10 15 20; do
    if [[ "$(docker exec test-mqtt-pinger-vmq0 vmq-admin cluster show | grep "VerneMQ@" | grep "true" | wc -l)" == "3" ]]; then
      return
    fi
    sleep $delay
  done

  exit 1
}

cleanup
trap cleanup EXIT

docker build . -t test-mqtt-pinger 1>/dev/null

docker network create --driver bridge test-mqtt-pinger 1>/dev/null
docker run -d --rm --network test-mqtt-pinger -e "DOCKER_VERNEMQ_ACCEPT_EULA=yes" -e "DOCKER_VERNEMQ_ALLOW_ANONYMOUS=on" -p 1883:1883 --name test-mqtt-pinger-vmq0 vernemq/vernemq 1>/dev/null
FIRST_VERNEMQ_IP=$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' test-mqtt-pinger-vmq0)
docker run -d --rm --network test-mqtt-pinger -e "DOCKER_VERNEMQ_ACCEPT_EULA=yes" -e "DOCKER_VERNEMQ_ALLOW_ANONYMOUS=on" -e "DOCKER_VERNEMQ_DISCOVERY_NODE=${FIRST_VERNEMQ_IP}" -p 1884:1883 --name test-mqtt-pinger-vmq1 vernemq/vernemq 1>/dev/null
docker run -d --rm --network test-mqtt-pinger -e "DOCKER_VERNEMQ_ACCEPT_EULA=yes" -e "DOCKER_VERNEMQ_ALLOW_ANONYMOUS=on" -e "DOCKER_VERNEMQ_DISCOVERY_NODE=${FIRST_VERNEMQ_IP}" -p 1885:1883 --name test-mqtt-pinger-vmq2 vernemq/vernemq 1>/dev/null

wait_for_cluster

docker run -d --rm --network test-mqtt-pinger -p 8081:8081 --name test-mqtt-pinger test-mqtt-pinger --broker-address test-mqtt-pinger-vmq0,test-mqtt-pinger-vmq1,test-mqtt-pinger-vmq2 1>/dev/null

exit 0