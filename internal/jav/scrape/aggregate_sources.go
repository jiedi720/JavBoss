package scrape

import (
	"regexp"
	"strings"
)

// Scrape source identifiers used by the global priority configuration.
// These are stable config values; only sources backed by a real provider are
// queried, the rest are placeholders kept so the configured order matches the
// reference priority list and can light up once a crawler is added.
const (
	SourceDmm        = "dmm"
	SourceMgstage    = "mgstage"
	SourceJavlibrary = "javlibrary"
	SourceAvbase     = "avbase"
	SourceHboxJP     = "hbox_jp"
	SourceJavDB      = "javdb"
	SourceJavBus     = "javbus"
	SourceJav321     = "jav321"
	SourceAvmoo      = "avmoo"
	SourceMmtv       = "mmtv"
	SourceAirav      = "airav_io"
	SourceFreejavbt  = "freejavbt"
	SourceMissAV     = "miss_av"
	SourceCarib      = "carib"
	SourceAvsox      = "avsox"
	SourceFc2        = "fc2"
	SourceFc2Hub     = "fc2_hub"
)

// Code categories map to one priority list each.
const (
	CodeCensored   = "censored"
	CodeUncensored = "uncensored"
	CodeAmateur    = "amateur"
	CodeFC2        = "fc2"
)

// DefaultPriority is the reference order shown in the settings UI. Sources
// without a backing provider are queried as no-ops until implemented.
var DefaultPriority = map[string][]string{
	CodeCensored:   {SourceDmm, SourceMgstage, SourceJavlibrary, SourceAvbase, SourceHboxJP, SourceJavDB, SourceJavBus, SourceJav321, SourceAvmoo, SourceMmtv, SourceAirav, SourceFreejavbt, SourceMissAV},
	CodeUncensored: {SourceCarib, SourceAvbase, SourceJavBus, SourceJavDB, SourceAvsox, SourceMmtv, SourceAirav, SourceFreejavbt, SourceMissAV},
	CodeAmateur:    {SourceMgstage, SourceCarib, SourceJavlibrary, SourceAvsox, SourceAvmoo, SourceJavBus, SourceJavDB, SourceJav321, SourceMmtv, SourceAirav, SourceFreejavbt, SourceMissAV},
	CodeFC2:        {SourceFc2, SourceFc2Hub, SourceJavDB, SourceAvsox, SourceMmtv, SourceAirav, SourceFreejavbt, SourceMissAV},
}

// AllSourceIDs lists every known source in display order.
var AllSourceIDs = []string{
	SourceDmm, SourceMgstage, SourceJavlibrary, SourceAvbase, SourceHboxJP,
	SourceJavDB, SourceJavBus, SourceJav321, SourceAvmoo, SourceMmtv,
	SourceAirav, SourceFreejavbt, SourceMissAV, SourceCarib, SourceAvsox,
	SourceFc2, SourceFc2Hub,
}

// ConfigKeyForCode returns the config key that stores one category's priority.
func ConfigKeyForCode(category string) string {
	return "jav_scrape_priority_" + category
}

var (
	fc2CodeRe    = regexp.MustCompile(`(?i)^fc2(?:[-_]?ppv)?[-_]?\d+`)
	pureNumRe    = regexp.MustCompile(`^\d{4,}[-_]\d{2,}`)
	alphaNumRe   = regexp.MustCompile(`(?i)^\d+[a-z]+[-_]\d+`)
	letterCodeRe = regexp.MustCompile(`(?i)^[a-z]{2,6}[-_]?\d{2,5}[a-z]{0,2}$`)
)

// ClassifyCode returns the category for a JAV code.
//
//   - FC2-PPV-1234 / FC2-1234            -> fc2
//   - 030919-047 / 123456_001            -> uncensored (pure numbers)
//   - 259LUXU-1905 / 1pondo-xxx (digit+letters) -> amateur
//   - LUXU-1905 / IPX-228                -> censored (letters + number)
//
// Unknown codes fall back to censored so they still get a usable priority list.
func ClassifyCode(code string) string {
	c := strings.ToUpper(strings.TrimSpace(code))
	if fc2CodeRe.MatchString(c) {
		return CodeFC2
	}
	if pureNumRe.MatchString(c) {
		return CodeUncensored
	}
	if alphaNumRe.MatchString(c) {
		return CodeAmateur
	}
	if letterCodeRe.MatchString(c) {
		return CodeCensored
	}
	return CodeCensored
}

// sourceToProvider maps a source id to the provider that can actually query it.
// Sources absent from this map are placeholders and are skipped at query time.
var sourceToProvider = map[string]Provider{
	SourceJavDB:  ProviderJavDB,
	SourceJavBus: ProviderJavBus,
	SourceAvmoo:  ProviderAvmoo,
	SourceAvsox:  ProviderAvsox,
}

// ProviderForSource resolves a configured source to a usable provider.
func ProviderForSource(source string) (Provider, bool) {
	p, ok := sourceToProvider[strings.ToLower(strings.TrimSpace(source))]
	return p, ok
}

// IsRealSource reports whether a source can currently be queried.
func IsRealSource(source string) bool {
	_, ok := ProviderForSource(source)
	return ok
}

// NormalizePriorityList keeps only known sources, removes duplicates and blanks,
// and always appends any missing real sources so a bad config cannot silently
// disable working providers.
func NormalizePriorityList(list []string) []string {
	known := make(map[string]struct{}, len(AllSourceIDs))
	for _, id := range AllSourceIDs {
		known[id] = struct{}{}
	}
	out := make([]string, 0, len(list))
	seen := make(map[string]struct{}, len(list))
	for _, item := range list {
		id := strings.ToLower(strings.TrimSpace(item))
		if _, ok := known[id]; !ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	for _, id := range AllSourceIDs {
		if IsRealSource(id) {
			if _, dup := seen[id]; !dup {
				out = append(out, id)
			}
		}
	}
	return out
}

// ParsePriorityConfig turns a config value ("javdb,javbus,...") into a list.
func ParsePriorityConfig(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.Split(value, ",")
}

// DefaultPriorityConfig returns the comma-joined default for a category.
func DefaultPriorityConfig(category string) string {
	list := DefaultPriority[category]
	if len(list) == 0 {
		return ""
	}
	return strings.Join(list, ",")
}

// ResolvePriority returns the configured list for a code's category, falling
// back to defaults and normalizing for safety.
func ResolvePriority(category, configured string) []string {
	list := ParsePriorityConfig(configured)
	if len(list) == 0 {
		list = DefaultPriority[category]
	}
	return NormalizePriorityList(list)
}
