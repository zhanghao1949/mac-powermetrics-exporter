package server

import (
	"log"
	"net/http"

	"mac-powermetrics-exporter/internal/collector"
	"mac-powermetrics-exporter/internal/config"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server represents the HTTP server
type Server struct {
	config *config.Config
}

// New creates a new server instance
func New(cfg *config.Config) *Server {
	return &Server{
		config: cfg,
	}
}

// Start starts the HTTP server with registered collectors
func (s *Server) Start() error {
	// Register collectors
	prometheus.MustRegister(collector.NewPowermetricsCollector())
	prometheus.MustRegister(collector.NewVmStatCollector())

	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Beginning to serve on port %s", s.config.Port)
	return http.ListenAndServe(s.config.Port, nil)
}
