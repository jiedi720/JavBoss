package api

import (
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"javboss/internal/common"
	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"
	"javboss/internal/jav/scrape"
	"javboss/internal/jav/scrape/cover"
	"javboss/internal/models"
)

func listJavIdols(c *gin.Context) {
	limit := queryInt(c, "limit", 100)
	offset := queryInt(c, "offset", 0)
	search := strings.TrimSpace(c.Query("search"))
	sort := strings.TrimSpace(c.Query("sort"))
	favoriteGroupID := int64(0)
	if favoriteGroupParam := strings.TrimSpace(c.Query("favorite_group_id")); favoriteGroupParam != "" {
		parsed, err := strconv.ParseInt(favoriteGroupParam, 10, 64)
		if err != nil || parsed <= 0 {
			respondLocalizedError(c, http.StatusBadRequest, "收藏夹 ID 无效", "Invalid favorite group ID")
			return
		}
		favoriteGroupID = parsed
	}
	filters, ok := parseJavIdolFilters(c)
	if !ok {
		return
	}
	gender := actorGender(c)
	filters.Gender = gender

	items, total, err := dbpkg.Default().ListJavIdols(c.Request.Context(), search, sort, limit, offset, nil, favoriteGroupID, filters)
	if err != nil {
		logging.Error("list jav actors gender=%s: %v", gender, err)
		respondActorError(c, http.StatusInternalServerError, "加载%s列表失败", "Failed to load %ss")
		return
	}
	enrichJavIdolSummaries(c.Request.Context(), items)
	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
	})
}

func createJavIdol(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondActorError(c, http.StatusBadRequest, "创建%s请求无效", "Invalid %s creation request")
		return
	}

	gender := actorGender(c)
	var (
		idol *models.JavIdol
		err  error
	)
	if gender == models.ActorGenderMale {
		idol, err = dbpkg.Default().CreateJavActor(c.Request.Context(), req.Name)
	} else {
		idol, err = dbpkg.Default().CreateJavIdol(c.Request.Context(), req.Name)
	}
	if err != nil {
		logging.Error("create jav actor gender=%s error: %v", gender, err)
		respondActorError(c, http.StatusBadRequest, "创建%s失败，名称可能为空或已被另一性别使用", "Failed to create %s; the name may be empty or used by the other gender")
		return
	}
	c.JSON(http.StatusCreated, dbpkg.JavIdolSummary{
		ID:           idol.ID,
		Gender:       idol.Gender,
		Name:         idol.Name,
		RomanName:    idol.RomanName,
		JapaneseName: idol.JapaneseName,
		ChineseName:  idol.ChineseName,
	})
}

func parseJavIdolFilters(c *gin.Context) (dbpkg.JavIdolFilters, bool) {
	filters := dbpkg.JavIdolFilters{}
	ranges := []struct {
		key     string
		labelCN string
		labelEN string
		minimum int
		maximum int
		target  *dbpkg.JavIdolIntRange
	}{
		{key: "height", labelCN: "身高", labelEN: "height", minimum: 130, maximum: 190, target: &filters.Height},
		{key: "age", labelCN: "年龄", labelEN: "age", minimum: 18, maximum: 60, target: &filters.Age},
		{key: "cup", labelCN: "罩杯", labelEN: "cup", minimum: 1, maximum: 11, target: &filters.Cup},
		{key: "bust", labelCN: "胸围", labelEN: "bust", minimum: 60, maximum: 130, target: &filters.Bust},
		{key: "waist", labelCN: "腰围", labelEN: "waist", minimum: 45, maximum: 100, target: &filters.Waist},
		{key: "hips", labelCN: "臀围", labelEN: "hips", minimum: 65, maximum: 130, target: &filters.Hips},
	}
	for _, item := range ranges {
		parsed, ok := parseJavIdolIntRange(c, item.key, item.labelCN, item.labelEN, item.minimum, item.maximum)
		if !ok {
			return filters, false
		}
		*item.target = parsed
	}
	return filters, true
}

func parseJavIdolIntRange(c *gin.Context, key, labelCN, labelEN string, minimum, maximum int) (dbpkg.JavIdolIntRange, bool) {
	result := dbpkg.JavIdolIntRange{}
	minParam := strings.TrimSpace(c.Query("idol_" + key + "_min"))
	maxParam := strings.TrimSpace(c.Query("idol_" + key + "_max"))
	if minParam == "" && maxParam == "" {
		return result, true
	}
	if minParam == "" || maxParam == "" {
		respondLocalizedError(c, http.StatusBadRequest, labelCN+"范围无效", "Invalid idol "+labelEN+" range")
		return result, false
	}
	parsedMin, minErr := strconv.Atoi(minParam)
	parsedMax, maxErr := strconv.Atoi(maxParam)
	if minErr != nil || maxErr != nil || parsedMin < minimum || parsedMax > maximum || parsedMin > parsedMax {
		respondLocalizedError(c, http.StatusBadRequest, labelCN+"范围无效", "Invalid idol "+labelEN+" range")
		return result, false
	}
	result.Min = &parsedMin
	result.Max = &parsedMax
	return result, true
}

func listJavIdolOptions(c *gin.Context) {
	limit := queryInt(c, "limit", 100)
	offset := queryInt(c, "offset", 0)
	search := strings.TrimSpace(c.Query("search"))

	items, total, err := dbpkg.Default().ListJavIdolOptions(
		c.Request.Context(),
		search,
		limit,
		offset,
		nil,
		actorGender(c),
	)
	if err != nil {
		logging.Error("list jav actor options: %v", err)
		respondActorError(c, http.StatusInternalServerError, "加载%s选项失败", "Failed to load %s options")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
	})
}

func resolveJavIdols(c *gin.Context) {
	ids := parseInt64CSV(c.Query("ids"))
	gender := actorGender(c)
	items, err := dbpkg.Default().ResolveJavIdols(c.Request.Context(), ids, gender)
	if err != nil {
		logging.Error("resolve jav actors gender=%s: %v", gender, err)
		respondActorError(c, http.StatusInternalServerError, "解析%s信息失败", "Failed to resolve %s information")
		return
	}
	if items == nil {
		items = []dbpkg.JavIdolSummary{}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func mergeJavIdols(c *gin.Context) {
	var req struct {
		CanonicalID int64   `json:"canonical_id"`
		MergeIDs    []int64 `json:"merge_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "合并女优请求无效", "Invalid idol merge request")
		return
	}
	if req.CanonicalID <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "主女优 ID 不能为空", "Canonical idol ID is required")
		return
	}
	if len(req.MergeIDs) == 0 {
		respondActorError(c, http.StatusBadRequest, "待合并%s ID 不能为空", "%s IDs to merge are required")
		return
	}
	if !requireActorGender(c, req.CanonicalID, actorGender(c)) {
		return
	}

	item, err := dbpkg.Default().MergeJavIdols(c.Request.Context(), req.CanonicalID, req.MergeIDs, nil)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondActorError(c, http.StatusNotFound, "%s不存在", "%s was not found")
			return
		}
		logging.Error("merge jav actors canonical=%d merge=%v: %v", req.CanonicalID, req.MergeIDs, err)
		respondActorError(c, http.StatusBadRequest, "合并%s失败，请检查所选%s是否有效或性别是否一致", "Failed to merge %ss; check the selected %ss and their genders")
		return
	}
	items := []dbpkg.JavIdolSummary{*item}
	enrichJavIdolSummaries(c.Request.Context(), items)
	c.JSON(http.StatusOK, items[0])
}

func updateJavIdol(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "女优 ID 无效", "Invalid idol ID")
		return
	}

	var req struct {
		Name         string   `json:"name"`
		RomanName    string   `json:"roman_name"`
		JapaneseName string   `json:"japanese_name"`
		ChineseName  string   `json:"chinese_name"`
		HeightCM     *int     `json:"height_cm"`
		BirthDate    *string  `json:"birth_date"`
		Bust         *int     `json:"bust"`
		Waist        *int     `json:"waist"`
		Hips         *int     `json:"hips"`
		Cup          *int     `json:"cup"`
		Aliases      []string `json:"aliases"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondActorError(c, http.StatusBadRequest, "修改%s信息请求无效", "Invalid %s update request")
		return
	}
	gender := actorGender(c)
	if !requireActorGender(c, id, gender) {
		return
	}

	var birthDate *time.Time
	if req.BirthDate != nil {
		raw := strings.TrimSpace(*req.BirthDate)
		if raw != "" {
			parsed, err := time.Parse("2006-01-02", raw)
			if err != nil {
				respondLocalizedError(c, http.StatusBadRequest, "出生日期格式必须为 YYYY-MM-DD", "Birth date must use the YYYY-MM-DD format")
				return
			}
			birthDate = &parsed
		}
	}

	item, err := dbpkg.Default().UpdateJavIdol(c.Request.Context(), id, dbpkg.JavIdolUpdateInput{
		Name:           req.Name,
		RomanName:      req.RomanName,
		JapaneseName:   req.JapaneseName,
		ChineseName:    req.ChineseName,
		HeightCM:       req.HeightCM,
		BirthDate:      birthDate,
		Bust:           req.Bust,
		Waist:          req.Waist,
		Hips:           req.Hips,
		Cup:            req.Cup,
		Aliases:        req.Aliases,
		ExpectedGender: gender,
	}, nil)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondActorError(c, http.StatusNotFound, "%s不存在", "%s was not found")
			return
		}
		logging.Error("update jav actor id=%d: %v", id, err)
		respondActorError(c, http.StatusBadRequest, "保存%s信息失败", "Failed to save %s information")
		return
	}
	items := []dbpkg.JavIdolSummary{*item}
	enrichJavIdolSummaries(c.Request.Context(), items)
	c.JSON(http.StatusOK, items[0])
}

func listJavIdolCoverOptions(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondActorError(c, http.StatusBadRequest, "%s ID 无效", "Invalid %s ID")
		return
	}
	if !requireActorGender(c, id, actorGender(c)) {
		return
	}

	options, err := dbpkg.Default().ListIdolCoverOptions(
		c.Request.Context(),
		id,
		nil,
	)
	if err != nil {
		logging.Error("list jav actor cover options id=%d: %v", id, err)
		respondActorError(c, http.StatusInternalServerError, "加载%s封面选项失败", "Failed to load %s cover options")
		return
	}
	if options == nil {
		options = []dbpkg.JavIdolCoverOption{}
	}
	c.JSON(http.StatusOK, gin.H{"items": options})
}

func updateJavIdolCover(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondActorError(c, http.StatusBadRequest, "%s ID 无效", "Invalid %s ID")
		return
	}
	if !requireActorGender(c, id, actorGender(c)) {
		return
	}

	var req struct {
		JavID    int64   `json:"jav_id"`
		CropLeft float64 `json:"crop_left"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondActorError(c, http.StatusBadRequest, "更新%s封面请求无效", "Invalid %s cover update request")
		return
	}
	if math.IsNaN(req.CropLeft) || math.IsInf(req.CropLeft, 0) {
		respondLocalizedError(c, http.StatusBadRequest, "封面裁剪位置无效", "Invalid cover crop position")
		return
	}

	item, err := dbpkg.Default().UpdateJavIdolCoverSelection(
		c.Request.Context(),
		id,
		req.JavID,
		req.CropLeft,
		nil,
	)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondActorError(c, http.StatusNotFound, "%s或封面作品不存在", "%s or cover item was not found")
			return
		}
		logging.Error("update jav actor cover id=%d: %v", id, err)
		respondActorError(c, http.StatusBadRequest, "保存%s封面失败", "Failed to save %s cover")
		return
	}

	items := []dbpkg.JavIdolSummary{*item}
	enrichJavIdolSummaries(c.Request.Context(), items)
	c.JSON(http.StatusOK, items[0])
}

// javIdolCoverCode is the cover file code for an actor's own uploaded or
// URL-downloaded cover. It lives in the shared JAV cover directory so the
// existing /jav/:code/cover endpoint serves it without extra plumbing.
func javIdolCoverCode(id int64) string {
	return fmt.Sprintf("idol-%d", id)
}

const maxActorCoverBytes = 12 << 20

// actorCoverExt maps an upload to an extension the cover package can decode.
// WebP is rejected because the package only registers JPEG and PNG decoders.
func actorCoverExt(filename, contentType string) string {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	switch {
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "jpeg"), strings.Contains(ct, "jpg"):
		return ".jpg"
	}
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".png":
		return ".png"
	case ".jpg", ".jpeg":
		return ".jpg"
	}
	return ""
}

func removeActorCoverVariants(coverDir, code, keepExt string) {
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp"} {
		if ext == keepExt {
			continue
		}
		_ = os.Remove(filepath.Join(coverDir, code+ext))
	}
}

// saveActorCoverFile validates an uploaded image and stores it as <code><ext>
// so that cover.FindPath resolves it on the next request.
func saveActorCoverFile(coverDir, code string, file *multipart.FileHeader) error {
	ext := actorCoverExt(file.Filename, file.Header.Get("Content-Type"))
	if ext == "" {
		return errors.New("unsupported image type")
	}
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	if err := os.MkdirAll(coverDir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(coverDir, "actor-cover-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	decoded, err := os.Open(tmpName)
	if err != nil {
		return err
	}
	img, _, decodeErr := image.Decode(decoded)
	decoded.Close()
	if decodeErr != nil || img.Bounds().Empty() {
		return errors.New("invalid image")
	}

	dst := filepath.Join(coverDir, code+ext)
	removeActorCoverVariants(coverDir, code, "")
	if err := os.Rename(tmpName, dst); err != nil {
		return err
	}
	return nil
}

func actorCoverID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondActorError(c, http.StatusBadRequest, "%s ID 无效", "Invalid %s ID")
		return 0, false
	}
	return id, true
}

func respondActorCoverSaved(c *gin.Context, id int64, code string) {
	item, err := dbpkg.Default().SetJavIdolCoverImage(c.Request.Context(), id, code)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondActorError(c, http.StatusNotFound, "%s不存在", "%s was not found")
			return
		}
		logging.Error("save jav actor cover id=%d: %v", id, err)
		respondActorError(c, http.StatusInternalServerError, "保存%s封面失败", "Failed to save %s cover")
		return
	}
	items := []dbpkg.JavIdolSummary{*item}
	enrichJavIdolSummaries(c.Request.Context(), items)
	c.JSON(http.StatusOK, items[0])
}

// uploadJavIdolCover stores an image picked from the local disk as the actor
// cover, mirroring the URL based flow but without any network access.
func uploadJavIdolCover(c *gin.Context) {
	id, ok := actorCoverID(c)
	if !ok {
		return
	}
	if !requireActorGender(c, id, actorGender(c)) {
		return
	}
	cfg := common.CurrentConfig()
	if cfg == nil {
		respondLocalizedError(c, http.StatusInternalServerError, "应用配置尚未加载", "Application configuration is not loaded")
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "请选择要上传的封面图片", "Choose a cover image to upload")
		return
	}
	if file.Size <= 0 || file.Size > maxActorCoverBytes {
		respondLocalizedError(c, http.StatusBadRequest, "封面图片需小于 12MB", "The cover image must be smaller than 12MB")
		return
	}
	code := javIdolCoverCode(id)
	if err := saveActorCoverFile(cfg.JavCoverDir, code, file); err != nil {
		logging.Error("upload jav actor cover id=%d: %v", id, err)
		respondLocalizedError(c, http.StatusBadRequest, "保存封面图片失败，请上传 JPG 或 PNG 图片", "Failed to save the cover image; upload a JPG or PNG")
		return
	}
	respondActorCoverSaved(c, id, code)
}

// updateJavIdolCoverURL downloads an image URL into the local cover directory
// and points the actor at it, matching the JAV edit "cover URL" behaviour.
func updateJavIdolCoverURL(c *gin.Context) {
	id, ok := actorCoverID(c)
	if !ok {
		return
	}
	if !requireActorGender(c, id, actorGender(c)) {
		return
	}
	cfg := common.CurrentConfig()
	if cfg == nil {
		respondLocalizedError(c, http.StatusInternalServerError, "应用配置尚未加载", "Application configuration is not loaded")
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondActorError(c, http.StatusBadRequest, "更新%s封面请求无效", "Invalid %s cover update request")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()
	code := javIdolCoverCode(id)
	if err := cover.DownloadFromURL(ctx, cfg.JavCoverDir, code, req.URL); err != nil {
		logging.Error("download jav actor cover id=%d: %v", id, err)
		respondLocalizedError(c, http.StatusBadRequest, "下载封面失败，请检查图片地址（仅支持 JPG/PNG）", "Failed to download the cover; check the image URL (JPG/PNG only)")
		return
	}
	respondActorCoverSaved(c, id, code)
}

func getJavIdol(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondActorError(c, http.StatusBadRequest, "%s ID 无效", "Invalid %s ID")
		return
	}
	if !requireActorGender(c, id, actorGender(c)) {
		return
	}

	item, err := dbpkg.Default().GetJavIdolSummary(c.Request.Context(), id, nil)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondActorError(c, http.StatusNotFound, "%s不存在", "%s was not found")
			return
		}
		logging.Error("get jav actor id=%d: %v", id, err)
		respondActorError(c, http.StatusInternalServerError, "加载%s信息失败", "Failed to load %s information")
		return
	}

	items := []dbpkg.JavIdolSummary{*item}
	enrichJavIdolSummaries(c.Request.Context(), items)
	c.JSON(http.StatusOK, items[0])
}

func getJavIdolJavDBURL(c *gin.Context) {
	code := strings.TrimSpace(c.Query("code"))
	name := strings.TrimSpace(c.Query("name"))
	if code == "" || name == "" {
		respondLocalizedError(c, http.StatusBadRequest, "番号和女优名称不能为空", "JAV code and idol name are required")
		return
	}

	profileURL, err := scrape.LookupActressURLByCodeAndName(c.Request.Context(), code, name, scrape.ProviderJavDB)
	if err != nil {
		if errors.Is(err, scrape.ErrNotFound) {
			respondLocalizedError(c, http.StatusNotFound, "未找到对应的 JavDB 女优页面", "JavDB idol page was not found")
			return
		}
		logging.Error("lookup javdb actress url code=%s name=%s: %v", code, name, err)
		respondLocalizedError(c, http.StatusInternalServerError, "查询 JavDB 女优页面失败", "Failed to look up the JavDB idol page")
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": profileURL})
}

func enrichJavIdolSummaries(ctx context.Context, items []dbpkg.JavIdolSummary) {
	cfg := common.CurrentConfig()
	coverDir := ""
	if cfg != nil {
		coverDir = cfg.JavCoverDir
	}
	for i := range items {
		enrichJavIdolSummary(ctx, &items[i], coverDir)
	}
}

func enrichJavIdolSummary(ctx context.Context, item *dbpkg.JavIdolSummary, coverDir string) {
	item.Name = strings.TrimSpace(item.Name)
	item.RomanName = strings.TrimSpace(item.RomanName)
	item.JapaneseName = strings.TrimSpace(item.JapaneseName)
	item.ChineseName = strings.TrimSpace(item.ChineseName)
	item.CoverCode = strings.TrimSpace(item.CoverCode)

	// A cover the user uploaded or downloaded from a URL wins over any
	// work-derived cover so the card shows exactly what the user picked.
	if item.CoverImage != "" {
		item.CoverCode = item.CoverImage
		return
	}

	// The summary only supplies a cover for a manual selection or a solo work.
	// Keep it empty when neither is available instead of falling back to group works.
	if coverDir == "" || item.CoverCode == "" {
		return
	}
	if item.CoverJavID != nil && item.CoverCode != "" {
		if !cover.Exists(item.CoverCode) {
			cover.Enqueue(item.CoverCode)
		}
		return
	}
	if item.CoverCode != "" {
		if _, ok := cover.FindPath(coverDir, item.CoverCode); ok {
			return
		}
	}
	codes, err := dbpkg.Default().ListIdolCoverCodes(ctx, item.ID, nil)
	if err != nil {
		logging.Error("list idol cover codes id=%d: %v", item.ID, err)
		return
	}
	var chosen string
	for _, code := range codes {
		if _, ok := cover.FindPath(coverDir, code); ok {
			chosen = code
			break
		}
	}
	if chosen == "" && len(codes) > 0 {
		chosen = codes[0]
	}
	if chosen != "" {
		item.CoverCode = chosen
		if !cover.Exists(chosen) {
			cover.Enqueue(chosen)
		}
	}
}
