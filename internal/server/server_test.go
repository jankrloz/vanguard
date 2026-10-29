package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jankrloz/vanguard/internal/domain"
	"github.com/jankrloz/vanguard/internal/executor"
)

type mockScanner struct {
	result *domain.ScanResult
	err    error
}

func (m *mockScanner) Scan(ctx context.Context) (*domain.ScanResult, error) {
	return m.result, m.err
}

type mockExecutor struct {
	upgraded []domain.Package
}

func (m *mockExecutor) ExecuteUpgrades(ctx context.Context, pkgs []domain.Package, onEvent executor.EventCallback) error {
	m.upgraded = pkgs
	onEvent(domain.UpgradeEvent{Type: "complete", Message: "ok"})
	return nil
}

func (m *mockExecutor) ExecuteClean(ctx context.Context, onEvent executor.EventCallback) error {
	onEvent(domain.UpgradeEvent{Type: "complete", Message: "clean ok"})
	return nil
}

func (m *mockExecutor) ExecuteTopgrade(ctx context.Context, onEvent executor.EventCallback) error {
	onEvent(domain.UpgradeEvent{Type: "complete", Message: "topgrade ok"})
	return nil
}

func TestServerEndpoints(t *testing.T) {
	t.Setenv("VANGUARD_CONFIG_DIR", t.TempDir())
	t.Setenv("VANGUARD_STATE_DIR", t.TempDir())

	scanner := &mockScanner{
		result: &domain.ScanResult{
			Packages: []domain.Package{
				{
					ID:             "npm:test-pkg",
					Name:           "test-pkg",
					Manager:        "npm",
					CurrentVersion: "1.0.0",
					LatestVersion:  "1.0.1",
					ChangeType:     domain.ChangePatch,
					Safe:           true,
					Selected:       true,
				},
			},
			Summary: domain.ScanSummary{
				TotalPackages: 1,
				SafeCount:     1,
			},
		},
	}
	exec := &mockExecutor{}

	srv := New(Config{
		Scanner:  scanner,
		Executor: exec,
	})

	// 1. Test /api/status
	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/status = %d, want 200", w.Code)
	}

	var statusResp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &statusResp)
	if statusResp["status"] != "ok" {
		t.Errorf("status != ok: %v", statusResp)
	}

	// 2. Test /api/outdated
	req = httptest.NewRequest("GET", "/api/outdated", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/outdated = %d, want 200", w.Code)
	}

	var scanResult domain.ScanResult
	_ = json.Unmarshal(w.Body.Bytes(), &scanResult)
	if scanResult.Summary.TotalPackages != 1 || len(scanResult.Packages) != 1 {
		t.Errorf("unexpected scan result: %+v", scanResult)
	}

	// 3. Test /api/upgrade
	body, _ := json.Marshal(domain.UpgradeRequest{Packages: []string{"npm:test-pkg"}})
	req = httptest.NewRequest("POST", "/api/upgrade", bytes.NewReader(body))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/upgrade = %d, want 200: %s", w.Code, w.Body.String())
	}

	// 4. Test /api/topgrade
	req = httptest.NewRequest("POST", "/api/topgrade", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/topgrade = %d, want 200: %s", w.Code, w.Body.String())
	}

	// 5. Test /api/history
	req = httptest.NewRequest("GET", "/api/history?limit=10", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/history = %d, want 200: %s", w.Code, w.Body.String())
	}

	// 6. Test /api/config/pin
	pinBody, _ := json.Marshal(map[string]string{"package_id": "npm:test-pkg"})
	req = httptest.NewRequest("POST", "/api/config/pin", bytes.NewReader(pinBody))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/config/pin = %d, want 200: %s", w.Code, w.Body.String())
	}

	// 7. Test cross-origin CSRF rejection & DNS rebinding protection
	req = httptest.NewRequest("POST", "/api/topgrade", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("POST /api/topgrade with evil Origin = %d, want 403", w.Code)
	}

	req = httptest.NewRequest("GET", "/api/outdated", nil)
	req.Host = "evil-rebind.attacker.com:9099"
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("GET /api/outdated with DNS rebinding Host = %d, want 403", w.Code)
	}
}
