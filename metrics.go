package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsServer contains the metrics server struct
type MetricsServer struct {
	httpServer *http.Server
}

// NewMetricsServer returns a metrics server
func NewMetricsServer(addr string, port int) *MetricsServer {
	srv := &http.Server{
		Addr:    net.JoinHostPort(addr, fmt.Sprintf("%d", port)),
		Handler: promhttp.Handler(),
	}

	return &MetricsServer{
		httpServer: srv,
	}
}

func (server *MetricsServer) Start(ctx context.Context) error {
	err := server.httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "metrics server failed to start or stop gracefully: %v\n", err)
		return err
	}
	return nil
}

func (server *MetricsServer) Stop(ctx context.Context) error {
	err := server.httpServer.Shutdown(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "metrics server failed to stop gracefully: %v\n", err)
		return err
	}

	return nil
}
