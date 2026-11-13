package migrations

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// TestRealJavBossDBMigration applies the male-actor gender migrations
// (202611130001 + 202611130002) to a REAL production-exported JavBoss database
// and proves that every related row (aliases, video associations), the original
// idol ids, the AUTOINCREMENT sequence and the foreign-key/integrity graph are
// preserved. This is the "real association scenario" safety test required by the
// review.
//
// It is gated behind REAL_DB_PATH so it never runs in CI without a database.
// Example:
//
//	REAL_DB_PATH=testdata/javboss.db go test ./internal/db/migrations/ -run TestRealJavBossDBMigration -v
func TestRealJavBossDBMigration(t *testing.T) {
	src := os.Getenv("REAL_DB_PATH")
	if src == "" {
		t.Skip("REAL_DB_PATH not set; skipping real-database migration test")
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read REAL_DB_PATH %q: %v", src, err)
	}

	dir := t.TempDir()
	baselinePath := filepath.Join(dir, "baseline.db")
	migratedPath := filepath.Join(dir, "migrated.db")
	if err := os.WriteFile(baselinePath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(migratedPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	openRO := func(p string) *sql.DB {
		db, err := sql.Open("sqlite3", p+"?_foreign_keys=OFF")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		return db
	}

	type counts struct {
		idol, alias, idmap, male int
		minID, maxID, seq        int
	}
	hasGender := func(db *sql.DB) bool {
		rows, err := db.Query(`PRAGMA table_info(jav_idol)`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var cid int
			var name, colType string
			var notNull int
			var def any
			var pk int
			if err := rows.Scan(&cid, &name, &colType, &notNull, &def, &pk); err != nil {
				t.Fatal(err)
			}
			if name == "gender" {
				return true
			}
		}
		return false
	}
	read := func(db *sql.DB) counts {
		var c counts
		c.idol = mustCount(t, db, `SELECT COUNT(*) FROM jav_idol`)
		c.alias = mustCount(t, db, `SELECT COUNT(*) FROM jav_idol_alias`)
		c.idmap = mustCount(t, db, `SELECT COUNT(*) FROM jav_idol_map`)
		if hasGender(db) {
			c.male = mustCount(t, db, `SELECT COUNT(*) FROM jav_idol WHERE gender = 'male'`)
		}
		c.minID = mustCount(t, db, `SELECT COALESCE(MIN(id),0) FROM jav_idol`)
		c.maxID = mustCount(t, db, `SELECT COALESCE(MAX(id),0) FROM jav_idol`)
		c.seq = mustCount(t, db, `SELECT COALESCE((SELECT seq FROM sqlite_sequence WHERE name='jav_idol'),0)`)
		return c
	}

	baselineDB := openRO(baselinePath)
	before := read(baselineDB)
	_ = baselineDB.Close()

	migDB := openRO(migratedPath)
	tx, err := migDB.BeginTx(context.Background(), nil)
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

	after := read(migDB)

	// Re-enable FK and validate the whole graph.
	if _, err := migDB.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatal(err)
	}
	if rows, err := migDB.Query(`PRAGMA foreign_key_check`); err != nil {
		t.Fatal(err)
	} else {
		if rows.Next() {
			t.Fatalf("foreign_key_check reported violations after migrating REAL db")
		}
		_ = rows.Close()
	}
	var integrity string
	if err := migDB.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		t.Fatal(err)
	}
	if integrity != "ok" {
		t.Fatalf("integrity_check = %q after migrating REAL db", integrity)
	}
	_ = migDB.Close()

	// Assertions: the real data must be untouched by the migration.
	if after.idol != before.idol {
		t.Fatalf("jav_idol %d -> %d (must be preserved)", before.idol, after.idol)
	}
	if after.alias != before.alias {
		t.Fatalf("jav_idol_alias %d -> %d (aliases must survive)", before.alias, after.alias)
	}
	if after.idmap != before.idmap {
		t.Fatalf("jav_idol_map %d -> %d (video associations must survive)", before.idmap, after.idmap)
	}
	if after.minID != before.minID || after.maxID != before.maxID {
		t.Fatalf("jav_idol id range %d..%d -> %d..%d (ids must be preserved)",
			before.minID, before.maxID, after.minID, after.maxID)
	}
	if after.seq < before.seq {
		t.Fatalf("jav_idol sqlite_sequence %d -> %d (sequence must not regress)", before.seq, after.seq)
	}
	// Real production export is female-only; male must remain 0 (no spurious males).
	if after.male != before.male {
		t.Fatalf("male count %d -> %d (unexpected male rows)", before.male, after.male)
	}

	t.Logf("REAL db migration OK: idols=%d aliases=%d idmap=%d ids=%d..%d seq=%d->%d fk=ok integrity=%s",
		after.idol, after.alias, after.idmap, after.minID, after.maxID, before.seq, after.seq, integrity)
}

func mustCount(t *testing.T, db *sql.DB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", q, err)
	}
	return n
}
