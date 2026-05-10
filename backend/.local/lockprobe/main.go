package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type statusSnapshot struct {
	CurrentWaits int64 `json:"currentWaits"`
	LockTimeMs   int64 `json:"lockTimeMs"`
	LockWaits    int64 `json:"lockWaits"`
}

type probeSummary struct {
	Start              statusSnapshot `json:"start"`
	End                statusSnapshot `json:"end"`
	DeltaCurrentWaits  int64          `json:"deltaCurrentWaits"`
	DeltaLockTimeMs    int64          `json:"deltaLockTimeMs"`
	DeltaLockWaits     int64          `json:"deltaLockWaits"`
	MaxCurrentWaits    int64          `json:"maxCurrentWaits"`
	MaxDataLockWaits   int64          `json:"maxDataLockWaits"`
	DataLockWaitsReady bool           `json:"dataLockWaitsReady"`
	LockWaitsTable     string         `json:"lockWaitsTable,omitempty"`
	Samples            int            `json:"samples"`
	DurationMs         int64          `json:"durationMs"`
}

func main() {
	var (
		dsn      string
		duration time.Duration
		interval time.Duration
	)
	flag.StringVar(&dsn, "dsn", "", "mysql dsn")
	flag.DurationVar(&duration, "duration", 15*time.Second, "probe duration")
	flag.DurationVar(&interval, "interval", 100*time.Millisecond, "sample interval")
	flag.Parse()

	if dsn == "" {
		panic("dsn is required")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), duration+10*time.Second)
	defer cancel()

	start, err := readStatus(ctx, db)
	if err != nil {
		panic(err)
	}
	lockWaitsTable := detectLockWaitsTable(ctx, db)
	dataLockWaitsReady := lockWaitsTable != ""

	summary := probeSummary{
		Start:              start,
		MaxCurrentWaits:    start.CurrentWaits,
		DataLockWaitsReady: dataLockWaitsReady,
		LockWaitsTable:     lockWaitsTable,
		DurationMs:         duration.Milliseconds(),
	}

	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		time.Sleep(interval)
		summary.Samples++

		cur, err := readStatus(ctx, db)
		if err == nil && cur.CurrentWaits > summary.MaxCurrentWaits {
			summary.MaxCurrentWaits = cur.CurrentWaits
		}
		if dataLockWaitsReady {
			waits, err := readLockWaitRows(ctx, db, lockWaitsTable)
			if err == nil && waits > summary.MaxDataLockWaits {
				summary.MaxDataLockWaits = waits
			}
		}
	}

	end, err := readStatus(ctx, db)
	if err != nil {
		panic(err)
	}
	summary.End = end
	summary.DeltaCurrentWaits = end.CurrentWaits - start.CurrentWaits
	summary.DeltaLockTimeMs = end.LockTimeMs - start.LockTimeMs
	summary.DeltaLockWaits = end.LockWaits - start.LockWaits

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(summary); err != nil {
		panic(err)
	}
}

func readStatus(ctx context.Context, db *sql.DB) (statusSnapshot, error) {
	rows, err := db.QueryContext(ctx, `
SHOW GLOBAL STATUS
WHERE Variable_name IN (
  'Innodb_row_lock_current_waits',
  'Innodb_row_lock_time',
  'Innodb_row_lock_waits'
)`)
	if err != nil {
		return statusSnapshot{}, err
	}
	defer rows.Close()

	snapshot := statusSnapshot{}
	for rows.Next() {
		var name string
		var value int64
		if err := rows.Scan(&name, &value); err != nil {
			return statusSnapshot{}, err
		}
		switch name {
		case "Innodb_row_lock_current_waits":
			snapshot.CurrentWaits = value
		case "Innodb_row_lock_time":
			snapshot.LockTimeMs = value
		case "Innodb_row_lock_waits":
			snapshot.LockWaits = value
		}
	}
	return snapshot, rows.Err()
}

func detectLockWaitsTable(ctx context.Context, db *sql.DB) string {
	candidates := []struct {
		schema string
		table  string
	}{
		{schema: "performance_schema", table: "data_lock_waits"},
		{schema: "information_schema", table: "innodb_lock_waits"},
		{schema: "sys", table: "innodb_lock_waits"},
	}
	for _, candidate := range candidates {
		if hasTable(ctx, db, candidate.schema, candidate.table) {
			return candidate.schema + "." + candidate.table
		}
	}
	return ""
}

func hasTable(ctx context.Context, db *sql.DB, schema string, table string) bool {
	var count int
	err := db.QueryRowContext(ctx, `
SELECT COUNT(1)
FROM information_schema.tables
WHERE table_schema = ?
  AND table_name = ?`, schema, table).Scan(&count)
	return err == nil && count > 0
}

func readLockWaitRows(ctx context.Context, db *sql.DB, table string) (int64, error) {
	var count int64
	err := db.QueryRowContext(ctx, "SELECT COUNT(1) FROM "+table).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("read lock wait rows from %s failed: %w", table, err)
	}
	return count, nil
}
