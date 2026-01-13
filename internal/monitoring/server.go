package monitoring

import (
	"context"
	"expvar"
	"fmt"
	"net/http"
	"time"
)

// Server runs the monitoring HTTP server
type Server struct {
	httpServer *http.Server
	port       int
}

// NewServer creates a new monitoring server
func NewServer(port int) *Server {
	return &Server{
		port: port,
	}
}

// Start starts the monitoring HTTP server in the background
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// expvar handler automatically registered at /debug/vars
	mux.Handle("/debug/vars", expvar.Handler())

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK\n")
	})

	// Root endpoint with links
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
	<title>Kartoza Timesheet Monitoring</title>
	<style>
		body { font-family: monospace; padding: 20px; }
		a { display: block; margin: 10px 0; }
	</style>
</head>
<body>
	<h1>Kartoza Timesheet Monitoring</h1>
	<p>Monitoring endpoints:</p>
	<a href="/debug/vars">Raw Metrics (JSON)</a>
	<a href="/health">Health Check</a>

	<h2>Using expvarmon</h2>
	<pre>
# Install expvarmon
go install github.com/divan/expvarmon@latest

# Monitor the application
expvarmon -ports="localhost:%d" \
  -vars="api.requests.total,api.requests.errors,api.requests.inflight,api.cache_hit_ratio" \
  -i 1s
	</pre>
</body>
</html>`, s.port)
	})

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start in background
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Monitoring server error: %v\n", err)
		}
	}()

	return nil
}

// Stop gracefully stops the monitoring server
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
