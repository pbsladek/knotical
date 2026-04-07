package store

import "database/sql"

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
    reduction_json TEXT,
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
	if err := ensureLogColumn(db, "reduction_json", "TEXT"); err != nil {
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
	_, err = db.Exec(`ALTER TABLE logs ADD COLUMN ` + name + ` ` + typ)
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
