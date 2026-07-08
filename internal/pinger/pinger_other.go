//go:build !windows

package pinger

import (
	"fmt"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

type probingPinger struct{}

func newPlatformPinger() Pinger {
	return &probingPinger{}
}

func (p *probingPinger) Ping(target string, timeout time.Duration) PingResult {
	res := PingResult{
		Target: target,
		Status: "error",
	}

	pinger, err := probing.NewPinger(target)
	if err != nil {
		res.ErrorMsg = fmt.Sprintf("ошибка инициализации пингера: %v", err)
		return res
	}

	pinger.Count = 1
	pinger.Timeout = timeout
	pinger.SetPrivileged(true) // Используем сырые сокеты по умолчанию на Linux/macOS (или fallback на UDP)

	err = pinger.Run()
	if err != nil {
		// Если сырой сокет не удался (нет root), пробуем неправлигерованный UDP
		pinger.SetPrivileged(false)
		err = pinger.Run()
		if err != nil {
			res.ErrorMsg = fmt.Sprintf("ошибка отправки ICMP: %v", err)
			return res
		}
	}

	stats := pinger.Statistics()
	if stats.IPAddr != nil {
		res.IP = stats.IPAddr.String()
	}

	if stats.PacketsRecv == 0 {
		res.Status = "timeout"
		res.ErrorMsg = "превышено время ожидания ответа (таймаут)"
		return res
	}

	res.Status = "success"
	res.RTT = stats.AvgRtt
	return res
}
