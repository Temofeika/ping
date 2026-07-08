package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/Temofeika/ping/internal/storage"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

// PrintReport выводит красивый отчет о состоянии связи и обрывах в терминал
func PrintReport(stats storage.Stats, outages []storage.OutageEvent, start, end time.Time) {
	fmt.Println()
	fmt.Println(colorBold + colorCyan + strings.Repeat("=", 70) + colorReset)
	fmt.Printf("%s         ОТЧЕТ О НЕПРЕРЫВНОСТИ ПИНГА И ДОСТУПНОСТИ УЗЛА%s\n", colorBold, colorReset)
	fmt.Println(colorBold + colorCyan + strings.Repeat("=", 70) + colorReset)
	fmt.Printf(" Цель (IP/Хост) : %s%s%s\n", colorBold, stats.Target, colorReset)
	fmt.Printf(" Период анализа : с %s по %s\n", start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"))
	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf(" Отправлено запросов : %d\n", stats.TotalSent)
	fmt.Printf(" Успешных ответов    : %d\n", stats.TotalReceived)
	fmt.Printf(" Потеряно пакетов    : %d\n", stats.TotalLost)

	uptimeColor := colorGreen
	if stats.UptimePercent < 99.0 {
		uptimeColor = colorYellow
	}
	if stats.UptimePercent < 90.0 {
		uptimeColor = colorRed
	}
	fmt.Printf(" Доступность (Uptime): %s%s%.2f%%%s\n", colorBold, uptimeColor, stats.UptimePercent, colorReset)
	if stats.AvgRTT > 0 {
		fmt.Printf(" Среднее время (RTT) : %v (мин: %v, макс: %v)\n", stats.AvgRTT, stats.MinRTT, stats.MaxRTT)
	}
	fmt.Println(colorBold + colorCyan + strings.Repeat("=", 70) + colorReset)

	if len(outages) == 0 {
		fmt.Printf("\n%s%s[ОТЛИЧНО] В указанном периоде связь НЕ ТЕРЯЛАСЬ! Потерь пакетов нет.%s\n\n", colorBold, colorGreen, colorReset)
		return
	}

	fmt.Printf("\n%s%s[ВНИМАНИЕ] Обнаружено сбоев связи: %d! Общее время простоя: %v%s\n\n",
		colorBold, colorRed, len(outages), stats.TotalOutageTime, colorReset)

	fmt.Printf("%-20s | %-20s | %-12s | %-8s | %s\n", "Начало сбоя", "Конец сбоя", "Длительность", "Пакеты", "Причина")
	fmt.Println(strings.Repeat("-", 75))

	for _, o := range outages {
		fmt.Printf("%-20s | %-20s | %-12s | %-8d | %s%s%s\n",
			o.StartTime.Format("2006-01-02 15:04:05"),
			o.EndTime.Format("2006-01-02 15:04:05"),
			o.Duration.Round(time.Second),
			o.LostCount,
			colorRed, o.LastErrorMsg, colorReset,
		)
	}
	fmt.Println()
}
