package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"github.com/Temofeika/ping/internal/storage"
)

// DesktopApp инкапсулирует состояние и компоненты нативного окна приложения
type DesktopApp struct {
	fyneApp         fyne.App
	win             fyne.Window
	st              *storage.Storage
	getActiveHosts  func() []string
	refreshHistory  func()
	refreshOverview func()
}

// RunDesktopApp инициализирует и запускает нативное графическое окно Windows
func RunDesktopApp(st *storage.Storage) {
	a := app.NewWithID("com.temofeika.pingmonitor")
	w := a.NewWindow("Ping Monitor — Сетевой монитор доступности")
	w.Resize(fyne.NewSize(950, 650))
	w.SetMaster()

	d := &DesktopApp{
		fyneApp: a,
		win:     w,
		st:      st,
	}

	monitorTab := d.createMonitorTab()
	historyTab := d.createHistoryTab()
	overviewTab := d.createOverviewTab()

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("⚡ Live Monitor", theme.ViewRefreshIcon(), monitorTab),
		container.NewTabItemWithIcon("📜 Outage History", theme.HistoryIcon(), historyTab),
		container.NewTabItemWithIcon("📊 Targets Overview", theme.ListIcon(), overviewTab),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	tabs.OnSelected = func(item *container.TabItem) {
		if item.Text == "📜 Outage History" && d.refreshHistory != nil {
			d.refreshHistory()
		}
		if item.Text == "📊 Targets Overview" && d.refreshOverview != nil {
			d.refreshOverview()
		}
	}

	w.SetContent(tabs)
	w.ShowAndRun()
}
