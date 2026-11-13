package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"
	"javboss/internal/jav/scrape"
	"javboss/internal/util"
)

// actressProfilePayload is the wire shape of one scraped actress profile.
type actressProfilePayload struct {
	JapaneseName string   `json:"japanese_name"`
	RomanName    string   `json:"roman_name"`
	ChineseName  string   `json:"chinese_name"`
	HeightCM     int      `json:"height_cm"`
	Bust         int      `json:"bust"`
	Waist        int      `json:"waist"`
	Hips         int      `json:"hips"`
	Cup          int      `json:"cup"`
	BirthDate    string   `json:"birth_date"`
	ProfileURL   string   `json:"profile_url"`
	Aliases      []string `json:"aliases"`
}

func newActressProfilePayload(info *scrape.ActressInfo) actressProfilePayload {
	payload := actressProfilePayload{}
	if info == nil {
		return payload
	}
	payload.JapaneseName = info.JapaneseName
	payload.RomanName = info.RomanName
	payload.ChineseName = info.ChineseName
	payload.HeightCM = info.HeightCM
	payload.Bust = info.Bust
	payload.Waist = info.Waist
	payload.Hips = info.Hips
	payload.Cup = info.Cup
	payload.ProfileURL = info.ProfileURL
	if len(info.Aliases) > 0 {
		payload.Aliases = info.Aliases
	}
	if info.BirthDate > 0 {
		payload.BirthDate = time.Unix(int64(info.BirthDate), 0).UTC().Format("2006-01-02")
	}
	return payload
}

// resolveJavActressProfile reads a profile by name. MinnanoAV is the primary
// source because it also reports former names; AV Wiki and JavModel are the
// fallbacks used when Minnanon returns nothing.
func resolveJavActressProfile(ctx context.Context, name string) (*scrape.ActressInfo, string, error) {
	sources := []struct {
		provider scrape.Provider
		label    string
	}{
		{scrape.ProviderMinnanoAV, "minnano"},
		{scrape.ProviderAVWiki, "avwiki"},
		{scrape.ProviderJavModel, "javmodel"},
	}
	for _, source := range sources {
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		info, err := scrape.LookupActressByJapaneseName(ctx, name, source.provider)
		if err != nil {
			if !errors.Is(err, scrape.ErrNotFound) {
				logging.Info("actress profile lookup failed source=%s name=%s err=%v", source.label, name, err)
			}
			continue
		}
		if info == nil {
			continue
		}
		return info, source.label, nil
	}
	return nil, "", scrape.ErrNotFound
}

// lookupJavActressesByCode lists the actresses credited on a work, so a card
// without actress links can still be filled in.
func lookupJavActressesByCode(c *gin.Context) {
	code := strings.ToUpper(strings.TrimSpace(c.Query("code")))
	if code == "" {
		respondLocalizedError(c, http.StatusBadRequest, "番号不能为空", "Code is required")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()
	refs, err := scrape.LookupActressRefsByCode(ctx, code)
	if err != nil {
		if errors.Is(err, scrape.ErrNotFound) {
			respondLocalizedError(c, http.StatusNotFound, "该番号没有查到女优信息", "No actress was found for this code")
			return
		}
		logging.Error("lookup actresses code=%s: %v", code, err)
		respondLocalizedError(c, http.StatusBadGateway, "查询女优失败，请稍后重试", "Failed to look up actresses; try again later")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":      code,
		"source":    "avwiki",
		"actresses": refs,
	})
}

// getJavActressProfile returns a scraped profile for one actress name. It powers
// the preview before the user commits a fill.
func getJavActressProfile(c *gin.Context) {
	name := strings.TrimSpace(c.Query("name"))
	if name == "" {
		respondLocalizedError(c, http.StatusBadRequest, "女优名不能为空", "Actress name is required")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()
	info, source, err := resolveJavActressProfile(ctx, name)
	if err != nil {
		if errors.Is(err, scrape.ErrNotFound) {
			respondLocalizedError(c, http.StatusNotFound, "没有查到该女优的资料", "No profile was found for this actress")
			return
		}
		logging.Error("actress profile name=%s: %v", name, err)
		respondLocalizedError(c, http.StatusBadGateway, "查询女优资料失败，请稍后重试", "Failed to load the actress profile; try again later")
		return
	}
	if info.ChineseName != "" {
		info.ChineseName = util.SimplifyChineseName(info.ChineseName)
	}
	c.JSON(http.StatusOK, gin.H{
		"name":    name,
		"source":  source,
		"profile": newActressProfilePayload(info),
	})
}

// fillJavActressProfile writes a scraped profile onto an existing idol or creates
// the idol, and optionally links it to the work being viewed. The caller normally
// sends the profile it previewed so the saved values match what was on screen; a
// bare name falls back to a fresh lookup.
func fillJavActressProfile(c *gin.Context) {
	var req struct {
		JavID     int64                  `json:"jav_id"`
		IdolID    int64                  `json:"idol_id"`
		Name      string                 `json:"name"`
		Overwrite bool                   `json:"overwrite"`
		Profile   *actressProfilePayload `json:"profile"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "回填请求无效", "Invalid fill request")
		return
	}
	name := strings.TrimSpace(req.Name)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	profile := dbpkg.JavIdolProfileInput{}
	source := "preview"
	if req.Profile != nil && strings.TrimSpace(req.Profile.JapaneseName) != "" {
		profile = javIdolProfileFromPayload(*req.Profile)
		if name == "" {
			name = profile.JapaneseName
		}
	} else {
		if name == "" {
			respondLocalizedError(c, http.StatusBadRequest, "女优名不能为空", "Actress name is required")
			return
		}
		info, resolvedSource, err := resolveJavActressProfile(ctx, name)
		if err != nil {
			if errors.Is(err, scrape.ErrNotFound) {
				respondLocalizedError(c, http.StatusNotFound, "没有查到该女优的资料", "No profile was found for this actress")
				return
			}
			logging.Error("fill actress profile name=%s: %v", name, err)
			respondLocalizedError(c, http.StatusBadGateway, "查询女优资料失败，请稍后重试", "Failed to load the actress profile; try again later")
			return
		}
		if info.ChineseName != "" {
			info.ChineseName = util.SimplifyChineseName(info.ChineseName)
		}
		source = resolvedSource
		profile = javIdolProfileFromInfo(name, info)
	}
	profile.Name = name
	if strings.TrimSpace(profile.JapaneseName) == "" {
		profile.JapaneseName = name
	}

	idol, filled, err := dbpkg.Default().FillJavIdolFromProfile(ctx, dbpkg.JavIdolFillInput{
		IdolID:    req.IdolID,
		JavID:     req.JavID,
		Overwrite: req.Overwrite,
		Profile:   profile,
	})
	if err != nil {
		logging.Error("fill actress profile name=%s idol=%d jav=%d: %v", name, req.IdolID, req.JavID, err)
		respondLocalizedError(c, http.StatusBadRequest, "写入女优资料失败", "Failed to save the actress profile")
		return
	}
	if filled == nil {
		filled = []string{}
	}
	payload := gin.H{
		"idol":   idol,
		"source": source,
		"filled": filled,
	}
	if req.JavID > 0 {
		if jav, err := dbpkg.Default().GetJav(ctx, req.JavID, nil); err == nil && jav != nil {
			payload["jav"] = jav
		}
	}
	c.JSON(http.StatusOK, payload)
}

// javIdolProfileFromPayload converts a previewed profile into the persistence input.
func javIdolProfileFromPayload(payload actressProfilePayload) dbpkg.JavIdolProfileInput {
	input := dbpkg.JavIdolProfileInput{
		Name:         strings.TrimSpace(payload.JapaneseName),
		JapaneseName: payload.JapaneseName,
		RomanName:    payload.RomanName,
		ChineseName:  payload.ChineseName,
		HeightCM:     payload.HeightCM,
		Bust:         payload.Bust,
		Waist:        payload.Waist,
		Hips:         payload.Hips,
		Cup:          payload.Cup,
		Aliases:      payload.Aliases,
		ProfileURL:   payload.ProfileURL,
	}
	if raw := strings.TrimSpace(payload.BirthDate); raw != "" {
		if parsed, err := time.Parse("2006-01-02", raw); err == nil {
			input.BirthDate = parsed.Unix()
		}
	}
	return input
}

// javIdolProfileFromInfo converts a provider result into the persistence input.
func javIdolProfileFromInfo(name string, info *scrape.ActressInfo) dbpkg.JavIdolProfileInput {
	input := dbpkg.JavIdolProfileInput{
		Name:       name,
		Aliases:    info.Aliases,
		ProfileURL: info.ProfileURL,
	}
	input.JapaneseName = info.JapaneseName
	input.RomanName = info.RomanName
	input.ChineseName = info.ChineseName
	input.HeightCM = info.HeightCM
	input.Bust = info.Bust
	input.Waist = info.Waist
	input.Hips = info.Hips
	input.Cup = info.Cup
	if info.BirthDate > 0 {
		input.BirthDate = int64(info.BirthDate)
	}
	return input
}
