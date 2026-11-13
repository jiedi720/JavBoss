package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext(
		"202611130004_add_video_favorite_rating.go",
		addVideoFavoriteRating,
		irreversibleMigration,
	)
}

// addVideoFavoriteRating adds a per-video favourite rating so videos can be
// rated the same way JAV works are, without sharing the JAV rating.
func addVideoFavoriteRating(ctx context.Context, tx *sql.Tx) error {
	return addColumnIfMissing(ctx, tx, "video", "favorite_rating", "real NOT NULL DEFAULT 0")
}
