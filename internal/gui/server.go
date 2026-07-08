package gui

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/Temofeika/ping/internal/pinger"
	"github.com/Temofeika/ping/internal/storage"
)

//go:embed web/*
var webFS embed.FS

// GUIServer управляет HTTP-сервером дашборда и фоновыми задачами мониторинга
type GUIServer struct {
	st           *storage.Storage
	port         int
	mu           sync.RWMutex
	activeTarget string
	interval     time.Duration
	timeout      time.Duration
	cancelFunc   context.CancelFunc
	isRunning    bool

	// Статистика текущей сессии для отображения в реальном времени
	lastRTT    time.Duration
	lastStatus string
	lastError  string
	consecLost int
	totalSent  int
	totalLost  int
	rttHistory []int64 // История последних 60 RTT (в мс) для отрисовки графика
}

// NewServer создает новый экземпляр GUI сервера
func NewServer(st *storage.Storage, port int) *GUIServer {
	return &GUIServer{
		st:      st,
		port:    port,
		timeout: 2 * time.Second,
	}
}

// Start запускает HTTP сервер и при необходимости открывает браузер
func (s *GUIServer) Start(openBrowser bool) error {
	subFS, err := fs.Sub(webFS, "web")
	if err != nil {
		return fmt.Errorf("ошибка инициализации встроенных файлов веб-ресурсов: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(subFS)))

	// API маршруты
	mux.HandleFunc("/api/targets", s.handleGetTargets)
	mux.HandleFunc("/api/monitor/start", s.handleStartMonitor)
	mux.HandleFunc("/api/monitor/stop", s.handleStopMonitor)
	mux.HandleFunc("/api/monitor/status", s.handleGetStatus)
	mux.HandleFunc("/api/history", s.handleGetHistory)
	mux.HandleFunc("/api/stats/summary", s.handleGetSummary)

	addr := fmt.Sprintf("localhost:%d", s.port)
	url := fmt.Sprintf("http://%s", addr)

	fmt.Println()
	fmt.Println("\033[1m\033[36m============================================================\033[0m")
	fmt.Printf("\033[1m         PING MONITOR — ГРАФИЧЕСКИЙ ВЕБ-ИНТЕРФЕЙС (GUI)\033[0m\n")
	fmt.Println("\033[1m\033[36m============================================================\033[0m")
	fmt.Printf(" \033[32m🟢 Веб-сервер успешно запущен!\033[0m\n")
	fmt.Printf(" 🌐 Адрес дашборда : \033[1m%s\033[0m\n", url)
	fmt.Println("------------------------------------------------------------")
	fmt.Println(" Нажмите Ctrl+C в этом окне для остановки сервера и выхода.")
	fmt.Println("\033[1m\033[36m============================================================\033[0m")
	fmt.Println()

	if openBrowser {
		go func() {
			time.Sleep(500 * time.Millisecond)
			OpenBrowser(url)
		}()
	}

	return http.ListenAndServe(addr, mux)
}

// OpenBrowser открывает URL в браузере по умолчанию для текущей ОС
func OpenBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = exec.Command("xdg-open", url).Start()
	}
	if err != nil {
		fmt.Printf("Не удалось автоматически открыть браузер: %v. Пожалуйста, перейдите по ссылке %s вручную.\n", err, url)
	}
}

// StartMonitoring запускает фоновый процесс пингования целевого узла
func (s *GUIServer) StartMonitoring(target string, intervalSec int) {
	s.StopMonitoring()

	s.mu.Lock()
	s.isRunning = true
	s.activeTarget = target
	if intervalSec < 1 {
		intervalSec = 1
	}
	s.interval = time.Duration(intervalSec) * time.Second
	if s.interval <= time.Second {
		s.timeout = 900 * time.Millisecond
	} else {
		s.timeout = 2 * time.Second
	}
	s.consecLost = 0
	s.totalSent = 0
	s.totalLost = 0
	s.lastRTT = 0
	s.lastStatus = "idle"
	s.lastError = ""
	s.rttHistory = make([]int64, 0, 60)

	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel
	s.mu.Unlock()

	go func() {
		p := pinger.NewPinger()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		doPing := func() {
			now := time.Now()
			res := p.Ping(target, s.timeout)

			_ = s.st.SavePing(storage.PingLog{
				Timestamp: now,
				Target:    target,
				Status:    res.Status,
				RTT:       res.RTT,
				ErrorMsg:  res.ErrorMsg,
			})

			s.mu.Lock()
			defer s.mu.Unlock()
			if !s.isRunning {
				return
			}
			s.totalSent++
			s.lastStatus = res.Status
			s.lastError = res.ErrorMsg
			s.lastRTT = res.RTT

			var rttMs int64 = -1
			if res.Status == "success" {
				s.consecLost = 0
				rttMs = res.RTT.Milliseconds()
				if rttMs == 0 {
					// Если RTT < 1 мс, ставим 1 для отображения на графике
					rttMs = 1
				}
			} else {
				s.totalLost++
				s.consecLost++
			}

			s.rttHistory = append(s.rttHistory, rttMs)
			if len(s.rttHistory) > 60 {
				s.rttHistory = s.rttHistory[1:]
			}
		}

		doPing()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				doPing()
			}
		}
	}()
}

// StopMonitoring останавливает текущую сессию пингования
func (s *GUIServer) StopMonitoring() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isRunning && s.cancelFunc != nil {
		s.cancelFunc()
		s.cancelFunc = nil
		s.isRunning = false
	}
}
