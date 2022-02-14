package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	metricsTotalReceivedPing = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mqtt_total_received_ping",
		Help: "Total number of successful ping",
	}, []string{"self", "other"})

	metricsTotalFailedPing = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mqtt_total_failed_ping",
		Help: "Total number of failed ping",
	}, []string{"self", "other"})
)

// Server contains the metrics server struct
type Server struct {
	httpServer *http.Server
}

// NewMetricsServer returns a metrics server
func NewMetricsServer(addr string, port int) *Server {
	router := mux.NewRouter()
	router.Handle("/metrics", promhttp.Handler())
	listenAddress := net.JoinHostPort(addr, fmt.Sprintf("%d", port))

	srv := &http.Server{
		Addr:    listenAddress,
		Handler: router,
	}

	return &Server{
		httpServer: srv,
	}
}

func (server *Server) Start(ctx context.Context) error {
	err := server.httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "metrics server failed to start or stop gracefully: %v\n", err)
		return err
	}
	return nil
}

func (server *Server) Stop(ctx context.Context) error {
	err := server.httpServer.Shutdown(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "metrics server failed to stop gracefully: %v\n", err)
		return err
	}

	return nil
}
