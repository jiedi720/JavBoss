package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/models"

	"gorm.io/gorm"
)

// JavIdolProfileInput is a scraped actress profile ready to be persisted.
type JavIdolProfileInput struct {
	Name         string
	JapaneseName string
	RomanName    string
	ChineseName  string
	HeightCM     int
	Bust         int
	Waist        int
	Hips         int
	Cup          int
	// BirthDate is a Unix timestamp in seconds, zero when unknown.
	BirthDate int64
	Aliases   []string
	// ProfileURL is kept for logging only; it is not stored on the idol row.
	ProfileURL string
}

// JavIdolFillInput selects the idol to fill and how aggressively to write.
type JavIdolFillInput struct {
	// IdolID targets an existing idol. Zero falls back to a name/alias match.
	IdolID int64
	// JavID links the filled idol to that work when it is not linked yet.
	JavID int64
	// Overwrite replaces fields that already hold a value.
	Overwrite bool
	Profile   JavIdolProfileInput
}

// FillJavIdolFromProfile writes a scraped profile onto an idol and optionally
// links that idol to a work. The idol is matched by id first and by name or alias
// second, so filling never creates a duplicate row for a known actress. Without
// Overwrite only the fields that are still empty are written.
func (store *Store) FillJavIdolFromProfile(ctx context.Context, input JavIdolFillInput) (*models.JavIdol, []string, error) {
	displayName := strings.TrimSpace(input.Profile.JapaneseName)
	if displayName == "" {
		displayName = strings.TrimSpace(input.Profile.Name)
	}
	if displayName == "" {
		return nil, nil, errors.New("actress name is required")
	}

	var idol models.JavIdol
	filled := make([]string, 0, 8)
	err := store.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		targetID := input.IdolID
		if targetID > 0 {
			if err := tx.Where("id = ?", targetID).First(&idol).Error; err != nil {
				return fmt.Errorf("find jav idol: %w", err)
			}
			if idol.Gender != models.ActorGenderFemale {
				return fmt.Errorf("actor %d is not a female actor", targetID)
			}
		} else {
			created, err := findOrCreateJavIdolByNameOrAliasWithGenderTx(tx, displayName, models.ActorGenderFemale)
			if err != nil {
				return fmt.Errorf("resolve jav idol %q: %w", displayName, err)
			}
			idol = created
			targetID = created.ID
		}

		updates := make(map[string]any, 8)
		applyText := func(column, current, next string) {
			next = strings.TrimSpace(next)
			if next == "" || current == next {
				return
			}
			if strings.TrimSpace(current) != "" && !input.Overwrite {
				return
			}
			updates[column] = next
			filled = append(filled, column)
		}
		applyNumber := func(column string, current *int, next int) {
			if next <= 0 {
				return
			}
			if current != nil {
				if *current == next {
					return
				}
				if *current > 0 && !input.Overwrite {
					return
				}
			}
			value := next
			updates[column] = &value
			filled = append(filled, column)
		}

		applyText("japanese_name", idol.JapaneseName, input.Profile.JapaneseName)
		applyText("roman_name", idol.RomanName, input.Profile.RomanName)
		applyText("chinese_name", idol.ChineseName, input.Profile.ChineseName)
		applyNumber("height_cm", idol.HeightCM, input.Profile.HeightCM)
		applyNumber("bust", idol.Bust, input.Profile.Bust)
		applyNumber("waist", idol.Waist, input.Profile.Waist)
		applyNumber("hips", idol.Hips, input.Profile.Hips)
		applyNumber("cup", idol.Cup, input.Profile.Cup)
		if input.Profile.BirthDate > 0 {
			next := time.Unix(input.Profile.BirthDate, 0).UTC()
			empty := idol.BirthDate == nil || idol.BirthDate.IsZero()
			if empty || (input.Overwrite && !idol.BirthDate.Equal(next)) {
				updates["birth_date"] = next
				filled = append(filled, "birth_date")
			}
		}

		if len(updates) > 0 {
			if err := tx.Model(&models.JavIdol{}).Where("id = ?", targetID).Updates(updates).Error; err != nil {
				return fmt.Errorf("update jav idol profile: %w", err)
			}
		}

		if len(input.Profile.Aliases) > 0 {
			var existing []models.JavIdolAlias
			if err := tx.Where("jav_idol_id = ?", targetID).Find(&existing).Error; err != nil {
				return fmt.Errorf("load jav idol aliases: %w", err)
			}
			names := make([]string, 0, len(existing)+len(input.Profile.Aliases))
			for _, item := range existing {
				names = append(names, item.Alias)
			}
			names = append(names, input.Profile.Aliases...)
			aliases := normalizeJavIdolAliases(names, idol.Name, input.Profile.JapaneseName, input.Profile.RomanName, input.Profile.ChineseName)
			if len(aliases) > len(existing) {
				// A conflicting alias is not worth failing the profile fill for; the
				// measurements are already written at this point.
				if err := validateJavIdolAliasesTx(tx, targetID, aliases); err != nil {
					logging.Info("fill jav idol aliases skipped id=%d: %v", targetID, err)
				} else if err := replaceJavIdolAliasesTx(tx, targetID, aliases); err != nil {
					return err
				}
			}
		}

		if input.JavID > 0 {
			var linked int64
			if err := tx.Model(&models.JavIdolMap{}).
				Where("jav_id = ? AND jav_idol_id = ?", input.JavID, targetID).
				Count(&linked).Error; err != nil {
				return fmt.Errorf("check jav idol link: %w", err)
			}
			if linked == 0 {
				link := models.JavIdolMap{JavID: input.JavID, JavIdolID: targetID}
				if err := tx.Create(&link).Error; err != nil {
					return fmt.Errorf("link jav idol: %w", err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	if err := store.conn.WithContext(ctx).Where("id = ?", idol.ID).First(&idol).Error; err != nil {
		return nil, nil, fmt.Errorf("reload jav idol: %w", err)
	}
	return &idol, filled, nil
}
