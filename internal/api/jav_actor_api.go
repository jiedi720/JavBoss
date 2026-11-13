package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"
	"javboss/internal/models"
)

const actorGenderContextKey = "javboss_actor_gender"

// withActorGender marks a route as belonging to the male actor module. Routes
// without it are the original actress endpoints and always use female.
func withActorGender(gender string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(actorGenderContextKey, gender)
		c.Next()
	}
}

func actorGender(c *gin.Context) string {
	if value, exists := c.Get(actorGenderContextKey); exists {
		if gender, ok := value.(string); ok && models.IsValidActorGender(gender) {
			return gender
		}
	}
	return models.ActorGenderFemale
}

func actorNameCN(gender string) string {
	if gender == models.ActorGenderMale {
		return "男优"
	}
	return "女优"
}

func actorNameEN(gender string) string {
	if gender == models.ActorGenderMale {
		return "actor"
	}
	return "idol"
}

// respondActorError builds a gender-correct localized error. Placeholders %s in
// the templates are replaced with the localized actor noun.
func respondActorError(c *gin.Context, status int, cn, en string) {
	gender := actorGender(c)
	respondLocalizedError(c, status,
		strings.ReplaceAll(cn, "%s", actorNameCN(gender)),
		strings.ReplaceAll(en, "%s", actorNameEN(gender)),
	)
}

// requireActorGender rejects requests that address an actor through the wrong
// gender's endpoints. It also returns a proper 404 for missing ids.
func requireActorGender(c *gin.Context, id int64, gender string) bool {
	item, err := dbpkg.Default().GetJavIdolSummary(c.Request.Context(), id, nil)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondActorError(c, http.StatusNotFound, "%s不存在", "%s was not found")
			return false
		}
		logging.Error("load jav actor gender id=%d: %v", id, err)
		respondActorError(c, http.StatusInternalServerError, "加载%s信息失败", "Failed to load %s information")
		return false
	}
	if item.Gender != gender {
		respondActorError(c, http.StatusNotFound, "%s不存在", "%s was not found")
		return false
	}
	return true
}

// deleteJavIdol removes a manually managed actor and all dependent rows.
func deleteJavIdol(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondActorError(c, http.StatusBadRequest, "%s ID 无效", "Invalid %s ID")
		return
	}
	gender := actorGender(c)
	if !requireActorGender(c, id, gender) {
		return
	}
	if err := dbpkg.Default().DeleteJavIdol(c.Request.Context(), id, gender); err != nil {
		logging.Error("delete jav actor id=%d: %v", id, err)
		respondActorError(c, http.StatusBadRequest, "删除%s失败", "Failed to delete %s")
		return
	}
	c.Status(http.StatusNoContent)
}
