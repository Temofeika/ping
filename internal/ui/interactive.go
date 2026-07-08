package ui

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Temofeika/ping/internal/storage"
)

// RunInteractive запускает текстовое консольное меню для удобства пользователей, открывающих .exe в проводнике
func RunInteractive(st *storage.Storage) {
	reader := bufio.NewReader(os.Stdin)

	for {
		printHeader()
		fmt.Println(" Выберите действие:")
		fmt.Println("   1. Запустить непрерывный мониторинг пинга до устройства")
		fmt.Println("   2. Проверить историю обрывов связи за промежуток времени")
		fmt.Println("   3. Показать общую статистику по всем сохраненным устройствам")
		fmt.Println("   0. Выход из программы")
		fmt.Println(strings.Repeat("-", 60))
		fmt.Print(" Ваш выбор (0-3): ")

		choiceStr, _ := reader.ReadString('\n')
		choiceStr = strings.TrimSpace(choiceStr)

		switch choiceStr {
		case "1":
			interactiveMonitor(reader, st)
		case "2":
			interactiveCheck(reader, st)
		case "3":
			interactiveStats(st)
		case "0", "q", "exit":
			fmt.Println("\nЗавершение работы программы. Всего доброго!")
			return
		default:
			fmt.Println(colorRed + "\n[ОШИБКА] Неверный ввод, выберите пункт от 0 до 3." + colorReset)
			time.Sleep(1 * time.Second)
		}
	}
}

func printHeader() {
	fmt.Println()
	fmt.Println(colorBold + colorCyan + strings.Repeat("=", 60) + colorReset)
	fmt.Printf("%s         МОНИТОРИНГ НЕПРЕРЫВНОСТИ ПИНГА В ЛОКАЛЬНОЙ СЕТИ%s\n", colorBold, colorReset)
	fmt.Println(colorBold + colorCyan + strings.Repeat("=", 60) + colorReset)
}

func interactiveMonitor(reader *bufio.Reader, st *storage.Storage) {
	fmt.Print("\nВведите IP-адрес или хост для мониторинга (по умолчанию 192.168.1.1): ")
	target, _ := reader.ReadString('\n')
	target = strings.TrimSpace(target)
	if target == "" {
		target = "192.168.1.1"
	}

	fmt.Print("Введите интервал опроса в секундах (по умолчанию 1): ")
	intStr, _ := reader.ReadString('\n')
	intStr = strings.TrimSpace(intStr)
	intervalSec := 1
	if val, err := strconv.Atoi(intStr); err == nil && val > 0 {
		intervalSec = val
	}

	interval := time.Duration(intervalSec) * time.Second
	timeout := 2 * time.Second
	if interval <= time.Second {
		timeout = 900 * time.Millisecond
	}

	RunMonitor(context.Background(), target, interval, timeout, st)
	fmt.Println("\nНажмите Enter для возврата в главное меню...")
	_, _ = reader.ReadString('\n')
}

func interactiveCheck(reader *bufio.Reader, st *storage.Storage) {
	targets, err := st.GetAllTargets()
	if err != nil || len(targets) == 0 {
		fmt.Println(colorYellow + "\n[ВНИМАНИЕ] В базе данных пока нет сохраненных записей пингов. Сначала запустите мониторинг!" + colorReset)
		time.Sleep(2 * time.Second)
		return
	}

	fmt.Println("\nСохраненные цели в базе данных:")
	for idx, t := range targets {
		fmt.Printf("   %d. %s\n", idx+1, t)
	}
	fmt.Print("Введите номер цели или сам IP-адрес: ")
	targetStr, _ := reader.ReadString('\n')
	targetStr = strings.TrimSpace(targetStr)

	var target string
	if idx, err := strconv.Atoi(targetStr); err == nil && idx >= 1 && idx <= len(targets) {
		target = targets[idx-1]
	} else {
		target = targetStr
	}
	if target == "" {
		target = targets[0]
	}

	fmt.Println("\nВыберите промежуток времени для анализа обрывов связи:")
	fmt.Println("   1. Последний 1 час")
	fmt.Println("   2. Последние 24 часа")
	fmt.Println("   3. Последние 7 дней")
	fmt.Println("   4. Ввести точные даты и время начала и конца")
	fmt.Print("Ваш выбор (1-4, по умолчанию 2): ")

	periodStr, _ := reader.ReadString('\n')
	periodStr = strings.TrimSpace(periodStr)

	now := time.Now()
	var start, end time.Time

	switch periodStr {
	case "1":
		start = now.Add(-1 * time.Hour)
		end = now
	case "3":
		start = now.Add(-7 * 24 * time.Hour)
		end = now
	case "4":
		fmt.Print("Введите время начала (формат: ГГГГ-ММ-ДД ЧЧ:ММ:СС, например 2026-07-08 10:00:00): ")
		startStr, _ := reader.ReadString('\n')
		startStr = strings.TrimSpace(startStr)
		parsedStart, err := time.ParseInLocation("2006-01-02 15:04:05", startStr, time.Local)
		if err != nil {
			fmt.Println(colorRed + "[ОШИБКА] Неверный формат времени начала. Используется начало текущего дня." + colorReset)
			start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
		} else {
			start = parsedStart
		}

		fmt.Print("Введите время конца (формат: ГГГГ-ММ-ДД ЧЧ:ММ:СС или Enter для текущего момента): ")
		endStr, _ := reader.ReadString('\n')
		endStr = strings.TrimSpace(endStr)
		if endStr == "" {
			end = now
		} else {
			parsedEnd, err := time.ParseInLocation("2006-01-02 15:04:05", endStr, time.Local)
			if err != nil {
				end = now
			} else {
				end = parsedEnd
			}
		}
	default: // По умолчанию последние 24 часа
		start = now.Add(-24 * time.Hour)
		end = now
	}

	stats, err := st.GetStats(target, start, end)
	if err != nil {
		fmt.Printf(colorRed+"[ОШИБКА] Не удалось рассчитать статистику: %v%s\n", err, colorReset)
		return
	}
	outages, err := st.GetOutages(target, start, end)
	if err != nil {
		fmt.Printf(colorRed+"[ОШИБКА] Не удалось получить историю сбоев: %v%s\n", err, colorReset)
		return
	}

	PrintReport(stats, outages, start, end)
	fmt.Println("Нажмите Enter для возврата в главное меню...")
	_, _ = reader.ReadString('\n')
}

func interactiveStats(st *storage.Storage) {
	targets, err := st.GetAllTargets()
	if err != nil || len(targets) == 0 {
		fmt.Println(colorYellow + "\n[ВНИМАНИЕ] В базе данных пока нет сохраненных целей." + colorReset)
		time.Sleep(2 * time.Second)
		return
	}

	now := time.Now()
	start := now.Add(-24 * time.Hour)

	fmt.Println()
	fmt.Println(colorBold + colorCyan + strings.Repeat("=", 70) + colorReset)
	fmt.Printf("%s         СВОДНАЯ СТАТИСТИКА ЗА ПОСЛЕДНИЕ 24 ЧАСА%s\n", colorBold, colorReset)
	fmt.Println(colorBold + colorCyan + strings.Repeat("=", 70) + colorReset)
	fmt.Printf("%-20s | %-10s | %-10s | %-10s | %s\n", "Узел (Цель)", "Отправлено", "Потеряно", "Аптайм", "Сбоев связи")
	fmt.Println(strings.Repeat("-", 70))

	for _, target := range targets {
		stats, err := st.GetStats(target, start, now)
		if err != nil {
			continue
		}
		uptimeColor := colorGreen
		if stats.UptimePercent < 99.0 {
			uptimeColor = colorYellow
		}
		if stats.UptimePercent < 90.0 {
			uptimeColor = colorRed
		}
		fmt.Printf("%-20s | %-10d | %-10d | %s%-9.2f%%%s | %d\n",
			target, stats.TotalSent, stats.TotalLost, uptimeColor, stats.UptimePercent, colorReset, stats.OutageCount)
	}
	fmt.Println(colorBold + colorCyan + strings.Repeat("=", 70) + colorReset)
	fmt.Println("\nНажмите Enter для возврата в главное меню...")
	reader := bufio.NewReader(os.Stdin)
	_, _ = reader.ReadString('\n')
}
