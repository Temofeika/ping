package gui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/Temofeika/ping/internal/pinger"
	"github.com/Temofeika/ping/internal/storage"
)

func (d *DesktopApp) createMonitorTab() fyne.CanvasObject {
	targetEntry := widget.NewEntry()
	targetEntry.SetText("192.168.1.1")
	targetEntry.SetPlaceHolder("IP или хост (например, 8.8.8.8)")

	intervalEntry := widget.NewEntry()
	intervalEntry.SetText("1")
	intervalEntry.SetPlaceHolder("сек")

	statusLabel := widget.NewLabelWithStyle("⚫ Ожидание запуска...", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	rttValue := widget.NewLabelWithStyle("0 мс", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	uptimeValue := widget.NewLabelWithStyle("100.00%", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	sentValue := widget.NewLabelWithStyle("0", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	lostValue := widget.NewLabelWithStyle("0", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	logArea := widget.NewMultiLineEntry()
	logArea.Disable()
	logArea.SetPlaceHolder("Здесь будут отображаться результаты пингования в реальном времени...")
	logScroll := container.NewScroll(logArea)
	logScroll.SetMinSize(fyne.NewSize(0, 300))

	var (
		mu         sync.Mutex
		cancelFunc context.CancelFunc
		isRunning  bool
		sentCount  int
		lostCount  int
		logLines   []string
	)

	var startBtn *widget.Button
	var stopBtn *widget.Button

	stopMonitor := func() {
		mu.Lock()
		if isRunning && cancelFunc != nil {
			cancelFunc()
			cancelFunc = nil
			isRunning = false
		}
		mu.Unlock()
		startBtn.Enable()
		stopBtn.Disable()
		targetEntry.Enable()
		intervalEntry.Enable()
		statusLabel.SetText("⏹️ Мониторинг остановлен")
	}

	startMonitor := func() {
		target := strings.TrimSpace(targetEntry.Text)
		if target == "" {
			statusLabel.SetText("❌ Ошибка: введите IP-адрес или хост")
			return
		}

		intervalSec, err := strconv.Atoi(strings.TrimSpace(intervalEntry.Text))
		if err != nil || intervalSec < 1 {
			intervalSec = 1
			intervalEntry.SetText("1")
		}

		stopMonitor()

		mu.Lock()
		isRunning = true
		sentCount = 0
		lostCount = 0
		logLines = make([]string, 0, 50)
		ctx, cancel := context.WithCancel(context.Background())
		cancelFunc = cancel
		mu.Unlock()

		startBtn.Disable()
		stopBtn.Enable()
		targetEntry.Disable()
		intervalEntry.Disable()

		statusLabel.SetText(fmt.Sprintf("🟢 Мониторинг узла %s запущен...", target))
		rttValue.SetText("...")
		uptimeValue.SetText("100.00%")
		sentValue.SetText("0")
		lostValue.SetText("0")
		logArea.SetText("")

		interval := time.Duration(intervalSec) * time.Second
		timeout := 2 * time.Second
		if interval <= time.Second {
			timeout = 900 * time.Millisecond
		}

		go func() {
			p := pinger.NewPinger()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			doPing := func() {
				now := time.Now()
				res := p.Ping(target, timeout)

				_ = d.st.SavePing(storage.PingLog{
					Timestamp: now,
					Target:    target,
					Status:    res.Status,
					RTT:       res.RTT,
					ErrorMsg:  res.ErrorMsg,
				})

				mu.Lock()
				if !isRunning {
					mu.Unlock()
					return
				}
				sentCount++
				var statusStr string
				var rttStr string

				if res.Status == "success" {
					rttMs := float64(res.RTT.Microseconds()) / 1000.0
					rttStr = fmt.Sprintf("%.1f мс", rttMs)
					rttValue.SetText(rttStr)
					statusLabel.SetText(fmt.Sprintf("🟢 Узел %s ДОСТУПЕН (RTT: %s)", target, rttStr))
					statusStr = "OK"
				} else {
					lostCount++
					rttValue.SetText("ТАЙМАУТ")
					statusLabel.SetText(fmt.Sprintf("🔴 СБОЙ СВЯЗИ с %s! (%s)", target, res.ErrorMsg))
					statusStr = "ПОТЕРЯ / " + res.ErrorMsg
				}

				sentValue.SetText(strconv.Itoa(sentCount))
				lostValue.SetText(strconv.Itoa(lostCount))

				uptime := float64(sentCount-lostCount) / float64(sentCount) * 100.0
				uptimeValue.SetText(fmt.Sprintf("%.2f%%", uptime))

				line := fmt.Sprintf("[%s] %s: rtt=%s | %s", now.Format("15:04:05"), target, rttStr, statusStr)
				logLines = append([]string{line}, logLines...)
				if len(logLines) > 50 {
					logLines = logLines[:50]
				}
				logArea.SetText(strings.Join(logLines, "\n"))
				mu.Unlock()
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

	startBtn = widget.NewButtonWithIcon("Запустить", theme.MediaPlayIcon(), startMonitor)
	startBtn.Importance = widget.HighImportance
	stopBtn = widget.NewButtonWithIcon("Остановить", theme.MediaStopIcon(), stopMonitor)
	stopBtn.Importance = widget.DangerImportance
	stopBtn.Disable()

	formGrid := container.NewGridWithColumns(3,
		container.New(layout.NewFormLayout(), widget.NewLabel("Цель (IP/Хост):"), targetEntry),
		container.New(layout.NewFormLayout(), widget.NewLabel("Интервал (сек):"), intervalEntry),
		container.NewGridWithColumns(2, startBtn, stopBtn),
	)

	metricsGrid := container.NewGridWithColumns(4,
		createMetricCard("Текущий RTT", rttValue),
		createMetricCard("Доступность (Uptime)", uptimeValue),
		createMetricCard("Отправлено", sentValue),
		createMetricCard("Потеряно", lostValue),
	)

	topBox := container.NewVBox(
		widget.NewLabelWithStyle("Управление мониторингом", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		formGrid,
		widget.NewSeparator(),
		statusLabel,
		widget.NewSeparator(),
		metricsGrid,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Лог пингов в реальном времени (последние 50 записей):", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)

	return container.NewBorder(topBox, nil, nil, nil, logScroll)
}

func createMetricCard(title string, valueLabel *widget.Label) fyne.CanvasObject {
	lbl := widget.NewLabelWithStyle(title, fyne.TextAlignCenter, fyne.TextStyle{})
	return container.NewVBox(lbl, valueLabel)
}
