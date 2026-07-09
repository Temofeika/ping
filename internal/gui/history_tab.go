package gui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (d *DesktopApp) createHistoryTab() fyne.CanvasObject {
	targets, _ := d.st.GetAllTargets()
	if len(targets) == 0 {
		targets = []string{"192.168.1.1", "8.8.8.8"}
	}

	targetSelect := widget.NewSelectEntry(targets)
	targetSelect.SetText(targets[0])
	targetSelect.SetPlaceHolder("Выберите или введите IP/хост")

	fromEntry := widget.NewEntry()
	fromEntry.SetText("24h")
	fromEntry.SetPlaceHolder("ГГГГ-ММ-ДД ЧЧ:ММ:СС или '24h'")

	toEntry := widget.NewEntry()
	toEntry.SetText("now")
	toEntry.SetPlaceHolder("ГГГГ-ММ-ДД ЧЧ:ММ:СС или 'now'")

	verdictLabel := widget.NewLabelWithStyle("Выберите цель и нажмите «Анализировать» для загрузки истории обрывов связи.", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	statsLabel := widget.NewLabelWithStyle("", fyne.TextAlignCenter, fyne.TextStyle{})

	refreshTargetsList := func() {
		targetMap := make(map[string]bool)
		dbTargets, _ := d.st.GetAllTargets()
		for _, t := range dbTargets {
			tClean := strings.TrimSpace(t)
			if tClean != "" {
				targetMap[tClean] = true
			}
		}
		if d.getActiveHosts != nil {
			for _, t := range d.getActiveHosts() {
				tClean := strings.TrimSpace(t)
				if tClean != "" {
					targetMap[tClean] = true
				}
			}
		}
		var allTargets []string
		for t := range targetMap {
			allTargets = append(allTargets, t)
		}
		sort.Strings(allTargets)
		if len(allTargets) == 0 {
			allTargets = []string{"192.168.1.1", "8.8.8.8"}
		}
		targetSelect.SetOptions(allTargets)
		if strings.TrimSpace(targetSelect.Text) == "" && len(allTargets) > 0 {
			targetSelect.SetText(allTargets[0])
		}
	}

	d.refreshHistory = refreshTargetsList

	var tableData [][]string
	tableData = append(tableData, []string{"№", "Начало сбоя", "Конец сбоя", "Длительность", "Потеряно", "Причина (Ошибка)"})

	table := widget.NewTable(
		func() (int, int) { return len(tableData), 6 },
		func() fyne.CanvasObject { return widget.NewLabel("Длинный пример текста для расчета ширины ячейки") },
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
	table.SetColumnWidth(0, 40)
	table.SetColumnWidth(1, 160)
	table.SetColumnWidth(2, 160)
	table.SetColumnWidth(3, 110)
	table.SetColumnWidth(4, 90)
	table.SetColumnWidth(5, 300)

	btn1h := widget.NewButton("1 час", func() { fromEntry.SetText("1h"); toEntry.SetText("now") })
	btn24h := widget.NewButton("24 часа", func() { fromEntry.SetText("24h"); toEntry.SetText("now") })
	btn7d := widget.NewButton("7 дней", func() { fromEntry.SetText("7d"); toEntry.SetText("now") })

	analyzeFunc := func() {
		refreshTargetsList()
		target := strings.TrimSpace(targetSelect.Text)
		if target == "" {
			verdictLabel.SetText("❌ Ошибка: укажите или выберите целевой узел")
			return
		}

		now := time.Now()
		var start, end time.Time

		toStr := strings.TrimSpace(toEntry.Text)
		if strings.EqualFold(toStr, "now") || toStr == "" {
			end = now
		} else {
			parsedEnd, err := time.ParseInLocation("2006-01-02 15:04:05", toStr, time.Local)
			if err != nil {
				parsedEnd, err = time.Parse("2006-01-02T15:04:05", toStr)
				if err != nil {
					verdictLabel.SetText("❌ Ошибка формата поля «По время» (используйте ГГГГ-ММ-ДД ЧЧ:ММ:СС или 'now')")
					return
				}
			}
			end = parsedEnd
		}

		fromStr := strings.TrimSpace(fromEntry.Text)
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
					verdictLabel.SetText("❌ Ошибка формата поля «С время» (используйте '24h' или ГГГГ-ММ-ДД ЧЧ:ММ:СС)")
					return
				}
			}
			start = parsedStart
		}

		stats, err := d.st.GetStats(target, start, end)
		if err != nil {
			verdictLabel.SetText(fmt.Sprintf("❌ Ошибка запроса статистики: %v", err))
			return
		}
		outages, err := d.st.GetOutages(target, start, end)
		if err != nil {
			verdictLabel.SetText(fmt.Sprintf("❌ Ошибка запроса сбоев: %v", err))
			return
		}

		if stats.TotalSent == 0 {
			verdictLabel.SetText(fmt.Sprintf("ℹ️ Нет сохраненных данных для узла %s за выбранный период", target))
			statsLabel.SetText("")
			tableData = [][]string{{"№", "Начало сбоя", "Конец сбоя", "Длительность", "Потеряно", "Причина (Ошибка)"}}
			table.Refresh()
			return
		}

		if len(outages) == 0 {
			verdictLabel.SetText(fmt.Sprintf("🟢 [ОТЛИЧНО] Связь с %s в данном периоде НЕ ТЕРЯЛАСЬ! Потерь пакетов нет.", target))
		} else {
			verdictLabel.SetText(fmt.Sprintf("🔴 [ВНИМАНИЕ] Обнаружено сбоев связи с %s: %d! Общее время простоя: %s", target, len(outages), stats.TotalOutageTime.Round(time.Second)))
		}

		avgMs := float64(stats.AvgRTT.Microseconds()) / 1000.0
		minMs := float64(stats.MinRTT.Microseconds()) / 1000.0
		maxMs := float64(stats.MaxRTT.Microseconds()) / 1000.0
		statsLabel.SetText(fmt.Sprintf("Отправлено: %d | Успешно: %d | Потеряно: %d | Доступность: %.2f%% | Средний RTT: %.1f мс (мин: %.1f, макс: %.1f)",
			stats.TotalSent, stats.TotalReceived, stats.TotalLost, stats.UptimePercent, avgMs, minMs, maxMs))

		tableData = [][]string{{"№", "Начало сбоя", "Конец сбоя", "Длительность", "Потеряно", "Причина (Ошибка)"}}
		for idx, o := range outages {
			tableData = append(tableData, []string{
				strconv.Itoa(idx + 1),
				o.StartTime.Format("2006-01-02 15:04:05"),
				o.EndTime.Format("2006-01-02 15:04:05"),
				o.Duration.Round(time.Second).String(),
				strconv.Itoa(o.LostCount),
				o.LastErrorMsg,
			})
		}
		table.Refresh()
	}

	analyzeBtn := widget.NewButtonWithIcon("Анализировать", theme.SearchIcon(), analyzeFunc)
	analyzeBtn.Importance = widget.HighImportance

	refreshTargetsBtn := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), refreshTargetsList)

	targetInputRow := container.NewBorder(nil, nil, nil, refreshTargetsBtn, targetSelect)

	filterBox := container.NewGridWithColumns(3,
		container.New(layout.NewFormLayout(), widget.NewLabel("Узел (IP/Хост):"), targetInputRow),
		container.New(layout.NewFormLayout(), widget.NewLabel("Быстрый период:"), container.NewGridWithColumns(3, btn1h, btn24h, btn7d)),
		analyzeBtn,
	)

	timeBox := container.NewGridWithColumns(2,
		container.New(layout.NewFormLayout(), widget.NewLabel("С (Начало):"), fromEntry),
		container.New(layout.NewFormLayout(), widget.NewLabel("По (Конец):"), toEntry),
	)

	topPanel := container.NewVBox(
		widget.NewLabelWithStyle("Параметры проверки обрывов связи", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		filterBox,
		timeBox,
		widget.NewSeparator(),
		verdictLabel,
		statsLabel,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Журнал зафиксированных отключений и сбоев:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)

	return container.NewBorder(topPanel, nil, nil, nil, table)
}
