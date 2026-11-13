package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext(
		"202611130002_align_jav_idol_gender_schema.go",
		alignJavIdolGenderSchema,
		irreversibleMigration,
	)
}

// jav_idol columns in the exact order declared by models.JavIdol. The GORM
// reference schema is compared column-by-column by
// TestMigratedSchemaMatchesGormModels, so a plain ALTER TABLE (which appends
// gender at the end and stores DEFAULT 'female' with single quotes) is not
// enough: the table has to be rebuilt.
const (
	javIdolGenderColumns = `"id", "gender", "name", "roman_name", "japanese_name", "chinese_name", "height_cm", "birth_date", "bust", "waist", "hips", "cup", "created_at", "updated_at", "cover_jav_id", "cover_crop_left"`
	javIdolGenderSelect  = `"id", CASE WHEN "gender" IN ('female', 'male') THEN "gender" ELSE 'female' END, "name", "roman_name", "japanese_name", "chinese_name", "height_cm", "birth_date", "bust", "waist", "hips", "cup", "created_at", "updated_at", "cover_jav_id", "cover_crop_left"`
)

// alignJavIdolGenderSchema rebuilds jav_idol so the migrated schema matches the
// GORM model exactly (column order, NOT NULL, default literal and indexes) and
// backfills any legacy/invalid gender value with 'female'. Re-running it is safe.
func alignJavIdolGenderSchema(ctx context.Context, tx *sql.Tx) error {
	exists, err := columnExists(ctx, tx, "jav_idol", "gender")
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("jav_idol.gender must exist before schema alignment")
	}
	return execStatements(ctx, tx,
		`DROP TABLE IF EXISTS "__new_jav_idol"`,
		`CREATE TABLE "__new_jav_idol" (
			id integer PRIMARY KEY AUTOINCREMENT,
			gender text NOT NULL DEFAULT "female",
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
			cover_crop_left real NOT NULL DEFAULT 0.53
		)`,
		`INSERT INTO "__new_jav_idol" (`+javIdolGenderColumns+`) SELECT `+javIdolGenderSelect+` FROM "jav_idol"`,
		`DROP TABLE "jav_idol"`,
		`ALTER TABLE "__new_jav_idol" RENAME TO "jav_idol"`,
		`CREATE UNIQUE INDEX idx_jav_idol_name ON jav_idol(name)`,
		`CREATE INDEX idx_jav_idol_gender ON jav_idol(gender)`,
		`CREATE INDEX idx_jav_idol_cover_jav_id ON jav_idol(cover_jav_id)`,
	)
}
