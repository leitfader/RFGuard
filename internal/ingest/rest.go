package ingest

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"rfguard/internal/config"
	"rfguard/internal/model"
	"rfguard/internal/normalize"
)

type RESTServer struct {
	cfg    *config.Manager
	out    chan<- model.NormalizedEvent
	logger *slog.Logger
}

func StartREST(ctx context.Context, cfg *config.Manager, out chan<- model.NormalizedEvent, logger *slog.Logger) *http.Server {
	current := cfg.Get().Ingest.REST
	if !current.Enabled {
		if logger != nil {
			logger.Info("rest ingest disabled")
		}
		return nil
	}
	if logger != nil {
		logger.Info("rest ingest enabled", "addr", current.Addr)
	}
	server := &RESTServer{cfg: cfg, out: out, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("/events", server.handleEvents)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
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
				logger.Error("rest ingest server error", "err", err)
			}
		}
	}()
	return httpServer
}

func (s *RESTServer) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if len(body) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	cfg := s.cfg.Get()
	accepted := 0
	failed := 0

	trim := bytesTrim(body)
	if len(trim) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if trim[0] == '[' {
		var list []map[string]interface{}
		if err := json.Unmarshal(trim, &list); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		for _, obj := range list {
			if err := s.processMap(obj, cfg); err != nil {
				failed++
				continue
			}
			accepted++
		}
	} else {
		var obj map[string]interface{}
		if err := json.Unmarshal(trim, &obj); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := s.processMap(obj, cfg); err != nil {
			failed++
		} else {
			accepted++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"accepted": accepted,
		"failed":   failed,
	})
}

func (s *RESTServer) processMap(obj map[string]interface{}, cfg *config.Config) error {
	fields := ParseJSONMap(obj)
	fields.Raw = "rest"
	ev, err := normalize.Normalize(*fields, cfg)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("rest normalize error", "err", err)
		}
		return err
	}
	ev.Source = "rest"
	SendNonBlocking(context.Background(), s.out, ev, s.logger)
	return nil
}

func bytesTrim(b []byte) []byte {
	start := 0
	for start < len(b) && (b[start] == ' ' || b[start] == '\n' || b[start] == '\r' || b[start] == '\t') {
		start++
	}
	end := len(b)
	for end > start && (b[end-1] == ' ' || b[end-1] == '\n' || b[end-1] == '\r' || b[end-1] == '\t') {
		end--
	}
	return b[start:end]
}
