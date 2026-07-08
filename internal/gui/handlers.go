package gui

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func sendError(w http.ResponseWriter, status int, msg string) {
	sendJSON(w, status, map[string]string{"error": msg})
}

func (s *GUIServer) handleGetTargets(w http.ResponseWriter, r *http.Request) {
	targets, err := s.st.GetAllTargets()
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, http.StatusOK, map[string]interface{}{"targets": targets})
}

type startRequest struct {
	Target   string `json:"target"`
	Interval int    `json:"interval"`
}

func (s *GUIServer) handleStartMonitor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendError(w, http.StatusMethodNotAllowed, "Только POST запросы")
		return
	}

	var req startRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Неверный формат JSON")
		return
	}

	req.Target = strings.TrimSpace(req.Target)
	if req.Target == "" {
		sendError(w, http.StatusBadRequest, "Поле target обязательно")
		return
	}

	if req.Interval < 1 {
		req.Interval = 1
	}

	s.StartMonitoring(req.Target, req.Interval)
	sendJSON(w, http.StatusOK, map[string]string{"status": "started", "target": req.Target})
}

func (s *GUIServer) handleStopMonitor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendError(w, http.StatusMethodNotAllowed, "Только POST запросы")
		return
	}
	s.StopMonitoring()
	sendJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

type statusResponse struct {
	IsRunning       bool    `json:"is_running"`
	Target          string  `json:"target"`
	IntervalSec     int     `json:"interval_sec"`
	LastRTTMs       float64 `json:"last_rtt_ms"`
	LastStatus      string  `json:"last_status"`
	LastError       string  `json:"last_error"`
	ConsecutiveLost int     `json:"consecutive_lost"`
	TotalSent       int     `json:"total_sent"`
	TotalLost       int     `json:"total_lost"`
	UptimePercent   float64 `json:"uptime_percent"`
	RTTHistory      []int64 `json:"rtt_history"`
}

func (s *GUIServer) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var uptime float64
	if s.totalSent > 0 {
		received := s.totalSent - s.totalLost
		uptime = float64(received) / float64(s.totalSent) * 100.0
	}

	var rttMs float64
	if s.lastRTT > 0 {
		rttMs = float64(s.lastRTT.Microseconds()) / 1000.0
	}

	histCopy := make([]int64, len(s.rttHistory))
	copy(histCopy, s.rttHistory)

	resp := statusResponse{
		IsRunning:       s.isRunning,
		Target:          s.activeTarget,
		IntervalSec:     int(s.interval.Seconds()),
		LastRTTMs:       rttMs,
		LastStatus:      s.lastStatus,
		LastError:       s.lastError,
		ConsecutiveLost: s.consecLost,
		TotalSent:       s.totalSent,
		TotalLost:       s.totalLost,
		UptimePercent:   uptime,
		RTTHistory:      histCopy,
	}

	sendJSON(w, http.StatusOK, resp)
}

func (s *GUIServer) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	if target == "" {
		sendError(w, http.StatusBadRequest, "Параметр target обязателен")
		return
	}

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	if fromStr == "" {
		fromStr = "24h"
	}
	if toStr == "" {
		toStr = "now"
	}

	now := time.Now()
	var start, end time.Time

	if strings.EqualFold(toStr, "now") {
		end = now
	} else {
		parsedEnd, err := time.ParseInLocation("2006-01-02 15:04:05", toStr, time.Local)
		if err != nil {
			parsedEnd, err = time.Parse("2006-01-02T15:04:05", toStr)
			if err != nil {
				sendError(w, http.StatusBadRequest, "Неверный формат параметра to")
				return
			}
		}
		end = parsedEnd
	}

	switch strings.ToLower(fromStr) {
	case "1h":
		start = end.Add(-1 * time.Hour)
	case "24h":
		start = end.Add(-24 * time.Hour)
	case "7d":
		start = end.Add(-7 * 24 * time.Hour)
	default:
		if strings.HasSuffix(fromStr, "h") {
			if hours, err := strconv.Atoi(strings.TrimSuffix(fromStr, "h")); err == nil {
				start = end.Add(-time.Duration(hours) * time.Hour)
				break
			}
		}
		parsedStart, err := time.ParseInLocation("2006-01-02 15:04:05", fromStr, time.Local)
		if err != nil {
			parsedStart, err = time.Parse("2006-01-02T15:04:05", fromStr)
			if err != nil {
				sendError(w, http.StatusBadRequest, "Неверный формат параметра from")
				return
			}
		}
		start = parsedStart
	}

	stats, err := s.st.GetStats(target, start, end)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	outages, err := s.st.GetOutages(target, start, end)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type outageJSON struct {
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
		Duration  string `json:"duration"`
		LostCount int    `json:"lost_count"`
		Reason    string `json:"reason"`
	}

	outagesJSON := make([]outageJSON, 0, len(outages))
	for _, o := range outages {
		outagesJSON = append(outagesJSON, outageJSON{
			StartTime: o.StartTime.Format("2006-01-02 15:04:05"),
			EndTime:   o.EndTime.Format("2006-01-02 15:04:05"),
			Duration:  o.Duration.Round(time.Second).String(),
			LostCount: o.LostCount,
			Reason:    o.LastErrorMsg,
		})
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"target":            target,
		"start":             start.Format("2006-01-02 15:04:05"),
		"end":               end.Format("2006-01-02 15:04:05"),
		"total_sent":        stats.TotalSent,
		"total_received":    stats.TotalReceived,
		"total_lost":        stats.TotalLost,
		"uptime_percent":    stats.UptimePercent,
		"avg_rtt_ms":        float64(stats.AvgRTT.Microseconds()) / 1000.0,
		"min_rtt_ms":        float64(stats.MinRTT.Microseconds()) / 1000.0,
		"max_rtt_ms":        float64(stats.MaxRTT.Microseconds()) / 1000.0,
		"outage_count":      len(outages),
		"total_outage_time": stats.TotalOutageTime.Round(time.Second).String(),
		"outages":           outagesJSON,
	})
}

func (s *GUIServer) handleGetSummary(w http.ResponseWriter, r *http.Request) {
	targets, err := s.st.GetAllTargets()
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	now := time.Now()
	start := now.Add(-24 * time.Hour)

	type targetSummary struct {
		Target        string  `json:"target"`
		TotalSent     int     `json:"total_sent"`
		TotalLost     int     `json:"total_lost"`
		UptimePercent float64 `json:"uptime_percent"`
		Outages       int     `json:"outages"`
		AvgRTTMs      float64 `json:"avg_rtt_ms"`
	}

	summaries := make([]targetSummary, 0, len(targets))
	for _, t := range targets {
		stats, err := s.st.GetStats(t, start, now)
		if err != nil {
			continue
		}
		summaries = append(summaries, targetSummary{
			Target:        t,
			TotalSent:     stats.TotalSent,
			TotalLost:     stats.TotalLost,
			UptimePercent: stats.UptimePercent,
			Outages:       stats.OutageCount,
			AvgRTTMs:      float64(stats.AvgRTT.Microseconds()) / 1000.0,
		})
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{"summaries": summaries})
}
