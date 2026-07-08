package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Temofeika/ping/internal/gui"
	"github.com/Temofeika/ping/internal/storage"
	"github.com/Temofeika/ping/internal/ui"
)

func main() {
	// Если программа запущена без аргументов (например, двойным кликом в Windows Explorer),
	// запускаем нативное десктопное графическое окно приложения (Fyne GUI)
	if len(os.Args) == 1 {
		runGuiCommand(nil)
		return
	}

	subcommand := os.Args[1]
	switch strings.ToLower(subcommand) {
	case "gui":
		runGuiCommand(os.Args[2:])
	case "menu", "interactive":
		runMenuCommand()
	case "monitor":
		runMonitorCommand(os.Args[2:])
	case "check":
		runCheckCommand(os.Args[2:])
	case "stats":
		runStatsCommand(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		// Если передан флаг без подкоманды (например ping-monitor.exe --target 192.168.1.1),
		// пытаемся обработать как команду monitor
		if strings.HasPrefix(subcommand, "-") {
			runMonitorCommand(os.Args[1:])
		} else {
			fmt.Printf("Неизвестная команда: %s\n\n", subcommand)
			printUsage()
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Println("Использование консольной утилиты Ping Monitor:")
	fmt.Println("  ping-monitor.exe                           # Запуск нативного десктопного окна (двойной клик)")
	fmt.Println("  ping-monitor.exe gui [флаги]               # Запуск десктопного GUI с выбором файла БД")
	fmt.Println("  ping-monitor.exe menu                      # Запуск текстового интерактивного меню в консоли")
	fmt.Println("  ping-monitor.exe monitor [флаги]           # Непрерывный мониторинг пинга")
	fmt.Println("  ping-monitor.exe check [флаги]             # Проверка обрывов связи за период времени")
	fmt.Println("  ping-monitor.exe stats [флаги]             # Сводная статистика доступности")
	fmt.Println("\nФлаги команды gui:")
	fmt.Println("  --db <путь>           Путь к базе данных SQLite (по умолчанию: ping_history.db)")
	fmt.Println("\nФлаги команды monitor:")
	fmt.Println("  --target <IP/Хост>    Узел для мониторинга (по умолчанию: 192.168.1.1)")
	fmt.Println("  --interval <сек>      Интервал опроса в секундах (по умолчанию: 1)")
	fmt.Println("  --db <путь>           Путь к базе данных SQLite (по умолчанию: ping_history.db)")
	fmt.Println("\nФлаги команды check:")
	fmt.Println("  --target <IP/Хост>    Узел для анализа истории (по умолчанию: 192.168.1.1)")
	fmt.Println("  --from <время>        Начало периода (ГГГГ-ММ-ДД ЧЧ:ММ:СС или '1h', '24h', '7d', по умолчанию '24h')")
	fmt.Println("  --to <время>          Конец периода (ГГГГ-ММ-ДД ЧЧ:ММ:СС или 'now', по умолчанию 'now')")
	fmt.Println("  --db <путь>           Путь к базе данных SQLite")
	fmt.Println("\nПример использования:")
	fmt.Println("  ping-monitor.exe monitor --target 8.8.8.8 --interval 1")
	fmt.Println("  ping-monitor.exe check --target 8.8.8.8 --from \"2026-07-08 10:00:00\" --to \"2026-07-08 12:00:00\"")
}

func runMenuCommand() {
	st, err := storage.NewStorage("ping_history.db")
	if err != nil {
		fmt.Printf("Ошибка открытия базы данных: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()
	ui.RunInteractive(st)
}

func runGuiCommand(args []string) {
	fs := flag.NewFlagSet("gui", flag.ExitOnError)
	dbPath := fs.String("db", "ping_history.db", "Путь к файлу базы данных SQLite")
	_ = fs.Parse(args)

	st, err := storage.NewStorage(*dbPath)
	if err != nil {
		fmt.Printf("Ошибка подключения к БД %s: %v\n", *dbPath, err)
		os.Exit(1)
	}
	defer st.Close()

	gui.RunDesktopApp(st)
}

func runMonitorCommand(args []string) {
	fs := flag.NewFlagSet("monitor", flag.ExitOnError)
	target := fs.String("target", "192.168.1.1", "IP-адрес или имя хоста для мониторинга")
	intervalSec := fs.Int("interval", 1, "Интервал опроса в секундах")
	dbPath := fs.String("db", "ping_history.db", "Путь к файлу базы данных SQLite")
	_ = fs.Parse(args)

	st, err := storage.NewStorage(*dbPath)
	if err != nil {
		fmt.Printf("Ошибка подключения к БД %s: %v\n", *dbPath, err)
		os.Exit(1)
	}
	defer st.Close()

	interval := time.Duration(*intervalSec) * time.Second
	timeout := 2 * time.Second
	if interval <= time.Second {
		timeout = 900 * time.Millisecond
	}

	ui.RunMonitor(context.Background(), *target, interval, timeout, st)
}

func runCheckCommand(args []string) {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	target := fs.String("target", "192.168.1.1", "IP-адрес или имя хоста")
	fromStr := fs.String("from", "24h", "Время начала или интервал (например '1h', '24h', '7d' или '2026-07-08 10:00:00')")
	toStr := fs.String("to", "now", "Время окончания (например 'now' или '2026-07-08 12:00:00')")
	dbPath := fs.String("db", "ping_history.db", "Путь к файлу базы данных SQLite")
	_ = fs.Parse(args)

	st, err := storage.NewStorage(*dbPath)
	if err != nil {
		fmt.Printf("Ошибка подключения к БД %s: %v\n", *dbPath, err)
		os.Exit(1)
	}
	defer st.Close()

	now := time.Now()
	var start, end time.Time

	if strings.EqualFold(*toStr, "now") {
		end = now
	} else {
		parsedEnd, err := time.ParseInLocation("2006-01-02 15:04:05", *toStr, time.Local)
		if err != nil {
			fmt.Printf("Ошибка парсинга параметра --to: %v. Используйте формат ГГГГ-ММ-ДД ЧЧ:ММ:СС или 'now'\n", err)
			os.Exit(1)
		}
		end = parsedEnd
	}

	switch strings.ToLower(*fromStr) {
	case "1h":
		start = end.Add(-1 * time.Hour)
	case "24h":
		start = end.Add(-24 * time.Hour)
	case "7d":
		start = end.Add(-7 * 24 * time.Hour)
	default:
		if strings.HasSuffix(*fromStr, "h") {
			if hours, err := strconv.Atoi(strings.TrimSuffix(*fromStr, "h")); err == nil {
				start = end.Add(-time.Duration(hours) * time.Hour)
				break
			}
		}
		parsedStart, err := time.ParseInLocation("2006-01-02 15:04:05", *fromStr, time.Local)
		if err != nil {
			fmt.Printf("Ошибка парсинга параметра --from: %v. Используйте формат ГГГГ-ММ-ДД ЧЧ:ММ:СС или '24h'\n", err)
			os.Exit(1)
		}
		start = parsedStart
	}

	stats, err := st.GetStats(*target, start, end)
	if err != nil {
		fmt.Printf("Ошибка получения статистики: %v\n", err)
		os.Exit(1)
	}
	outages, err := st.GetOutages(*target, start, end)
	if err != nil {
		fmt.Printf("Ошибка получения истории сбоев: %v\n", err)
		os.Exit(1)
	}

	ui.PrintReport(stats, outages, start, end)
}

func runStatsCommand(args []string) {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	target := fs.String("target", "192.168.1.1", "IP-адрес или имя хоста")
	lastStr := fs.String("last", "24h", "За какой период показать статистику (например '1h', '24h', '7d')")
	dbPath := fs.String("db", "ping_history.db", "Путь к файлу базы данных SQLite")
	_ = fs.Parse(args)

	st, err := storage.NewStorage(*dbPath)
	if err != nil {
		fmt.Printf("Ошибка подключения к БД %s: %v\n", *dbPath, err)
		os.Exit(1)
	}
	defer st.Close()

	now := time.Now()
	var dur time.Duration = 24 * time.Hour
	if strings.HasSuffix(*lastStr, "h") {
		if hours, err := strconv.Atoi(strings.TrimSuffix(*lastStr, "h")); err == nil {
			dur = time.Duration(hours) * time.Hour
		}
	} else if strings.HasSuffix(*lastStr, "d") {
		if days, err := strconv.Atoi(strings.TrimSuffix(*lastStr, "d")); err == nil {
			dur = time.Duration(days) * 24 * time.Hour
		}
	}

	start := now.Add(-dur)
	stats, err := st.GetStats(*target, start, now)
	if err != nil {
		fmt.Printf("Ошибка получения статистики: %v\n", err)
		os.Exit(1)
	}
	outages, err := st.GetOutages(*target, start, now)
	if err != nil {
		fmt.Printf("Ошибка получения истории сбоев: %v\n", err)
		os.Exit(1)
	}

	ui.PrintReport(stats, outages, start, now)
}
