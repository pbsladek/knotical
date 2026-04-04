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
    (id, conversation, model, provider, system_prompt, schema_json, fragments_json, prompt, response, input_tokens, output_tokens, duration_ms, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID,
		entry.Conversation,
		entry.Model,
		entry.Provider,
		entry.SystemPrompt,
		entry.SchemaJSON,
		entry.FragmentsJSON,
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

func (s *LogStore) queryWithFallback(filter LogFilter) ([]model.LogEntry, error) {
	entries, err := s.query(filter, filter.Search != "")
	if err == nil {
		return entries, nil
	}
	if filter.Search == "" || !isFTSError(err) {
		return nil, err
	}
	return s.query(filter, false)
}

func (s *LogStore) query(filter LogFilter, useFTS bool) ([]model.LogEntry, error) {
	db, err := s.dbHandle()
	if err != nil {
		return nil, err
	}
	query := `SELECT logs.id, logs.conversation, logs.model, logs.provider, logs.system_prompt, logs.schema_json, logs.fragments_json, logs.prompt, logs.response, logs.input_tokens, logs.output_tokens, logs.duration_ms, logs.created_at
FROM logs`
	if useFTS && filter.Search != "" {
		query += ` JOIN logs_fts ON logs_fts.id = logs.id`
	}
	query += ` WHERE 1=1`
	args := []any{}
	if filter.Conversation != "" {
		query += ` AND logs.conversation = ?`
		args = append(args, filter.Conversation)
	} else if filter.LatestConversation {
		query += ` AND logs.conversation = (
SELECT conversation FROM logs
WHERE conversation IS NOT NULL AND conversation != ''
ORDER BY created_at DESC
LIMIT 1
)`
	}
	if filter.Model != "" {
		query += ` AND logs.model = ?`
		args = append(args, filter.Model)
	}
	if filter.Search != "" {
		if useFTS {
			query += ` AND logs_fts MATCH ?`
			args = append(args, ftsQuery(filter.Search))
		} else {
			query += ` AND (logs.prompt LIKE ? OR logs.response LIKE ?)`
			pattern := "%" + filter.Search + "%"
			args = append(args, pattern, pattern)
		}
	}
	if filter.IDGT != "" {
		query += ` AND logs.id > ?`
		args = append(args, filter.IDGT)
	}
	if filter.IDGTE != "" {
		query += ` AND logs.id >= ?`
		args = append(args, filter.IDGTE)
	}
	if filter.Search != "" && !filter.Latest {
		args = append(args, filter.Search, filter.Search, filter.Search, filter.Search)
	}
	query += buildLogOrderBy(filter)
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []model.LogEntry{}
	for rows.Next() {
		entry, err := scanLogEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func buildLogOrderBy(filter LogFilter) string {
	if filter.Search == "" || filter.Latest {
		return ` ORDER BY created_at DESC`
	}
	return ` ORDER BY
CASE
    WHEN lower(logs.prompt) = lower(?) OR lower(logs.response) = lower(?) THEN 0
    WHEN instr(lower(logs.prompt), lower(?)) > 0 THEN 1
    WHEN instr(lower(logs.response), lower(?)) > 0 THEN 2
    ELSE 3
END,
logs.created_at DESC`
}

func (s *LogStore) Get(id string) (*model.LogEntry, error) {
	db, err := s.dbHandle()
	if err != nil {
		return nil, err
	}
	row := db.QueryRow(`SELECT id, conversation, model, provider, system_prompt, schema_json, fragments_json, prompt, response, input_tokens, output_tokens, duration_ms, created_at
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

type logScanner interface {
	Scan(dest ...any) error
}

func scanLogEntry(scanner logScanner) (model.LogEntry, error) {
	var entry model.LogEntry
	var conversation sql.NullString
	var systemPrompt sql.NullString
	var schemaJSON sql.NullString
	var fragmentsJSON sql.NullString
	var inputTokens sql.NullInt64
	var outputTokens sql.NullInt64
	var durationMS sql.NullInt64
	var createdAt string

	err := scanner.Scan(
		&entry.ID,
		&conversation,
		&entry.Model,
		&entry.Provider,
		&systemPrompt,
		&schemaJSON,
		&fragmentsJSON,
		&entry.Prompt,
		&entry.Response,
		&inputTokens,
		&outputTokens,
		&durationMS,
		&createdAt,
	)
	if err != nil {
		return model.LogEntry{}, err
	}
	if conversation.Valid {
		entry.Conversation = &conversation.String
	}
	if systemPrompt.Valid {
		entry.SystemPrompt = &systemPrompt.String
	}
	if schemaJSON.Valid {
		entry.SchemaJSON = &schemaJSON.String
	}
	if fragmentsJSON.Valid {
		entry.FragmentsJSON = &fragmentsJSON.String
	}
	if inputTokens.Valid {
		entry.InputTokens = &inputTokens.Int64
	}
	if outputTokens.Valid {
		entry.OutputTokens = &outputTokens.Int64
	}
	if durationMS.Valid {
		entry.DurationMS = &durationMS.Int64
	}
	if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
		entry.CreatedAt = ts
	}
	return entry, nil
}

func applyLogSchema(db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS logs (
    id TEXT PRIMARY KEY,
    conversation TEXT,
    model TEXT NOT NULL,
    provider TEXT NOT NULL,
    system_prompt TEXT,
    schema_json TEXT,
    fragments_json TEXT,
    prompt TEXT NOT NULL,
    response TEXT NOT NULL,
    input_tokens INTEGER,
    output_tokens INTEGER,
    duration_ms INTEGER,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS logs_conversation ON logs (conversation);
CREATE INDEX IF NOT EXISTS logs_created_at ON logs (created_at DESC);
CREATE INDEX IF NOT EXISTS logs_model ON logs (model);`
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	if err := ensureLogColumn(db, "schema_json", "TEXT"); err != nil {
		return err
	}
	if err := ensureLogColumn(db, "fragments_json", "TEXT"); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS logs_fts USING fts4(id, prompt, response)`); err != nil {
		return err
	}
	return syncLogFTSIfNeeded(db)
}

func ensureLogColumn(db *sql.DB, name string, typ string) error {
	exists, err := logColumnExists(db, name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = db.Exec(fmt.Sprintf(`ALTER TABLE logs ADD COLUMN %s %s`, name, typ))
	return err
}

func logColumnExists(db *sql.DB, name string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(logs)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var columnName string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if columnName == name {
			return true, nil
		}
	}
	return false, rows.Err()
}

func isFTSError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "fts") || strings.Contains(msg, "match")
}

func ftsQuery(search string) string {
	parts := strings.Fields(strings.TrimSpace(search))
	if len(parts) == 0 {
		return ""
	}
	for i, part := range parts {
		part = strings.Trim(part, `"'`)
		if part == "" {
			continue
		}
		if !strings.HasSuffix(part, "*") {
			part += "*"
		}
		parts[i] = part
	}
	return strings.Join(parts, " ")
}

func syncLogFTSIfNeeded(db *sql.DB) error {
	logCount, err := tableCount(db, "logs")
	if err != nil {
		return err
	}
	ftsCount, err := tableCount(db, "logs_fts")
	if err != nil {
		return err
	}
	if logCount == ftsCount {
		return nil
	}
	if _, err := db.Exec(`DELETE FROM logs_fts`); err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO logs_fts (id, prompt, response) SELECT id, prompt, response FROM logs`)
	return err
}

func tableCount(db *sql.DB, table string) (int, error) {
	var count int
	err := db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s`, table)).Scan(&count)
	return count, err
}

func randomID() string {
	var payload [16]byte
	if _, err := rand.Read(payload[:]); err != nil {
		return fmt.Sprintf("%020d-log", time.Now().UTC().UnixMicro())
	}
	return fmt.Sprintf("%020d-%s", time.Now().UTC().UnixMicro(), hex.EncodeToString(payload[:]))
}
