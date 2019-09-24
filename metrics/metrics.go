package metrics

import (
	"net"
	"net/http"

	"github.com/op/go-logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Make a global private log in `core`
var log = logging.MustGetLogger("metrics")

// Metrics holds the prometheus config.
type Metrics struct {
	Reg  *prometheus.Registry
	Addr string

	l   net.Listener
	mux *http.ServeMux
}

// New returns a new instance of Matrics.
func New(addr string) *Metrics {
	m := &Metrics{
		Addr: addr,
		Reg:  prometheus.NewRegistry(),
	}

	// Add the default collectors
	m.Reg.MustRegister(prometheus.NewGoCollector())
	m.Reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	return m
}

// NewWithMux return metrics server with mux.
func NewWithMux(mux *http.ServeMux) *Metrics {
	m := &Metrics{
		mux: mux,
		Reg: prometheus.NewRegistry(),
	}

	// Add the default collectors
	m.Reg.MustRegister(prometheus.NewGoCollector())
	m.Reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	return m
}

// MustRegister wraps m.Reg.MustRegister.
func (m *Metrics) MustRegister(c prometheus.Collector) {
	m.Reg.MustRegister(c)
}

// OnStartup sets up the metrics on startup.
func (m *Metrics) OnStartup() error {
	if m.mux == nil {
		l, err := net.Listen("tcp", m.Addr)
		if err != nil {
			log.Errorf("Failed to start metrics handler: %s", err)
			return err
		}
		m.l = l
		m.mux = http.NewServeMux()
		go func() {
			http.Serve(m.l, m.mux)
		}()
	}
	log.Infof("Metrics listion on /metrics")
	m.mux.Handle("/metrics", promhttp.HandlerFor(m.Reg, promhttp.HandlerOpts{}))
	return nil
}

// OnFinalShutdown tears down the metrics listener on shutdown and restart.
func (m *Metrics) OnFinalShutdown() error {
	// We allow prometheus statements in multiple Server Blocks, but only the first
	// will open the listener, for the rest they are all nil; guard against that.
	if m.l == nil {
		return nil
	}

	err := m.l.Close()
	m.l = nil
	return err
}
