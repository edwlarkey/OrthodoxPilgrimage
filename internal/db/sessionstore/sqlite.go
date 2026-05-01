package sessionstore

import (
	"database/sql"
	"time"
)

// SQLiteStore represents the session store for SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// New returns a new SQLiteStore instance.
func New(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{db: db}
}

// Find returns the data for a given session token from the SQLiteStore instance.
func (s *SQLiteStore) Find(token string) (b []byte, found bool, err error) {
	row := s.db.QueryRow("SELECT data FROM sessions WHERE token = ? AND expiry > ?", token, float64(time.Now().UnixNano())/1e9)
	err = row.Scan(&b)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

// Commit adds a session token and data to the SQLiteStore instance with the
// given expiry time. If the session token already exists, then the data and
// expiry time are updated.
func (s *SQLiteStore) Commit(token string, b []byte, expiry time.Time) error {
	_, err := s.db.Exec("INSERT INTO sessions (token, data, expiry) VALUES (?, ?, ?) ON CONFLICT(token) DO UPDATE SET data = excluded.data, expiry = excluded.expiry", token, b, float64(expiry.UnixNano())/1e9)
	return err
}

// Delete removes a session token from the SQLiteStore instance.
func (s *SQLiteStore) Delete(token string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE token = ?", token)
	return err
}

// Cleanup removes expired sessions from the SQLiteStore instance.
func (s *SQLiteStore) Cleanup() error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE expiry < ?", float64(time.Now().UnixNano())/1e9)
	return err
}
