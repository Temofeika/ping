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

type targetLiveState struct {
	Target string
	Status string
	RTT    string
	Uptime string
	Sent   int
	Lost   int
}

func (d *DesktopApp) createMonitorTab() fyne.CanvasObject {
	targetEntry := widget.NewEntry()
	targetEntry.SetText("192.168.1.1, 8.8.8.8, 1.1.1.1")
	targetEntry.SetPlaceHolder("IP или хосты через запятую (например, 192.168.1.1, 8.8.8.8)")

	d.getActiveHosts = func() []string {
		rawTargets := strings.Split(targetEntry.Text, ",")
		var targets []string
		for _, rt := range rawTargets {
			tClean := strings.TrimSpace(rt)
			if tClean != "" {
				targets = append(targets, tClean)
			}
		}
		return targets
	}

	intervalEntry := widget.NewEntry()
	intervalEntry.SetText("1")
	intervalEntry.SetPlaceHolder("сек")

	statusLabel := widget.NewLabelWithStyle("⚫ Ожидание запуска...", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	activeNodesValue := widget.NewLabelWithStyle("0", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	avgUptimeValue := widget.NewLabelWithStyle("100.00%", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	totalSentValue := widget.NewLabelWithStyle("0", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	totalLostValue := widget.NewLabelWithStyle("0", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	var tableData [][]string
	tableData = append(tableData, []string{"№", "Целевой узел", "Статус соединения", "Текущий RTT", "Доступность %", "Отправлено", "Потеряно"})

	var statusTable *widget.Table
	statusTable = widget.NewTable(
		func() (int, int) { return len(tableData), 7 },
		func() fyne.CanvasObject { return widget.NewLabel("Длинный пример для расчета ячейки таблицы") },
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			label := cell.(*widget.Label)
			if id.Row < len(tableData) && id.Col < len(tableData[id.Row]) {
				label.SetText(tableData[id.Row][id.Col])
				if id.Row == 0 {
					label.TextStyle = fyne.TextStyle{Bold: true}
				} else {
					label.TextStyle = fyne.TextStyle{}
				}
			}
		},
	)
	statusTable.SetColumnWidth(0, 40)
	statusTable.SetColumnWidth(1, 180)
	statusTable.SetColumnWidth(2, 180)
	statusTable.SetColumnWidth(3, 110)
	statusTable.SetColumnWidth(4, 130)
	statusTable.SetColumnWidth(5, 100)
	statusTable.SetColumnWidth(6, 90)

	tableBox := container.NewBorder(
		widget.NewLabelWithStyle("Статус отслеживаемых устройств в реальном времени:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil, nil, nil, statusTable,
	)

	logArea := widget.NewMultiLineEntry()
	logArea.Disable()
	logArea.SetPlaceHolder("Здесь будут отображаться результаты пингования всех устройств в реальном времени...")
	logScroll := container.NewScroll(logArea)

	logBox := container.NewBorder(
		widget.NewLabelWithStyle("Общий хронологический лог пингов (последние 50 событий):", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil, nil, nil, logScroll,
	)

	splitContent := container.NewVSplit(tableBox, logBox)
	splitContent.SetOffset(0.55)

	var (
		mu             sync.Mutex
		cancelFunc     context.CancelFunc
		isRunning      bool
		totalSentCount int
		totalLostCount int
		liveStates     []targetLiveState
		logLines       []string
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
		statusLabel.SetText("⏹️ Мониторинг всех устройств остановлен")
	}

	startMonitor := func() {
		rawTargets := strings.Split(targetEntry.Text, ",")
		var targets []string
		for _, rt := range rawTargets {
			tClean := strings.TrimSpace(rt)
			if tClean != "" {
				targets = append(targets, tClean)
			}
		}

		if len(targets) == 0 {
			statusLabel.SetText("❌ Ошибка: укажите хотя бы один IP-адрес или хост")
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
		totalSentCount = 0
		totalLostCount = 0
		logLines = make([]string, 0, 50)
		liveStates = make([]targetLiveState, len(targets))

		tableData = [][]string{{"№", "Целевой узел", "Статус соединения", "Текущий RTT", "Доступность %", "Отправлено", "Потеряно"}}
		for idx, t := range targets {
			liveStates[idx] = targetLiveState{
				Target: t,
				Status: "⚫ Опрос...",
				RTT:    "...",
				Uptime: "100.00%",
				Sent:   0,
				Lost:   0,
			}
			tableData = append(tableData, []string{
				strconv.Itoa(idx + 1),
				t,
				"⚫ Опрос...",
				"...",
				"100.00%",
				"0",
				"0",
			})
		}
		statusTable.Refresh()

		ctx, cancel := context.WithCancel(context.Background())
		cancelFunc = cancel
		mu.Unlock()

		startBtn.Disable()
		stopBtn.Enable()
		targetEntry.Disable()
		intervalEntry.Disable()

		statusLabel.SetText(fmt.Sprintf("🟢 Мониторинг %d устройств(а) запущен...", len(targets)))
		activeNodesValue.SetText(strconv.Itoa(len(targets)))
		avgUptimeValue.SetText("100.00%")
		totalSentValue.SetText("0")
		totalLostValue.SetText("0")
		logArea.SetText("")

		interval := time.Duration(intervalSec) * time.Second
		timeout := 2 * time.Second
		if interval <= time.Second {
			timeout = 900 * time.Millisecond
		}

		for idx, host := range targets {
			go func(targetIdx int, targetHost string) {
				p := pinger.NewPinger()
				ticker := time.NewTicker(interval)
				defer ticker.Stop()

				doPing := func() {
					now := time.Now()
					res := p.Ping(targetHost, timeout)

					_ = d.st.SavePing(storage.PingLog{
						Timestamp: now,
						Target:    targetHost,
						Status:    res.Status,
						RTT:       res.RTT,
						ErrorMsg:  res.ErrorMsg,
					})

					mu.Lock()
					if !isRunning {
						mu.Unlock()
						return
					}
					ts := &liveStates[targetIdx]
					ts.Sent++
					totalSentCount++
					var rttStr, statusStr string

					if res.Status == "success" {
						rttMs := float64(res.RTT.Microseconds()) / 1000.0
						rttStr = fmt.Sprintf("%.1f мс", rttMs)
						ts.RTT = rttStr
						ts.Status = "🟢 ДОСТУПЕН"
						statusStr = "OK"
					} else {
						ts.Lost++
						totalLostCount++
						ts.RTT = "ТАЙМАУТ"
						ts.Status = "🔴 СБОЙ (" + res.ErrorMsg + ")"
						statusStr = "ПОТЕРЯ / " + res.ErrorMsg
					}

					uptime := float64(ts.Sent-ts.Lost) / float64(ts.Sent) * 100.0
					ts.Uptime = fmt.Sprintf("%.2f%%", uptime)

					if targetIdx+1 < len(tableData) {
						tableData[targetIdx+1][2] = ts.Status
						tableData[targetIdx+1][3] = ts.RTT
						tableData[targetIdx+1][4] = ts.Uptime
						tableData[targetIdx+1][5] = strconv.Itoa(ts.Sent)
						tableData[targetIdx+1][6] = strconv.Itoa(ts.Lost)
					}

					var sumUptime float64
					for _, s := range liveStates {
						if s.Sent > 0 {
							sumUptime += float64(s.Sent-s.Lost) / float64(s.Sent) * 100.0
						} else {
							sumUptime += 100.0
						}
					}
					avgUptime := sumUptime / float64(len(liveStates))

					activeNodesValue.SetText(strconv.Itoa(len(liveStates)))
					avgUptimeValue.SetText(fmt.Sprintf("%.2f%%", avgUptime))
					totalSentValue.SetText(strconv.Itoa(totalSentCount))
					totalLostValue.SetText(strconv.Itoa(totalLostCount))

					line := fmt.Sprintf("[%s] [%s]: rtt=%s | %s", now.Format("15:04:05"), targetHost, rttStr, statusStr)
					logLines = append([]string{line}, logLines...)
					if len(logLines) > 50 {
						logLines = logLines[:50]
					}
					logArea.SetText(strings.Join(logLines, "\n"))
					statusTable.Refresh()
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
			}(idx, host)
		}
	}

	startBtn = widget.NewButtonWithIcon("Запустить все", theme.MediaPlayIcon(), startMonitor)
	startBtn.Importance = widget.HighImportance
	stopBtn = widget.NewButtonWithIcon("Остановить все", theme.MediaStopIcon(), stopMonitor)
	stopBtn.Importance = widget.DangerImportance
	stopBtn.Disable()

	formGrid := container.NewGridWithColumns(3,
		container.New(layout.NewFormLayout(), widget.NewLabel("Цели (через запятую):"), targetEntry),
		container.New(layout.NewFormLayout(), widget.NewLabel("Интервал (сек):"), intervalEntry),
		container.NewGridWithColumns(2, startBtn, stopBtn),
	)

	metricsGrid := container.NewGridWithColumns(4,
		createMetricCard("Отслеживается узлов", activeNodesValue),
		createMetricCard("Средняя доступность", avgUptimeValue),
		createMetricCard("Всего отправлено", totalSentValue),
		createMetricCard("Всего потеряно", totalLostValue),
	)

	topBox := container.NewVBox(
		widget.NewLabelWithStyle("Панель мульти-мониторинга сетевых устройств", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		formGrid,
		widget.NewSeparator(),
		statusLabel,
		widget.NewSeparator(),
		metricsGrid,
		widget.NewSeparator(),
	)

	return container.NewBorder(topBox, nil, nil, nil, splitContent)
}

func createMetricCard(title string, valueLabel *widget.Label) fyne.CanvasObject {
	lbl := widget.NewLabelWithStyle(title, fyne.TextAlignCenter, fyne.TextStyle{})
	return container.NewVBox(lbl, valueLabel)
}
