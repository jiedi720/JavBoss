package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestAlignJavIdolGenderSchema(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Simulate the state left by 202611130001: gender appended at the end with a
	// single-quoted default, plus legacy rows and one already-male actor.
	if _, err := db.Exec(`
		CREATE TABLE jav_idol (
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
		);
		CREATE UNIQUE INDEX idx_jav_idol_name ON jav_idol(name);
		CREATE INDEX idx_jav_idol_cover_jav_id ON jav_idol(cover_jav_id);
		CREATE INDEX idx_jav_idol_gender ON jav_idol(gender);
		INSERT INTO jav_idol (id, name, gender) VALUES (1, 'Legacy Idol', 'female'), (2, 'Invalid Gender', 'unknown'), (3, 'Manual Actor', 'male');
	`); err != nil {
		t.Fatal(err)
	}

	// Run twice to prove the migration is idempotent.
	for i := 0; i < 2; i++ {
		tx, err := db.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := alignJavIdolGenderSchema(context.Background(), tx); err != nil {
			tx.Rollback()
			t.Fatalf("run %d: %v", i+1, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := db.Query(`SELECT name FROM pragma_table_info('jav_idol') ORDER BY cid`)
	if err != nil {
		t.Fatal(err)
	}
	var columns []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		columns = append(columns, name)
	}
	rows.Close()
	if len(columns) != 16 || columns[1] != "gender" {
		t.Fatalf("jav_idol columns = %v, want gender at position 1", columns)
	}

	var defaultValue string
	if err := db.QueryRow(`SELECT dflt_value FROM pragma_table_info('jav_idol') WHERE name = 'gender'`).Scan(&defaultValue); err != nil {
		t.Fatal(err)
	}
	if defaultValue != `"female"` {
		t.Fatalf("gender default = %q, want %q", defaultValue, `"female"`)
	}

	checks := []struct {
		id     int
		name   string
		gender string
	}{
		{1, "Legacy Idol", "female"},
		{2, "Invalid Gender", "female"},
		{3, "Manual Actor", "male"},
	}
	for _, check := range checks {
		var name, gender string
		if err := db.QueryRow(`SELECT name, gender FROM jav_idol WHERE id = ?`, check.id).Scan(&name, &gender); err != nil {
			t.Fatal(err)
		}
		if name != check.name || gender != check.gender {
			t.Fatalf("row %d = (%q, %q), want (%q, %q)", check.id, name, gender, check.name, check.gender)
		}
	}

	indexRows, err := db.Query(`SELECT name FROM pragma_index_list('jav_idol') WHERE origin != 'pk'`)
	if err != nil {
		t.Fatal(err)
	}
	defer indexRows.Close()
	indexes := map[string]bool{}
	for indexRows.Next() {
		var name string
		if err := indexRows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		indexes[name] = true
	}
	for _, want := range []string{"idx_jav_idol_name", "idx_jav_idol_gender", "idx_jav_idol_cover_jav_id"} {
		if !indexes[want] {
			t.Fatalf("missing index %s: %v", want, indexes)
		}
	}

	var integrity string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		t.Fatal(err)
	}
	if integrity != "ok" {
		t.Fatal(fmt.Sprintf("integrity_check = %q", integrity))
	}
}

func TestAddJavIdolGender(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE jav_idol (id integer PRIMARY KEY, name text); INSERT INTO jav_idol VALUES (1, 'Legacy Idol')`); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for i := 0; i < 2; i++ {
		if err := addJavIdolGender(context.Background(), tx); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var name, gender string
	if err := db.QueryRow(`SELECT name, gender FROM jav_idol WHERE id = 1`).Scan(&name, &gender); err != nil {
		t.Fatal(err)
	}
	if name != "Legacy Idol" || gender != "female" {
		t.Fatalf("migrated row = (%q, %q), want (%q, %q)", name, gender, "Legacy Idol", "female")
	}
	var indexName string
	if err := db.QueryRow(`SELECT name FROM pragma_index_list('jav_idol') WHERE name = 'idx_jav_idol_gender'`).Scan(&indexName); err != nil {
		t.Fatal(err)
	}
}
