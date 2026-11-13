package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext(
		"202611130001_add_jav_idol_gender.go",
		addJavIdolGender,
		irreversibleMigration,
	)
}

func addJavIdolGender(ctx context.Context, tx *sql.Tx) error {
	if err := addColumnIfMissing(ctx, tx, "jav_idol", "gender", "text NOT NULL DEFAULT 'female'"); err != nil {
		return err
	}
	return execDB(ctx, tx, `CREATE INDEX IF NOT EXISTS idx_jav_idol_gender ON jav_idol(gender)`)
}
