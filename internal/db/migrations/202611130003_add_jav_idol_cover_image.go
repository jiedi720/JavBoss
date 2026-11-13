package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext(
		"202611130003_add_jav_idol_cover_image.go",
		addJavIdolCoverImage,
		irreversibleMigration,
	)
}

// addJavIdolCoverImage lets an actor keep a locally uploaded or URL-downloaded
// cover instead of borrowing one from a work. The column holds the cover file
// code (without extension); an empty value preserves the old work-based cover.
func addJavIdolCoverImage(ctx context.Context, tx *sql.Tx) error {
	return addColumnIfMissing(ctx, tx, "jav_idol", "cover_image", "text NOT NULL DEFAULT \"\"")
}
