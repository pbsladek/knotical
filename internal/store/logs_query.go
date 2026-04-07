package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/pbsladek/knotical/internal/model"
)

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
	query, args := buildLogQuery(filter, useFTS)
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

func buildLogQuery(filter LogFilter, useFTS bool) (string, []any) {
	query := `SELECT logs.id, logs.conversation, logs.model, logs.provider, logs.system_prompt, logs.schema_json, logs.fragments_json, logs.reduction_json, logs.prompt, logs.response, logs.input_tokens, logs.output_tokens, logs.duration_ms, logs.created_at
FROM logs`
	if useFTS && filter.Search != "" {
		query += ` JOIN logs_fts ON logs_fts.id = logs.id`
	}
	query += ` WHERE 1=1`
	args := []any{}
	query, args = appendConversationClause(query, args, filter)
	query, args = appendModelClause(query, args, filter)
	query, args = appendSearchClause(query, args, filter, useFTS)
	query, args = appendIDClauses(query, args, filter)
	if filter.Search != "" && !filter.Latest {
		args = append(args, filter.Search, filter.Search, filter.Search, filter.Search)
	}
	query += buildLogOrderBy(filter)
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	return query, args
}

func appendConversationClause(query string, args []any, filter LogFilter) (string, []any) {
	if filter.Conversation != "" {
		query += ` AND logs.conversation = ?`
		args = append(args, filter.Conversation)
		return query, args
	}
	if filter.LatestConversation {
		query += ` AND logs.conversation = (
SELECT conversation FROM logs
WHERE conversation IS NOT NULL AND conversation != ''
ORDER BY created_at DESC
LIMIT 1
)`
	}
	return query, args
}

func appendModelClause(query string, args []any, filter LogFilter) (string, []any) {
	if filter.Model != "" {
		query += ` AND logs.model = ?`
		args = append(args, filter.Model)
	}
	return query, args
}

func appendSearchClause(query string, args []any, filter LogFilter, useFTS bool) (string, []any) {
	if filter.Search == "" {
		return query, args
	}
	if useFTS {
		query += ` AND logs_fts MATCH ?`
		args = append(args, ftsQuery(filter.Search))
		return query, args
	}
	query += ` AND (logs.prompt LIKE ? OR logs.response LIKE ?)`
	pattern := "%" + filter.Search + "%"
	args = append(args, pattern, pattern)
	return query, args
}

func appendIDClauses(query string, args []any, filter LogFilter) (string, []any) {
	if filter.IDGT != "" {
		query += ` AND logs.id > ?`
		args = append(args, filter.IDGT)
	}
	if filter.IDGTE != "" {
		query += ` AND logs.id >= ?`
		args = append(args, filter.IDGTE)
	}
	return query, args
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

type logScanner interface {
	Scan(dest ...any) error
}

func scanLogEntry(scanner logScanner) (model.LogEntry, error) {
	var entry model.LogEntry
	var nullable logNullableFields
	var createdAt string

	err := scanner.Scan(
		&entry.ID,
		&nullable.conversation,
		&entry.Model,
		&entry.Provider,
		&nullable.systemPrompt,
		&nullable.schemaJSON,
		&nullable.fragmentsJSON,
		&nullable.reductionJSON,
		&entry.Prompt,
		&entry.Response,
		&nullable.inputTokens,
		&nullable.outputTokens,
		&nullable.durationMS,
		&createdAt,
	)
	if err != nil {
		return model.LogEntry{}, err
	}
	assignNullableLogFields(&entry, nullable)
	if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
		entry.CreatedAt = ts
	}
	return entry, nil
}

type logNullableFields struct {
	conversation  sql.NullString
	systemPrompt  sql.NullString
	schemaJSON    sql.NullString
	fragmentsJSON sql.NullString
	reductionJSON sql.NullString
	inputTokens   sql.NullInt64
	outputTokens  sql.NullInt64
	durationMS    sql.NullInt64
}

func assignNullableLogFields(entry *model.LogEntry, nullable logNullableFields) {
	if nullable.conversation.Valid {
		entry.Conversation = &nullable.conversation.String
	}
	if nullable.systemPrompt.Valid {
		entry.SystemPrompt = &nullable.systemPrompt.String
	}
	if nullable.schemaJSON.Valid {
		entry.SchemaJSON = &nullable.schemaJSON.String
	}
	if nullable.fragmentsJSON.Valid {
		entry.FragmentsJSON = &nullable.fragmentsJSON.String
	}
	if nullable.reductionJSON.Valid {
		entry.ReductionJSON = &nullable.reductionJSON.String
	}
	if nullable.inputTokens.Valid {
		entry.InputTokens = &nullable.inputTokens.Int64
	}
	if nullable.outputTokens.Valid {
		entry.OutputTokens = &nullable.outputTokens.Int64
	}
	if nullable.durationMS.Valid {
		entry.DurationMS = &nullable.durationMS.Int64
	}
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

func tableCount(db *sql.DB, table string) (int, error) {
	var count int
	err := db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s`, table)).Scan(&count)
	return count, err
}
