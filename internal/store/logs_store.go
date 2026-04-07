package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/pbsladek/knotical/internal/model"
)

type LogFilter struct {
	Conversation       string
	LatestConversation bool
	Model              string
	Search             string
	Latest             bool
	IDGT               string
	IDGTE              string
	Limit              int
}

type LogStore struct {
	Path string
	db   *sql.DB
}

func NewLogStore(path string) *LogStore {
	return &LogStore{Path: path}
}

func (s *LogStore) Open() error {
	_, err := s.dbHandle()
	return err
}

func (s *LogStore) Insert(entry model.LogEntry) error {
	db, err := s.dbHandle()
	if err != nil {
		return err
	}
	if entry.ID == "" {
		entry.ID = randomID()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	_, err = tx.Exec(`
INSERT INTO logs
    (id, conversation, model, provider, system_prompt, schema_json, fragments_json, reduction_json, prompt, response, input_tokens, output_tokens, duration_ms, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID,
		entry.Conversation,
		entry.Model,
		entry.Provider,
		entry.SystemPrompt,
		entry.SchemaJSON,
		entry.FragmentsJSON,
		entry.ReductionJSON,
		entry.Prompt,
		entry.Response,
		entry.InputTokens,
		entry.OutputTokens,
		entry.DurationMS,
		entry.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO logs_fts (id, prompt, response) VALUES (?, ?, ?)`, entry.ID, entry.Prompt, entry.Response); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *LogStore) Query(filter LogFilter) ([]model.LogEntry, error) {
	return s.queryWithFallback(filter)
}

func (s *LogStore) Get(id string) (*model.LogEntry, error) {
	db, err := s.dbHandle()
	if err != nil {
		return nil, err
	}
	row := db.QueryRow(`SELECT id, conversation, model, provider, system_prompt, schema_json, fragments_json, reduction_json, prompt, response, input_tokens, output_tokens, duration_ms, created_at
FROM logs WHERE id = ?`, id)
	entry, err := scanLogEntry(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func (s *LogStore) Count() (int, error) {
	db, err := s.dbHandle()
	if err != nil {
		return 0, err
	}
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM logs`).Scan(&count)
	return count, err
}

func (s *LogStore) CountConversations() (int, error) {
	db, err := s.dbHandle()
	if err != nil {
		return 0, err
	}
	var count int
	err = db.QueryRow(`SELECT COUNT(DISTINCT conversation) FROM logs WHERE conversation IS NOT NULL AND conversation != ''`).Scan(&count)
	return count, err
}

func (s *LogStore) Clear() error {
	db, err := s.dbHandle()
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.Exec(`DELETE FROM logs`); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM logs_fts`); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *LogStore) Backup(destination string) error {
	db, err := s.dbHandle()
	if err != nil {
		return err
	}
	if err := ensureSecureParent(destination); err != nil {
		return err
	}
	if _, err := os.Stat(destination); err == nil {
		if err := os.Remove(destination); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	escaped := strings.ReplaceAll(destination, `'`, `''`)
	_, err = db.Exec(fmt.Sprintf(`VACUUM INTO '%s'`, escaped))
	return err
}

func (s *LogStore) dbHandle() (*sql.DB, error) {
	if s.db != nil {
		return s.db, nil
	}
	if err := ensureSecureParent(s.Path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", s.Path)
	if err != nil {
		return nil, err
	}
	if err := applyLogSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := os.Chmod(s.Path, secureFileMode); err != nil && !os.IsNotExist(err) {
		_ = db.Close()
		return nil, err
	}
	s.db = db
	return s.db, nil
}

func randomID() string {
	var payload [16]byte
	if _, err := rand.Read(payload[:]); err != nil {
		return fmt.Sprintf("%020d-log", time.Now().UTC().UnixMicro())
	}
	return fmt.Sprintf("%020d-%s", time.Now().UTC().UnixMicro(), hex.EncodeToString(payload[:]))
}
