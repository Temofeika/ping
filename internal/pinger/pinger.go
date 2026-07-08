package pinger

import (
	"time"
)

// PingResult описывает результат одной попытки эхо-запроса
type PingResult struct {
	Target   string
	IP       string
	Status   string // "success", "timeout", "error"
	RTT      time.Duration
	ErrorMsg string
}

// Pinger определяет интерфейс для сетевого мониторинга узла
type Pinger interface {
	Ping(target string, timeout time.Duration) PingResult
}

// NewPinger возвращает платформенную реализацию интерфейса Pinger
func NewPinger() Pinger {
	return newPlatformPinger()
}
