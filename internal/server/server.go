package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os/exec"
	"sync"
	"time"

	"github.com/jc/up/internal/domain"
	"github.com/jc/up/internal/executor"
	"github.com/jc/up/internal/scanner"
)

type Config struct {
	DevMode      bool
	ViteDevURL   string
	StaticFS     fs.FS
	StaticDir    string
	Scanner      *scanner.Scanner
	Executor     *executor.Executor
}

type Server struct {
	cfg        Config
	mux        *http.ServeMux
	clientsMu  sync.Mutex
	clients    map[chan domain.UpgradeEvent]struct{}
	lastResult *domain.ScanResult
	resultMu   sync.RWMutex
}

func New(cfg Config) *Server {
	s := &Server{
		cfg:     cfg,
		mux:     http.NewServeMux(),
		clients: make(map[chan domain.UpgradeEvent]struct{}),
	}

	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/api/outdated", s.handleOutdated)
	s.mux.HandleFunc("/api/upgrade", s.handleUpgrade)
	s.mux.HandleFunc("/api/clean", s.handleClean)
	s.mux.HandleFunc("/api/events", s.handleEvents)

	// Frontend handler
	if s.cfg.DevMode && s.cfg.ViteDevURL != "" {
		targetURL, err := url.Parse(s.cfg.ViteDevURL)
		if err == nil {
			proxy := httputil.NewSingleHostReverseProxy(targetURL)
			s.mux.Handle("/", proxy)
			return
		}
	}

	if s.cfg.StaticFS != nil {
		s.mux.Handle("/", http.FileServer(http.FS(s.cfg.StaticFS)))
	} else if s.cfg.StaticDir != "" {
		s.mux.Handle("/", http.FileServer(http.Dir(s.cfg.StaticDir)))
	} else {
		s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<html><body><h3>UP API running. Build frontend with 'vp build' in web/</h3></body></html>")
		})
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for local dev
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	s.mux.ServeHTTP(w, r)
}

func (s *Server) broadcastEvent(event domain.UpgradeEvent) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	for ch := range s.clients {
		select {
		case ch <- event:
		default:
		}
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	sudoActive := exec.Command("sudo", "-n", "true").Run() == nil

	resp := map[string]interface{}{
		"status":      "ok",
		"sudo_active": sudoActive,
		"platform":    "darwin",
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleOutdated(w http.ResponseWriter, r *http.Request) {
	refresh := r.URL.Query().Get("refresh") == "true"

	s.resultMu.RLock()
	cached := s.lastResult
	s.resultMu.RUnlock()

	if cached != nil && !refresh {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(cached)
		return
	}

	res, err := s.cfg.Scanner.Scan(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	s.resultMu.Lock()
	s.lastResult = res
	s.resultMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req domain.UpgradeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	s.resultMu.RLock()
	cached := s.lastResult
	s.resultMu.RUnlock()

	if cached == nil {
		http.Error(w, "No scan result available. Run /api/outdated first", http.StatusBadRequest)
		return
	}

	// Filter packages to upgrade
	selectedSet := make(map[string]bool)
	for _, id := range req.Packages {
		selectedSet[id] = true
	}

	var toUpgrade []domain.Package
	for _, p := range cached.Packages {
		if selectedSet[p.ID] || selectedSet[p.Name] {
			toUpgrade = append(toUpgrade, p)
		}
	}

	if len(toUpgrade) == 0 {
		http.Error(w, "No matching packages selected", http.StatusBadRequest)
		return
	}

	// Run in background and stream events via SSE
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		_ = s.cfg.Executor.ExecuteUpgrades(ctx, toUpgrade, func(ev domain.UpgradeEvent) {
			s.broadcastEvent(ev)
		})

		// Invalidate cache
		s.resultMu.Lock()
		s.lastResult = nil
		s.resultMu.Unlock()
	}()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "started",
		"count":   len(toUpgrade),
		"message": fmt.Sprintf("Upgrade queued for %d packages", len(toUpgrade)),
	})
}

func (s *Server) handleClean(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		_ = s.cfg.Executor.ExecuteClean(ctx, func(ev domain.UpgradeEvent) {
			s.broadcastEvent(ev)
		})
	}()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "Cleanup job queued",
	})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	eventCh := make(chan domain.UpgradeEvent, 100)

	s.clientsMu.Lock()
	s.clients[eventCh] = struct{}{}
	s.clientsMu.Unlock()

	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, eventCh)
		close(eventCh)
		s.clientsMu.Unlock()
	}()

	// Send initial ping
	fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
	flusher.Flush()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case ev := <-eventCh:
			data, err := json.Marshal(ev)
			if err == nil {
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	}
}
