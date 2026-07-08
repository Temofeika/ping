package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

// Storage инкапсулирует работу с базой данных SQLite
type Storage struct {
	db *sql.DB
}

// PingLog представляет одну запись результата пинга
type PingLog struct {
	ID        int64
	Timestamp time.Time
	Target    string
	Status    string // "success", "timeout", "error"
	RTT       time.Duration
	ErrorMsg  string
}

// OutageEvent представляет сгруппированное событие обрыва связи (серию последовательных потерь пакетов)
type OutageEvent struct {
	Target       string
	StartTime    time.Time
	EndTime      time.Time
	Duration     time.Duration
	LostCount    int
	LastStatus   string
	LastErrorMsg string
}

// Stats представляет общую статистику мониторинга за выбранный период
type Stats struct {
	Target          string
	TotalSent       int
	TotalReceived   int
	TotalLost       int
	UptimePercent   float64
	AvgRTT          time.Duration
	MinRTT          time.Duration
	MaxRTT          time.Duration
	OutageCount     int
	TotalOutageTime time.Duration
}

// NewStorage создает и инициализирует подключение к SQLite базе данных
func NewStorage(dbPath string) (*Storage, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия базы данных sqlite: %w", err)
	}

	// Включаем режим WAL для увеличения производительности записи и чтения
	_, _ = db.Exec("PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL;")

	s := &Storage{db: db}
	if err := s.init(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ошибка инициализации схемы БД: %w", err)
	}

	return s, nil
}

// Close закрывает соединение с базой данных
func (s *Storage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// init создает таблицу pings и необходимые индексы, если они еще не существуют
func (s *Storage) init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS pings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		target TEXT NOT NULL,
		status TEXT NOT NULL,
		rtt_ns INTEGER NOT NULL,
		error_msg TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_pings_target_timestamp ON pings(target, timestamp);
	CREATE INDEX IF NOT EXISTS idx_pings_status ON pings(status);
	`
	_, err := s.db.Exec(schema)
	return err
}
