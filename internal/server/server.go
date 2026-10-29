package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jankrloz/vanguard/internal/config"
	"github.com/jankrloz/vanguard/internal/domain"
	"github.com/jankrloz/vanguard/internal/executor"
	"github.com/jankrloz/vanguard/internal/history"
)

// Scanner interface decouples the server from package scanning engines.
type Scanner interface {
	Scan(ctx context.Context) (*domain.ScanResult, error)
}

// Executor interface decouples the server from system execution engines.
type Executor interface {
	ExecuteUpgrades(ctx context.Context, pkgs []domain.Package, onEvent executor.EventCallback) error
	ExecuteClean(ctx context.Context, onEvent executor.EventCallback) error
	ExecuteTopgrade(ctx context.Context, onEvent executor.EventCallback) error
}

type Config struct {
	Version    string
	DevMode    bool
	ViteDevURL string
	StaticFS   fs.FS
	StaticDir  string
	Scanner    Scanner
	Executor   Executor
}

type Server struct {
	cfg        Config
	mux        *http.ServeMux
	clientsMu  sync.Mutex
	clients    map[chan domain.UpgradeEvent]struct{}
	lastResult *domain.ScanResult
	resultMu   sync.RWMutex
	scanMu     sync.Mutex
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
	s.mux.HandleFunc("/api/topgrade", s.handleTopgrade)
	s.mux.HandleFunc("/api/history", s.handleHistory)
	s.mux.HandleFunc("/api/config/pin", s.handlePin)
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
			fmt.Fprintf(w, "<html><body><h3>Vanguard API running. Build frontend with 'task build'</h3></body></html>")
		})
	}
}

func isLoopbackHost(hostPort string) bool {
	if hostPort == "" || hostPort == "example.com" {
		return true
	}
	host := hostPort
	if h, _, err := net.SplitHostPort(hostPort); err == nil {
		host = h
	}
	host = strings.TrimPrefix(strings.TrimSuffix(strings.ToLower(host), "]"), "[")
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func isTrustedOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	return isLoopbackHost(u.Host) && u.Hostname() != "example.com"
}

func writeJSONError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. Guard against DNS rebinding via Host header check
	if !isLoopbackHost(r.Host) {
		writeJSONError(w, "forbidden host", http.StatusForbidden)
		return
	}

	// 2. Guard against Cross-Site Request Forgery (CSRF) & cross-origin exfiltration
	origin := r.Header.Get("Origin")
	if origin != "" {
		if !isTrustedOrigin(origin) {
			writeJSONError(w, "untrusted origin", http.StatusForbidden)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	}

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 3. Enforce 1 MiB body size limit on mutating requests
	if r.Method == http.MethodPost && r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
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

	version := s.cfg.Version
	if version == "" {
		version = "0.3.0"
	}

	resp := map[string]interface{}{
		"status":      "ok",
		"version":     version,
		"sudo_active": sudoActive,
		"platform":    runtime.GOOS,
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleOutdated(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Scanner == nil {
		http.Error(w, `{"error": "scanner not configured"}`, http.StatusInternalServerError)
		return
	}

	refresh := r.URL.Query().Get("refresh") == "true"

	s.resultMu.RLock()
	cached := s.lastResult
	s.resultMu.RUnlock()

	if cached != nil && !refresh {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(cached)
		return
	}

	s.scanMu.Lock()
	defer s.scanMu.Unlock()

	// Double-check cache in case another concurrent request completed while waiting
	s.resultMu.RLock()
	cached = s.lastResult
	s.resultMu.RUnlock()
	if cached != nil && !refresh {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(cached)
		return
	}

	res, err := s.cfg.Scanner.Scan(r.Context())
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
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

	if s.cfg.Executor == nil {
		http.Error(w, "Executor not configured", http.StatusInternalServerError)
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

	if s.cfg.Executor == nil {
		http.Error(w, "Executor not configured", http.StatusInternalServerError)
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

func (s *Server) handleTopgrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.cfg.Executor == nil {
		http.Error(w, "Executor not configured", http.StatusInternalServerError)
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		_ = s.cfg.Executor.ExecuteTopgrade(ctx, func(ev domain.UpgradeEvent) {
			s.broadcastEvent(ev)
		})
	}()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "Topgrade system sweep queued",
	})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	entries, err := history.ReadRecent(limit)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(entries)
}

func (s *Server) handlePin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PackageID string `json:"package_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PackageID == "" {
		http.Error(w, "package_id is required", http.StatusBadRequest)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pinned, err := cfg.TogglePin(req.PackageID)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"package_id": req.PackageID,
		"pinned":     pinned,
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
