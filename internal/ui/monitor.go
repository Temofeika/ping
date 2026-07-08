package ui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Temofeika/ping/internal/pinger"
	"github.com/Temofeika/ping/internal/storage"
)

// RunMonitor запускает непрерывный мониторинг пинга до узла с сохранением истории в базу данных
func RunMonitor(ctx context.Context, target string, interval, timeout time.Duration, st *storage.Storage) {
	fmt.Println()
	fmt.Println(colorBold + colorCyan + strings.Repeat("=", 70) + colorReset)
	fmt.Printf("%s         ЗАПУСК НЕПРЕРЫВНОГО МОНИТОРИНГА ПИНГА%s\n", colorBold, colorReset)
	fmt.Println(colorBold + colorCyan + strings.Repeat("=", 70) + colorReset)
	fmt.Printf(" Цель мониторинга : %s%s%s\n", colorBold, target, colorReset)
	fmt.Printf(" Интервал опроса  : %v\n", interval)
	fmt.Printf(" Таймаут ответа   : %v\n", timeout)
	fmt.Println(" Нажмите Ctrl+C для остановки и просмотра итоговой статистики.")
	fmt.Println(strings.Repeat("-", 70))

	p := pinger.NewPinger()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	var totalSent, totalLost, consecLost int
	var minRtt, maxRtt, sumRtt time.Duration
	sessionStart := time.Now()

	doPing := func() {
		totalSent++
		now := time.Now()
		res := p.Ping(target, timeout)

		logEntry := storage.PingLog{
			Timestamp: now,
			Target:    target,
			Status:    res.Status,
			RTT:       res.RTT,
			ErrorMsg:  res.ErrorMsg,
		}

		_ = st.SavePing(logEntry)

		timeStr := now.Format("15:04:05")
		if res.Status == "success" {
			consecLost = 0
			sumRtt += res.RTT
			if minRtt == 0 || res.RTT < minRtt {
				minRtt = res.RTT
			}
			if res.RTT > maxRtt {
				maxRtt = res.RTT
			}
			fmt.Printf("[%s] %s (%s): %srtt=%v%s | Статус: %sOK%s\n",
				timeStr, target, res.IP, colorGreen, res.RTT.Round(10*time.Microsecond), colorReset, colorGreen, colorReset)
		} else {
			totalLost++
			consecLost++
			fmt.Printf("[%s] %s : %sСБОЙ / ТАЙМАУТ (%s)%s | Потеряно подряд: %s%d%s\n",
				timeStr, target, colorRed, res.ErrorMsg, colorReset, colorBold+colorRed, consecLost, colorReset)
		}
	}

	// Выполняем первый пинг сразу
	doPing()

	for {
		select {
		case <-ctx.Done():
			printSessionSummary(target, sessionStart, time.Now(), totalSent, totalLost, minRtt, maxRtt, sumRtt)
			return
		case <-sigChan:
			printSessionSummary(target, sessionStart, time.Now(), totalSent, totalLost, minRtt, maxRtt, sumRtt)
			return
		case <-ticker.C:
			doPing()
		}
	}
}

func printSessionSummary(target string, start, end time.Time, sent, lost int, min, max, sum time.Duration) {
	fmt.Println()
	fmt.Println(colorBold + colorYellow + strings.Repeat("=", 70) + colorReset)
	fmt.Printf("%s         ИТОГИ СЕССИИ МОНИТОРИНГА: %s%s\n", colorBold, target, colorReset)
	fmt.Println(colorBold + colorYellow + strings.Repeat("=", 70) + colorReset)
	fmt.Printf(" Время работы : %v\n", end.Sub(start).Round(time.Second))
	fmt.Printf(" Отправлено   : %d\n", sent)
	fmt.Printf(" Потеряно     : %d\n", lost)
	if sent > 0 {
		received := sent - lost
		uptime := float64(received) / float64(sent) * 100.0
		fmt.Printf(" Аптайм сессии: %.2f%%\n", uptime)
		if received > 0 {
			avg := time.Duration(int64(sum) / int64(received))
			fmt.Printf(" RTT (мин/ср/макс) : %v / %v / %v\n", min, avg, max)
		}
	}
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()
}
