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

	// Root endpoint with auto-refreshing dashboard
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
	<title>Kartoza Timesheet Monitoring</title>
	<meta http-equiv="refresh" content="2">
	<style>
		body {
			font-family: 'Courier New', monospace;
			padding: 20px;
			background: #1a1a2e;
			color: #eee;
		}
		h1 { color: #DDA036; }
		h2 { color: #569FC6; border-bottom: 1px solid #569FC6; padding-bottom: 5px; }
		.dashboard { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 15px; margin: 20px 0; }
		.metric-card {
			background: #16213e;
			border: 1px solid #0f3460;
			border-radius: 8px;
			padding: 15px;
		}
		.metric-label { color: #888; font-size: 12px; text-transform: uppercase; }
		.metric-value { color: #DDA036; font-size: 28px; font-weight: bold; margin: 5px 0; }
		.metric-unit { color: #666; font-size: 12px; }
		.error { color: #e94560; }
		.success { color: #4ecca3; }
		a { color: #569FC6; }
		pre { background: #16213e; padding: 15px; border-radius: 5px; overflow-x: auto; }
		table { width: 100%%; border-collapse: collapse; margin: 10px 0; }
		th, td { text-align: left; padding: 8px; border-bottom: 1px solid #0f3460; }
		th { color: #DDA036; }
	</style>
</head>
<body>
	<h1>Kartoza Timesheet Monitoring</h1>
	<p style="color: #888;">Auto-refreshing every 2 seconds | <a href="/debug/vars">Raw JSON</a> | <a href="/health">Health</a></p>

	<h2>API Metrics</h2>
	<div class="dashboard" id="metrics">
		<div class="metric-card">
			<div class="metric-label">Total Requests</div>
			<div class="metric-value" id="total">-</div>
		</div>
		<div class="metric-card">
			<div class="metric-label">Errors</div>
			<div class="metric-value error" id="errors">-</div>
		</div>
		<div class="metric-card">
			<div class="metric-label">In Flight</div>
			<div class="metric-value" id="inflight">-</div>
		</div>
		<div class="metric-card">
			<div class="metric-label">Last Duration</div>
			<div class="metric-value" id="duration">-</div>
			<div class="metric-unit">ms</div>
		</div>
		<div class="metric-card">
			<div class="metric-label">Cache Hit Ratio</div>
			<div class="metric-value success" id="cache">-</div>
			<div class="metric-unit">%%</div>
		</div>
	</div>

	<h2>Memory</h2>
	<div class="dashboard">
		<div class="metric-card">
			<div class="metric-label">Heap Alloc</div>
			<div class="metric-value" id="heap">-</div>
			<div class="metric-unit">MB</div>
		</div>
		<div class="metric-card">
			<div class="metric-label">Sys Memory</div>
			<div class="metric-value" id="sys">-</div>
			<div class="metric-unit">MB</div>
		</div>
	</div>

	<h2>Requests by Endpoint</h2>
	<table>
		<thead><tr><th>Endpoint</th><th>Count</th><th>Avg Duration (ms)</th></tr></thead>
		<tbody id="endpoints"></tbody>
	</table>

	<h2>Using expvarmon CLI</h2>
	<pre>
expvarmon -ports="%d" \
  -vars="api.requests.total,api.requests.errors,api.requests.inflight,duration:api.requests.duration_ms,api.cache_hit_ratio,mem:memstats.Alloc,mem:memstats.HeapAlloc" \
  -i 1s
	</pre>

	<script>
		fetch('/debug/vars')
			.then(r => r.json())
			.then(data => {
				document.getElementById('total').textContent = data['api.requests.total'] || 0;
				document.getElementById('errors').textContent = data['api.requests.errors'] || 0;
				document.getElementById('inflight').textContent = data['api.requests.inflight'] || 0;
				document.getElementById('duration').textContent = (data['api.requests.duration_ms'] || 0).toFixed(0);
				document.getElementById('cache').textContent = (data['api.cache_hit_ratio'] || 0).toFixed(1);

				if (data.memstats) {
					document.getElementById('heap').textContent = (data.memstats.HeapAlloc / 1048576).toFixed(2);
					document.getElementById('sys').textContent = (data.memstats.Sys / 1048576).toFixed(2);
				}

				// Populate endpoints table
				const byPath = data['api.requests.by_path'] || {};
				const durations = data['api.duration.by_path'] || {};
				const tbody = document.getElementById('endpoints');
				tbody.innerHTML = '';
				for (const [endpoint, count] of Object.entries(byPath)) {
					const avgDur = durations[endpoint] || '-';
					tbody.innerHTML += '<tr><td>' + endpoint + '</td><td>' + count + '</td><td>' + avgDur + '</td></tr>';
				}
			})
			.catch(console.error);
	</script>
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
