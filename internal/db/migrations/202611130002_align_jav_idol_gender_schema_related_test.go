package migrations

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// buildPreGenderSchema creates the jav_idol schema as left by migration
// 202611130001 (gender column appended at the end with a single-quoted default)
// plus the child tables that reference jav_idol via FK (jav_idol_alias,
// jav_idol_map) and a parent video table (jav). This mirrors the production
// state immediately before 202611130002 runs.
func buildPreGenderSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	stmts := []string{
		`CREATE TABLE jav_idol (
			id integer PRIMARY KEY AUTOINCREMENT,
			name text,
			roman_name text,
			japanese_name text,
			chinese_name text,
			height_cm integer,
			birth_date datetime,
			bust integer,
			waist integer,
			hips integer,
			cup integer,
			created_at datetime,
			updated_at datetime,
			cover_jav_id integer,
			cover_crop_left real NOT NULL DEFAULT 0.53,
			gender text NOT NULL DEFAULT 'female'
		)`,
		`CREATE UNIQUE INDEX idx_jav_idol_name ON jav_idol(name)`,
		`CREATE INDEX idx_jav_idol_cover_jav_id ON jav_idol(cover_jav_id)`,
		`CREATE INDEX idx_jav_idol_gender ON jav_idol(gender)`,

		// Child table with FK -> jav_idol (OnDelete CASCADE)
		`CREATE TABLE jav_idol_alias (
			id integer PRIMARY KEY AUTOINCREMENT,
			jav_idol_id integer NOT NULL,
			alias text NOT NULL,
			created_at datetime,
			CONSTRAINT fk_jav_idol_alias_jav_idol FOREIGN KEY (jav_idol_id) REFERENCES jav_idol(id) ON UPDATE CASCADE ON DELETE CASCADE
		)`,
		`CREATE UNIQUE INDEX idx_jav_idol_alias_alias ON jav_idol_alias(alias)`,

		// Parent video table referenced by jav_idol_map
		`CREATE TABLE jav (
			id integer PRIMARY KEY AUTOINCREMENT,
			code text
		)`,

		// Join table with FK -> jav_idol (OnDelete CASCADE)
		`CREATE TABLE jav_idol_map (
			jav_id integer NOT NULL,
			jav_idol_id integer NOT NULL,
			created_at datetime,
			PRIMARY KEY (jav_id, jav_idol_id),
			CONSTRAINT fk_jav_idol_map_jav_idol FOREIGN KEY (jav_idol_id) REFERENCES jav_idol(id) ON UPDATE CASCADE ON DELETE CASCADE,
			CONSTRAINT fk_jav_idol_map_jav FOREIGN KEY (jav_id) REFERENCES jav(id) ON UPDATE CASCADE ON DELETE CASCADE
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("pre-schema: %v\nstmt: %s", err, s)
		}
	}

	// One female, one male, one more female idol.
	if _, err := db.Exec(`INSERT INTO jav_idol (id, name, gender) VALUES
		(1, '女优A', 'female'),
		(2, '男优B', 'male'),
		(3, '女优C', 'female')`); err != nil {
		t.Fatal(err)
	}
	// Aliases for idol 1 and idol 2.
	if _, err := db.Exec(`INSERT INTO jav_idol_alias (jav_idol_id, alias) VALUES
		(1, '别名A1'),
		(2, '别名B1')`); err != nil {
		t.Fatal(err)
	}
	// One video linked to both the female and the male idol.
	if _, err := db.Exec(`INSERT INTO jav (id, code) VALUES (10, 'ABC-123')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO jav_idol_map (jav_id, jav_idol_id) VALUES
		(10, 1),
		(10, 2)`); err != nil {
		t.Fatal(err)
	}
}

// openTempDB opens an on-disk sqlite DB with foreign keys OFF, mirroring the
// production migration path (db.Open sets PRAGMA foreign_keys=OFF before
// running migrations). On-disk (not :memory:) so sqlite_sequence persistence is
// exercised realistically.
func openTempDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=OFF"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func countRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}

// TestAlignJavIdolGenderSchemaPreservesRelatedData verifies, with a real
// foreign-key-equipped schema, that running 202611130001 + 202611130002 keeps
// every related row (aliases, video associations) and the original idol ids,
// and that the schema still validates once foreign keys are re-enabled.
func TestAlignJavIdolGenderSchemaPreservesRelatedData(t *testing.T) {
	db := openTempDB(t)
	buildPreGenderSchema(t, db)

	// Apply both migrations in production order (addJavIdolGender is a no-op
	// here since gender already exists; alignJavIdolGenderSchema rebuilds).
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := addJavIdolGender(context.Background(), tx); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := alignJavIdolGenderSchema(context.Background(), tx); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	// --- Data preservation assertions ---
	if n := countRows(t, db, `SELECT COUNT(*) FROM jav_idol`); n != 3 {
		t.Fatalf("jav_idol rows = %d, want 3", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM jav_idol_alias`); n != 2 {
		t.Fatalf("jav_idol_alias rows = %d, want 2 (aliases must survive)", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM jav_idol_map`); n != 2 {
		t.Fatalf("jav_idol_map rows = %d, want 2 (video associations must survive)", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM jav`); n != 1 {
		t.Fatalf("jav rows = %d, want 1", n)
	}

	// Actor IDs stay consistent (no renumbering on rebuild).
	var ids []int
	rows, err := db.Query(`SELECT id FROM jav_idol ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	wantIDs := []int{1, 2, 3}
	for i, id := range ids {
		if id != wantIDs[i] {
			t.Fatalf("jav_idol ids = %v, want %v (IDs must be preserved across migration)", ids, wantIDs)
		}
	}

	// Gender values preserved, including an explicit male and backfilled default.
	type row struct {
		id, gender, name string
	}
	checks := []row{{"1", "female", "女优A"}, {"2", "male", "男优B"}, {"3", "female", "女优C"}}
	for _, c := range checks {
		var name, gender string
		if err := db.QueryRow(`SELECT name, gender FROM jav_idol WHERE id = ?`, c.id).Scan(&name, &gender); err != nil {
			t.Fatal(err)
		}
		if name != c.name || gender != c.gender {
			t.Fatalf("row %s = (%q,%q), want (%q,%q)", c.id, name, gender, c.name, c.gender)
		}
	}

	// Child links still point at the correct idols.
	if n := countRows(t, db, `SELECT COUNT(*) FROM jav_idol_alias WHERE jav_idol_id = 1 AND alias = '别名A1'`); n != 1 {
		t.Fatalf("alias for idol 1 lost/changed")
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM jav_idol_alias WHERE jav_idol_id = 2 AND alias = '别名B1'`); n != 1 {
		t.Fatalf("alias for idol 2 lost/changed")
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM jav_idol_map WHERE jav_id = 10 AND jav_idol_id = 1`); n != 1 {
		t.Fatalf("video->female idol link lost")
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM jav_idol_map WHERE jav_id = 10 AND jav_idol_id = 2`); n != 1 {
		t.Fatalf("video->male idol link lost")
	}

	// Indexes / unique constraints preserved.
	idxRows, err := db.Query(`SELECT name FROM pragma_index_list('jav_idol') WHERE origin != 'pk'`)
	if err != nil {
		t.Fatal(err)
	}
	defer idxRows.Close()
	indexes := map[string]bool{}
	for idxRows.Next() {
		var name string
		if err := idxRows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		indexes[name] = true
	}
	for _, want := range []string{"idx_jav_idol_name", "idx_jav_idol_gender", "idx_jav_idol_cover_jav_id"} {
		if !indexes[want] {
			t.Fatalf("missing index %s after migration: %v", want, indexes)
		}
	}

	// AUTOINCREMENT sequence must not reset (next insert should not collide).
	var seq int
	if err := db.QueryRow(`SELECT seq FROM sqlite_sequence WHERE name = 'jav_idol'`).Scan(&seq); err != nil {
		t.Fatalf("read sqlite_sequence: %v", err)
	}
	if seq < 3 {
		t.Fatalf("jav_idol sqlite_sequence = %d, want >= 3 (AUTOINCREMENT must survive rebuild)", seq)
	}
	res, err := db.Exec(`INSERT INTO jav_idol (name) VALUES ('新演员D')`)
	if err != nil {
		t.Fatalf("insert after migration: %v", err)
	}
	newID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}
	if newID <= 3 {
		t.Fatalf("new idol id = %d, expected > 3 (sequence was not preserved)", newID)
	}

	// Re-enable FK and verify the whole graph is still consistent.
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatal(err)
	}
	fkRows, err := db.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer fkRows.Close()
	if fkRows.Next() {
		cols, _ := fkRows.Columns()
		t.Fatalf("foreign_key_check reported violations: %v", cols)
	}

	var integrity string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		t.Fatal(err)
	}
	if integrity != "ok" {
		t.Fatalf("integrity_check = %q", integrity)
	}
}

// TestAlignJavIdolGenderSchemaIsIdempotent runs the alignment twice and proves
// the related data is untouched on the second (repeat) execution.
func TestAlignJavIdolGenderSchemaIsIdempotent(t *testing.T) {
	db := openTempDB(t)
	buildPreGenderSchema(t, db)

	for run := 1; run <= 2; run++ {
		tx, err := db.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := alignJavIdolGenderSchema(context.Background(), tx); err != nil {
			tx.Rollback()
			t.Fatalf("run %d: %v", run, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}

	if n := countRows(t, db, `SELECT COUNT(*) FROM jav_idol`); n != 3 {
		t.Fatalf("after idempotent re-run jav_idol = %d, want 3", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM jav_idol_alias`); n != 2 {
		t.Fatalf("after idempotent re-run jav_idol_alias = %d, want 2", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM jav_idol_map`); n != 2 {
		t.Fatalf("after idempotent re-run jav_idol_map = %d, want 2", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM jav_idol WHERE gender = 'male'`); n != 1 {
		t.Fatalf("after idempotent re-run male count = %d, want 1", n)
	}
}

// TestAlignJavIdolGenderSchemaRollback verifies that a failed/aborted migration
// transaction leaves both the schema and the related data exactly as they were
// before the migration started (no partial table drop, no orphaned children).
func TestAlignJavIdolGenderSchemaRollback(t *testing.T) {
	db := openTempDB(t)
	buildPreGenderSchema(t, db)

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := alignJavIdolGenderSchema(context.Background(), tx); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	// Simulate a failure after the migration body: roll back instead of commit.
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	// Pre-migration schema: gender is the LAST column (position 15, 0-indexed),
	// and the data must be fully intact.
	var colCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('jav_idol')`).Scan(&colCount); err != nil {
		t.Fatal(err)
	}
	if colCount != 16 {
		t.Fatalf("rolled-back jav_idol column count = %d, want 16", colCount)
	}
	var lastCol string
	if err := db.QueryRow(`SELECT name FROM pragma_table_info('jav_idol') ORDER BY cid DESC LIMIT 1`).Scan(&lastCol); err != nil {
		t.Fatal(err)
	}
	if lastCol != "gender" {
		t.Fatalf("rolled-back schema: last column = %q, want 'gender' (old layout preserved)", lastCol)
	}

	if n := countRows(t, db, `SELECT COUNT(*) FROM jav_idol`); n != 3 {
		t.Fatalf("rolled-back jav_idol = %d, want 3", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM jav_idol_alias`); n != 2 {
		t.Fatalf("rolled-back jav_idol_alias = %d, want 2", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM jav_idol_map`); n != 2 {
		t.Fatalf("rolled-back jav_idol_map = %d, want 2", n)
	}
	// The rebuilt temporary table must not linger.
	if n := countRows(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE name = '__new_jav_idol'`); n != 0 {
		t.Fatalf("temporary table __new_jav_idol still present after rollback")
	}
}

// TestAlignJavIdolGenderSchemaRequiresForeignKeysOff enforces the safety
// contract: the rebuild in 202611130002 drops and renames the whole jav_idol
// table. If foreign keys were ENABLED during the migration, SQLite's implicit
// DELETE on DROP TABLE would cascade-delete jav_idol_alias and jav_idol_map.
// Production avoids this because db.Open sets PRAGMA foreign_keys=OFF before
// running migrations (open.go). This test asserts the migration is correct
// under that contract (FK OFF) and leaves the graph FK-consistent afterwards.
func TestAlignJavIdolGenderSchemaRequiresForeignKeysOff(t *testing.T) {
	db := openTempDB(t) // FK OFF, matching db.Open's migration contract
	buildPreGenderSchema(t, db)

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := alignJavIdolGenderSchema(context.Background(), tx); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	if n := countRows(t, db, `SELECT COUNT(*) FROM jav_idol_alias`); n != 2 {
		t.Fatalf("FK=OFF: jav_idol_alias = %d, want 2 (no cascade delete)", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM jav_idol_map`); n != 2 {
		t.Fatalf("FK=OFF: jav_idol_map = %d, want 2 (no cascade delete)", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM jav_idol`); n != 3 {
		t.Fatalf("FK=OFF: jav_idol = %d, want 3", n)
	}

	// After the migration the contract is that FK is turned back ON; the graph
	// must be fully consistent (no orphaned children).
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatal(err)
	}
	fkRows, err := db.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer fkRows.Close()
	if fkRows.Next() {
		t.Fatalf("foreign_key_check reported violations after migration under FK=OFF contract")
	}
}
