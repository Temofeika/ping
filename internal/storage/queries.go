package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// SavePing сохраняет запись мониторинга в SQLite
func (s *Storage) SavePing(log PingLog) error {
	query := `INSERT INTO pings (timestamp, target, status, rtt_ns, error_msg) VALUES (?, ?, ?, ?, ?)`
	rttNs := int64(log.RTT)
	if log.Status != "success" {
		rttNs = -1
	}
	_, err := s.db.Exec(query, log.Timestamp.UTC(), log.Target, log.Status, rttNs, log.ErrorMsg)
	if err != nil {
		return fmt.Errorf("ошибка сохранения записи пинга: %w", err)
	}
	return nil
}

// GetPingsBetween возвращает все записи пингов для целевого узла в заданном интервале времени
func (s *Storage) GetPingsBetween(target string, start, end time.Time) ([]PingLog, error) {
	query := `
		SELECT id, timestamp, target, status, rtt_ns, error_msg 
		FROM pings 
		WHERE target = ? AND timestamp >= ? AND timestamp <= ? 
		ORDER BY timestamp ASC`
	
	rows, err := s.db.Query(query, target, start.UTC(), end.UTC())
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса истории пингов: %w", err)
	}
	defer rows.Close()

	var logs []PingLog
	for rows.Next() {
		var l PingLog
		var rttNs int64
		var errStr sql.NullString
		if err := rows.Scan(&l.ID, &l.Timestamp, &l.Target, &l.Status, &rttNs, &errStr); err != nil {
			return nil, fmt.Errorf("ошибка чтения строки истории: %w", err)
		}
		l.Timestamp = l.Timestamp.Local()
		if rttNs >= 0 {
			l.RTT = time.Duration(rttNs)
		} else {
			l.RTT = -1
		}
		if errStr.Valid {
			l.ErrorMsg = errStr.String
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// GetOutages анализирует историю и группирует последовательные потери пакетов в события обрыва связи
func (s *Storage) GetOutages(target string, start, end time.Time) ([]OutageEvent, error) {
	logs, err := s.GetPingsBetween(target, start, end)
	if err != nil {
		return nil, err
	}

	var outages []OutageEvent
	var current *OutageEvent

	for _, log := range logs {
		if log.Status != "success" {
			if current == nil {
				current = &OutageEvent{
					Target:       log.Target,
					StartTime:    log.Timestamp,
					EndTime:      log.Timestamp,
					LostCount:    1,
					LastStatus:   log.Status,
					LastErrorMsg: log.ErrorMsg,
				}
			} else {
				current.EndTime = log.Timestamp
				current.LostCount++
				current.LastStatus = log.Status
				current.LastErrorMsg = log.ErrorMsg
			}
		} else {
			if current != nil {
				current.Duration = current.EndTime.Sub(current.StartTime)
				if current.Duration == 0 {
					current.Duration = time.Second // Минимальная длительность для 1 потерянного пакета
				}
				outages = append(outages, *current)
				current = nil
			}
		}
	}

	// Если период закончился во время обрыва связи
	if current != nil {
		current.Duration = current.EndTime.Sub(current.StartTime)
		if current.Duration == 0 {
			current.Duration = time.Second
		}
		outages = append(outages, *current)
	}

	return outages, nil
}

// GetStats рассчитывает общую статистику доступности узла за период
func (s *Storage) GetStats(target string, start, end time.Time) (Stats, error) {
	logs, err := s.GetPingsBetween(target, start, end)
	if err != nil {
		return Stats{}, err
	}

	stats := Stats{
		Target: target,
		MinRTT: -1,
	}

	if len(logs) == 0 {
		return stats, nil
	}

	var totalRTT time.Duration
	var successCount int

	for _, log := range logs {
		stats.TotalSent++
		if log.Status == "success" {
			stats.TotalReceived++
			successCount++
			totalRTT += log.RTT
			if stats.MinRTT == -1 || log.RTT < stats.MinRTT {
				stats.MinRTT = log.RTT
			}
			if log.RTT > stats.MaxRTT {
				stats.MaxRTT = log.RTT
			}
		} else {
			stats.TotalLost++
		}
	}

	if stats.TotalSent > 0 {
		stats.UptimePercent = float64(stats.TotalReceived) / float64(stats.TotalSent) * 100.0
	}
	if successCount > 0 {
		stats.AvgRTT = time.Duration(int64(totalRTT) / int64(successCount))
	}

	outages, err := s.GetOutages(target, start, end)
	if err == nil {
		stats.OutageCount = len(outages)
		for _, o := range outages {
			stats.TotalOutageTime += o.Duration
		}
	}

	return stats, nil
}

// GetAllTargets возвращает список всех уникальных целевых узлов в базе данных
func (s *Storage) GetAllTargets() ([]string, error) {
	query := `SELECT DISTINCT target FROM pings ORDER BY target ASC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения списка целей: %w", err)
	}
	defer rows.Close()

	var targets []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}
