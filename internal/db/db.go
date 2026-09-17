// Package db handles Postgres persistence for analysis reports so repeated
// requests for the same repo can be served from cache.
package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"archaeologist/internal/analyzer"
)

type Store struct {
	conn *sql.DB
}

// Open connects to Postgres and ensures the schema exists.
func Open(dsn string) (*Store, error) {
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	s := &Store{conn: conn}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.conn.Exec(`
		CREATE TABLE IF NOT EXISTS reports (
			id          SERIAL PRIMARY KEY,
			repo_path   TEXT NOT NULL,
			report_json JSONB NOT NULL,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS idx_reports_repo_path ON reports (repo_path);
	`)
	return err
}

// SaveReport persists a report and returns its assigned ID.
func (s *Store) SaveReport(r *analyzer.Report) (int64, error) {
	body, err := json.Marshal(r)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.conn.QueryRow(
		`INSERT INTO reports (repo_path, report_json) VALUES ($1, $2) RETURNING id`,
		r.RepoPath, body,
	).Scan(&id)
	return id, err
}

// LatestReport returns the most recent cached report for a repo path, and
// whether it is still fresh (younger than maxAge).
func (s *Store) LatestReport(repoPath string, maxAge time.Duration) (*analyzer.Report, bool, error) {
	var body []byte
	var createdAt time.Time
	err := s.conn.QueryRow(
		`SELECT report_json, created_at FROM reports
		 WHERE repo_path = $1 ORDER BY created_at DESC LIMIT 1`,
		repoPath,
	).Scan(&body, &createdAt)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var r analyzer.Report
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, false, err
	}
	fresh := time.Since(createdAt) < maxAge
	return &r, fresh, nil
}

// History returns the N most recent repo paths that have been analyzed.
func (s *Store) History(limit int) ([]string, error) {
	rows, err := s.conn.Query(
		`SELECT DISTINCT repo_path FROM reports ORDER BY repo_path LIMIT $1`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
