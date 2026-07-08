package ui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Temofeika/ping/internal/pinger"
	"github.com/Temofeika/ping/internal/storage"
)

type targetSessionStats struct {
	sent       int
	lost       int
	consecLost int
	minRtt     time.Duration
	maxRtt     time.Duration
	sumRtt     time.Duration
}

// RunMonitor запускает непрерывный мониторинг пинга до одного или нескольких узлов через запятую
func RunMonitor(ctx context.Context, targetStr string, interval, timeout time.Duration, st *storage.Storage) {
	rawTargets := strings.Split(targetStr, ",")
	var targets []string
	for _, rt := range rawTargets {
		tClean := strings.TrimSpace(rt)
		if tClean != "" {
			targets = append(targets, tClean)
		}
	}
	if len(targets) == 0 {
		targets = []string{"192.168.1.1"}
	}

	fmt.Println()
	fmt.Println(colorBold + colorCyan + strings.Repeat("=", 75) + colorReset)
	fmt.Printf("%s           ЗАПУСК НЕПРЕРЫВНОГО МУЛЬТИ-МОНИТОРИНГА ПИНГА (%d узла/ов)%s\n", colorBold, len(targets), colorReset)
	fmt.Println(colorBold + colorCyan + strings.Repeat("=", 75) + colorReset)
	fmt.Printf(" Цели мониторинга : %s%s%s\n", colorBold, strings.Join(targets, ", "), colorReset)
	fmt.Printf(" Интервал опроса  : %v\n", interval)
	fmt.Printf(" Таймаут ответа   : %v\n", timeout)
	fmt.Println(" Нажмите Ctrl+C для остановки и просмотра итоговой сводной статистики.")
	fmt.Println(strings.Repeat("-", 75))

	p := pinger.NewPinger()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	var mu sync.Mutex
	statsMap := make(map[string]*targetSessionStats)
	for _, t := range targets {
		statsMap[t] = &targetSessionStats{}
	}

	sessionStart := time.Now()
	ctxMon, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for _, host := range targets {
		wg.Add(1)
		go func(targetHost string) {
			defer wg.Done()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			doPing := func() {
				now := time.Now()
				res := p.Ping(targetHost, timeout)

				_ = st.SavePing(storage.PingLog{
					Timestamp: now,
					Target:    targetHost,
					Status:    res.Status,
					RTT:       res.RTT,
					ErrorMsg:  res.ErrorMsg,
				})

				mu.Lock()
				ts := statsMap[targetHost]
				ts.sent++
				timeStr := now.Format("15:04:05")
				if res.Status == "success" {
					ts.consecLost = 0
					ts.sumRtt += res.RTT
					if ts.minRtt == 0 || res.RTT < ts.minRtt {
						ts.minRtt = res.RTT
					}
					if res.RTT > ts.maxRtt {
						ts.maxRtt = res.RTT
					}
					fmt.Printf("[%s] [%s] %s%srtt=%v%s | Статус: %sOK%s\n",
						timeStr, targetHost, colorBold, colorGreen, res.RTT.Round(10*time.Microsecond), colorReset, colorGreen, colorReset)
				} else {
					ts.lost++
					ts.consecLost++
					fmt.Printf("[%s] [%s] %sСБОЙ / ТАЙМАУТ (%s)%s | Потеряно подряд: %s%d%s\n",
						timeStr, targetHost, colorRed, res.ErrorMsg, colorReset, colorBold+colorRed, ts.consecLost, colorReset)
				}
				mu.Unlock()
			}

			doPing()

			for {
				select {
				case <-ctxMon.Done():
					return
				case <-ticker.C:
					doPing()
				}
			}
		}(host)
	}

	select {
	case <-ctx.Done():
	case <-sigChan:
	}

	cancel()
	wg.Wait()

	printMultiSessionSummary(targets, statsMap, sessionStart, time.Now())
}

func printMultiSessionSummary(targets []string, statsMap map[string]*targetSessionStats, start, end time.Time) {
	fmt.Println()
	fmt.Println(colorBold + colorYellow + strings.Repeat("=", 75) + colorReset)
	fmt.Printf("%s               ИТОГИ СВОДНОЙ СЕССИИ МОНИТОРИНГА (%s)\n%s", colorBold, end.Sub(start).Round(time.Second), colorReset)
	fmt.Println(colorBold + colorYellow + strings.Repeat("=", 75) + colorReset)

	for _, host := range targets {
		ts := statsMap[host]
		fmt.Printf(" %sУзел: %s%s\n", colorBold, host, colorReset)
		fmt.Printf("   Отправлено   : %d | Потеряно : %d\n", ts.sent, ts.lost)
		if ts.sent > 0 {
			received := ts.sent - ts.lost
			uptime := float64(received) / float64(ts.sent) * 100.0
			fmt.Printf("   Аптайм       : %.2f%%\n", uptime)
			if received > 0 {
				avg := time.Duration(int64(ts.sumRtt) / int64(received))
				fmt.Printf("   RTT (м/с/м)  : %v / %v / %v\n", ts.minRtt, avg, ts.maxRtt)
			}
		}
		fmt.Println(strings.Repeat("-", 75))
	}
	fmt.Println()
}
