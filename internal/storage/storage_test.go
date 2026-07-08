package storage

import (
	"testing"
	"time"
)

func TestStorageAndOutages(t *testing.T) {
	st, err := NewStorage(":memory:")
	if err != nil {
		t.Fatalf("ошибка создания in-memory БД: %v", err)
	}
	defer st.Close()

	target := "192.168.1.100"
	baseTime := time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC)

	// Симулируем пинги: 3 успешных, 2 сбоя (обрыв связи), 2 успешных
	pings := []PingLog{
		{Timestamp: baseTime.Add(0 * time.Second), Target: target, Status: "success", RTT: 2 * time.Millisecond},
		{Timestamp: baseTime.Add(1 * time.Second), Target: target, Status: "success", RTT: 3 * time.Millisecond},
		{Timestamp: baseTime.Add(2 * time.Second), Target: target, Status: "success", RTT: 2 * time.Millisecond},
		{Timestamp: baseTime.Add(3 * time.Second), Target: target, Status: "timeout", ErrorMsg: "timeout"},
		{Timestamp: baseTime.Add(4 * time.Second), Target: target, Status: "timeout", ErrorMsg: "timeout"},
		{Timestamp: baseTime.Add(5 * time.Second), Target: target, Status: "success", RTT: 4 * time.Millisecond},
		{Timestamp: baseTime.Add(6 * time.Second), Target: target, Status: "success", RTT: 2 * time.Millisecond},
	}

	for _, p := range pings {
		if err := st.SavePing(p); err != nil {
			t.Fatalf("ошибка сохранения пинга: %v", err)
		}
	}

	// Проверяем GetPingsBetween
	logs, err := st.GetPingsBetween(target, baseTime, baseTime.Add(10*time.Second))
	if err != nil {
		t.Fatalf("ошибка GetPingsBetween: %v", err)
	}
	if len(logs) != 7 {
		t.Errorf("ожидалось 7 записей, получено %d", len(logs))
	}

	// Проверяем GetOutages
	outages, err := st.GetOutages(target, baseTime, baseTime.Add(10*time.Second))
	if err != nil {
		t.Fatalf("ошибка GetOutages: %v", err)
	}
	if len(outages) != 1 {
		t.Fatalf("ожидалось 1 событие обрыва связи, получено %d", len(outages))
	}
	outage := outages[0]
	if outage.LostCount != 2 {
		t.Errorf("ожидалось 2 потерянных пакета в событии, получено %d", outage.LostCount)
	}
	if outage.Duration != 1*time.Second {
		t.Errorf("ожидалось длительность 1s, получено %v", outage.Duration)
	}

	// Проверяем GetStats
	stats, err := st.GetStats(target, baseTime, baseTime.Add(10*time.Second))
	if err != nil {
		t.Fatalf("ошибка GetStats: %v", err)
	}
	if stats.TotalSent != 7 || stats.TotalReceived != 5 || stats.TotalLost != 2 {
		t.Errorf("неверная статистика: %+v", stats)
	}
}
