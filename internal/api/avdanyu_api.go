package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"javboss/internal/common/logging"
	"javboss/internal/jav/scrape/avdanyu"
)

// lookupAvdanyuActors searches AV男優まとめ for male actors by JAV code or
// actress name. Results are returned as candidates only — nothing is written
// to the database until the user confirms them in the edit dialog.
func lookupAvdanyuActors(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("keyword"))
	if keyword == "" {
		keyword = strings.TrimSpace(c.Query("code"))
	}
	if keyword == "" {
		respondLocalizedError(c, http.StatusBadRequest, "关键词不能为空", "Keyword is required")
		return
	}

	client := avdanyu.NewClient()
	result, err := client.Lookup(c.Request.Context(), keyword)
	if err != nil {
		if result != nil && result.Error == "cloudflare_blocked" {
			respondLocalizedError(c, http.StatusServiceUnavailable,
				"暂未能通过网站验证，请先在浏览器里打开一次 avdanyuwiki.com",
				"Site verification pending; open avdanyuwiki.com in the browser first")
			return
		}
		logging.Error("avdanyu lookup keyword=%s: %v", keyword, err)
		respondLocalizedError(c, http.StatusServiceUnavailable,
			"AV男優まとめ 查询暂不可用（需要本地浏览器支持）",
			"AV男優まとめ lookup unavailable (local browser required)")
		return
	}

	works := result.Works
	if works == nil {
		works = []avdanyu.Work{}
	}
	c.JSON(http.StatusOK, gin.H{
		"keyword":   result.Keyword,
		"title":     result.Title,
		"works":     works,
		"available": true,
	})
}
