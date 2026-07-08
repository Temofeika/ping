package gui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Temofeika/ping/internal/storage"
)

func TestGUIServerHandlers(t *testing.T) {
	st, err := storage.NewStorage(":memory:")
	if err != nil {
		t.Fatalf("ошибка in-memory БД: %v", err)
	}
	defer st.Close()

	// Запишем тестовые логи
	_ = st.SavePing(storage.PingLog{
		Timestamp: time.Now().Add(-10 * time.Minute),
		Target:    "192.168.1.50",
		Status:    "success",
		RTT:       5 * time.Millisecond,
	})

	srv := NewServer(st, 8585)

	// Тест GET /api/targets
	req := httptest.NewRequest(http.MethodGet, "/api/targets", nil)
	w := httptest.NewRecorder()
	srv.handleGetTargets(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("ожидался статус 200, получено %d", resp.StatusCode)
	}

	var targetsData map[string][]string
	_ = json.NewDecoder(resp.Body).Decode(&targetsData)
	if len(targetsData["targets"]) != 1 || targetsData["targets"][0] != "192.168.1.50" {
		t.Errorf("неверный список целей: %v", targetsData)
	}

	// Тест GET /api/monitor/status
	reqStatus := httptest.NewRequest(http.MethodGet, "/api/monitor/status", nil)
	wStatus := httptest.NewRecorder()
	srv.handleGetStatus(wStatus, reqStatus)
	if wStatus.Result().StatusCode != http.StatusOK {
		t.Errorf("ошибка /api/monitor/status: %d", wStatus.Result().StatusCode)
	}

	// Тест GET /api/history
	reqHist := httptest.NewRequest(http.MethodGet, "/api/history?target=192.168.1.50&from=1h&to=now", nil)
	wHist := httptest.NewRecorder()
	srv.handleGetHistory(wHist, reqHist)
	if wHist.Result().StatusCode != http.StatusOK {
		t.Errorf("ошибка /api/history: %d", wHist.Result().StatusCode)
	}
}
