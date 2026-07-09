package gui

import (
	"fmt"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (d *DesktopApp) createOverviewTab() fyne.CanvasObject {
	var tableData [][]string
	tableData = append(tableData, []string{"№", "Целевой узел (IP/Хост)", "Отправлено", "Потеряно", "Доступность (Uptime)", "Сбоев", "Средний RTT"})

	var table *widget.Table
	table = widget.NewTable(
		func() (int, int) { return len(tableData), 7 },
		func() fyne.CanvasObject { return widget.NewLabel("Длинный пример для расчета ячейки") },
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
	table.SetColumnWidth(1, 200)
	table.SetColumnWidth(2, 110)
	table.SetColumnWidth(3, 100)
	table.SetColumnWidth(4, 160)
	table.SetColumnWidth(5, 80)
	table.SetColumnWidth(6, 120)

	refreshData := func() {
		targets, _ := d.st.GetAllTargets()
		tableData = [][]string{{"№", "Целевой узел (IP/Хост)", "Отправлено", "Потеряно", "Доступность (Uptime)", "Сбоев", "Средний RTT"}}

		now := time.Now()
		start := now.Add(-24 * time.Hour)

		for idx, t := range targets {
			stats, err := d.st.GetStats(t, start, now)
			if err != nil {
				continue
			}
			avgMs := float64(stats.AvgRTT.Microseconds()) / 1000.0
			tableData = append(tableData, []string{
				strconv.Itoa(idx + 1),
				t,
				strconv.Itoa(stats.TotalSent),
				strconv.Itoa(stats.TotalLost),
				fmt.Sprintf("%.2f%%", stats.UptimePercent),
				strconv.Itoa(stats.OutageCount),
				fmt.Sprintf("%.1f мс", avgMs),
			})
		}
		table.Refresh()
	}

	d.refreshOverview = refreshData

	refreshBtn := widget.NewButtonWithIcon("Обновить данные из БД", theme.ViewRefreshIcon(), refreshData)
	refreshBtn.Importance = widget.MediumImportance

	topBox := container.NewVBox(
		widget.NewLabelWithStyle("Общая статистика доступности сетевых устройств в локальной сети (за последние 24 часа)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(refreshBtn),
		widget.NewSeparator(),
	)

	go func() {
		time.Sleep(200 * time.Millisecond)
		refreshData()
	}()

	return container.NewBorder(topBox, nil, nil, nil, table)
}
