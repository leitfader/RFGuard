package api

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rfguard/internal/alerts"
	"rfguard/internal/api/web"
	"rfguard/internal/config"
	"rfguard/internal/metrics"
	"rfguard/internal/model"
)

type EngineControl interface {
	Reset()
	UpdateConfig(cfg *config.Config)
}

type Server struct {
	cfg     *config.Manager
	metrics *metrics.Store
	alerts  *alerts.Store
	engine  EngineControl
	logger  *slog.Logger
	version string
}

type statusResponse struct {
	Status        string                     `json:"status"`
	Time          string                     `json:"time"`
	Version       string                     `json:"version"`
	ConfigPath    string                     `json:"config_path"`
	AccessControl config.AccessControlConfig `json:"access_control"`
	Ingest        ingestStatus               `json:"ingest"`
	API           apiStatus                  `json:"api"`
	Detection     detectionStatus            `json:"detection"`
}

type ingestStatus struct {
	REST      bool `json:"rest"`
	Syslog    bool `json:"syslog"`
	FileTail  bool `json:"file_tail"`
	TCPStream bool `json:"tcp_stream"`
	Kafka     bool `json:"kafka"`
}

type apiStatus struct {
	Enabled bool   `json:"enabled"`
	Addr    string `json:"addr"`
}

type detectionStatus struct {
	Windows []string `json:"windows"`
}

func Start(ctx context.Context, cfg *config.Manager, metricsStore *metrics.Store, alertsStore *alerts.Store, engine EngineControl, logger *slog.Logger, version string) *http.Server {
	if cfg == nil {
		return nil
	}
	current := cfg.Get().API
	if !current.Enabled {
		if logger != nil {
			logger.Info("api disabled")
		}
		return nil
	}
	if logger != nil {
		logger.Info("api enabled", "addr", current.Addr)
	}
	server := &Server{
		cfg:     cfg,
		metrics: metricsStore,
		alerts:  alertsStore,
		engine:  engine,
		logger:  logger,
		version: version,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/status", server.handleStatus)
	mux.HandleFunc("/metrics", server.handleMetrics)
	mux.HandleFunc("/metrics/", server.handleMetrics)
	mux.HandleFunc("/alerts", server.handleAlerts)
	mux.HandleFunc("/config/access_control", server.handleAccessControl)
	mux.HandleFunc("/admin/clear", server.handleClear)
	mux.HandleFunc("/admin/restart", server.handleRestart)
	mux.HandleFunc("/ui", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ui/", http.StatusMovedPermanently)
	})
	uiFS, err := fs.Sub(web.FS, ".")
	if err == nil {
		mux.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(http.FS(uiFS))))
	}

	httpServer := &http.Server{Addr: current.Addr, Handler: mux}
	go func() {
		<-ctx.Done()
		ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(ctxShutdown)
	}()
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			if logger != nil {
				logger.Error("api server error", "err", err)
			}
		}
	}()
	return httpServer
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	cfg := s.cfg.Get()
	windows := make([]string, 0, len(cfg.Detection.Windows))
	for _, d := range cfg.Detection.Windows {
		windows = append(windows, d.String())
	}
	resp := statusResponse{
		Status:        "ok",
		Time:          time.Now().UTC().Format(time.RFC3339Nano),
		Version:       s.version,
		ConfigPath:    s.cfg.Path(),
		AccessControl: cfg.AccessControl,
		Ingest: ingestStatus{
			REST:      cfg.Ingest.REST.Enabled,
			Syslog:    cfg.Ingest.Syslog.Enabled,
			FileTail:  cfg.Ingest.FileTail.Enabled,
			TCPStream: cfg.Ingest.TCPStream.Enabled,
			Kafka:     cfg.Ingest.Kafka.Enabled,
		},
		API: apiStatus{Enabled: cfg.API.Enabled, Addr: cfg.API.Addr},
		Detection: detectionStatus{
			Windows: windows,
		},
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/metrics")
	path = strings.TrimPrefix(path, "/")
	if path != "" {
		metrics, updated, ok := s.metrics.Get(path)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"reader_id":  path,
			"updated_at": updated.Format(time.RFC3339Nano),
			"metrics":    metrics,
		})
		return
	}
	all := s.metrics.GetAll()
	writeJSON(w, http.StatusOK, map[string]any{
		"metrics": all,
		"count":   len(all),
	})
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	limit := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	sinceStr := r.URL.Query().Get("since")
	var list []model.Alert
	if sinceStr != "" {
		if ts, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			list = s.alerts.Since(ts)
		} else {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	} else {
		list = s.alerts.List(limit)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"alerts": list,
		"count":  len(list),
	})
}

func (s *Server) handleAccessControl(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := s.cfg.Get()
		writeJSON(w, http.StatusOK, map[string]any{
			"access_control": cfg.AccessControl,
		})
		return
	case http.MethodPost:
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var ac config.AccessControlConfig
		if err := json.Unmarshal(body, &ac); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		ac.Whitelist = sanitizeUIDList(ac.Whitelist)
		ac.Blacklist = sanitizeUIDList(ac.Blacklist)
		ac.ReaderWhitelists = sanitizeReaderLists(ac.ReaderWhitelists)
		ac.ReaderBlacklists = sanitizeReaderLists(ac.ReaderBlacklists)
		current := s.cfg.Get()
		next := *current
		next.AccessControl = ac
		if err := s.cfg.Update(&next); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if s.engine != nil {
			s.engine.UpdateConfig(&next)
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	body, _ := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	var req struct {
		Target string `json:"target"`
	}
	_ = json.Unmarshal(body, &req)
	target := strings.ToLower(strings.TrimSpace(req.Target))
	if target == "" {
		target = "all"
	}
	switch target {
	case "all":
		if s.metrics != nil {
			s.metrics.Clear()
		}
		if s.alerts != nil {
			s.alerts.Clear()
		}
	case "alerts", "logs":
		if s.alerts != nil {
			s.alerts.Clear()
		}
	case "metrics":
		if s.metrics != nil {
			s.metrics.Clear()
		}
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.engine != nil {
		s.engine.Reset()
	}
	if s.metrics != nil {
		s.metrics.Clear()
	}
	if s.alerts != nil {
		s.alerts.Clear()
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func sanitizeUIDList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return out
}

func sanitizeReaderLists(values map[string][]string) map[string][]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string][]string, len(values))
	for reader, list := range values {
		reader = strings.TrimSpace(reader)
		if reader == "" {
			continue
		}
		clean := sanitizeUIDList(list)
		if len(clean) == 0 {
			continue
		}
		out[reader] = clean
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
