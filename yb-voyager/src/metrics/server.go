package metrics

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

// Server exposes a Prometheus registry over HTTP on its own ServeMux, so it does
// not touch http.DefaultServeMux and can be started/stopped in tests.
type Server struct {
	port int
	reg  *prometheus.Registry

	mu   sync.Mutex
	addr string
	http *http.Server
}

func NewServer(port int, reg *prometheus.Registry) *Server {
	return &Server{port: port, reg: reg}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(s.reg, promhttp.HandlerOpts{}))

	ln, err := net.Listen("tcp", ":"+strconv.Itoa(s.port))
	if err != nil {
		// Non-fatal: metrics are best-effort and must not abort a migration.
		log.Errorf("metrics: failed to listen on port %d: %v", s.port, err)
		return nil
	}

	s.mu.Lock()
	s.addr = ln.Addr().String()
	s.http = &http.Server{Handler: mux}
	s.mu.Unlock()

	log.Infof("metrics: serving on http://%s/metrics", s.addr)
	go func() {
		if err := s.http.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Errorf("metrics: server error: %v", err)
		}
	}()
	return nil
}

func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	srv := s.http
	s.mu.Unlock()
	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}
