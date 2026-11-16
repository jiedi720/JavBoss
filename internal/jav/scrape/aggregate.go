package scrape

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// priorityMu guards the runtime priority configuration injected from the app config.
var (
	priorityMu      sync.RWMutex
	priorityConfigs = map[string]string{}
)

// SetPriorityConfig replaces the runtime scrape priority configuration. Keys are
// ConfigKeyForCode(category) values; unknown keys are ignored on lookup.
func SetPriorityConfig(cfg map[string]string) {
	priorityMu.Lock()
	defer priorityMu.Unlock()
	next := make(map[string]string, len(cfg))
	for k, v := range cfg {
		next[k] = v
	}
	priorityConfigs = next
}

func priorityConfigFor(category string) string {
	priorityMu.RLock()
	defer priorityMu.RUnlock()
	return priorityConfigs[ConfigKeyForCode(category)]
}

// lookupSource pairs a configured source id with the provider that can query it.
type lookupSource struct {
	id       string
	provider Provider
}

// sourcesForPriority turns a priority list into usable lookup sources,
// de-duplicating providers and skipping placeholder sources.
func sourcesForPriority(list []string) []lookupSource {
	out := make([]lookupSource, 0, len(list))
	seen := make(map[Provider]bool, len(list))
	for _, id := range list {
		p, ok := ProviderForSource(id)
		if !ok || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, lookupSource{id: id, provider: p})
	}
	return out
}

// orderedSourcesForCode returns the sources to query for a code in priority
// order. Specialised providers (FC2 API, JavMenu, ...) are placed first so
// their fields win priority and the aggregate path never loses the coverage of
// the original resolver.
func orderedSourcesForCode(code string) []lookupSource {
	category := ClassifyCode(code)
	configSources := sourcesForPriority(ResolvePriority(category, priorityConfigFor(category)))
	seen := make(map[Provider]bool, len(configSources))
	for _, s := range configSources {
		seen[s.provider] = true
	}

	// Specialised providers lead the list; their fields take priority.
	special := make([]lookupSource, 0, 2)
	for _, p := range providersForCode(code) {
		if p == ProviderUnknown || seen[p] {
			continue
		}
		seen[p] = true
		special = append(special, lookupSource{id: p.String(), provider: p})
	}
	return append(special, configSources...)
}

type sourceResult struct {
	source string
	info   *JavInfo
	err    error
}

// AggregateResolve queries every source in parallel and merges the returned
// metadata by source priority. The first non-empty value wins for scalar
// fields; list fields are combined and de-duplicated. JavDB covers are used
// only as a fallback because they usually carry a watermark.
func (c *MetadataClient) AggregateResolve(ctx context.Context, code string, sources []lookupSource) (*JavInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, ErrNotFound
	}

	resultsCh := make(chan sourceResult, len(sources))
	var wg sync.WaitGroup
	for _, s := range sources {
		wg.Add(1)
		go func(s lookupSource) {
			defer wg.Done()
			info, err := c.LookupJavByCode(ctx, code, s.provider)
			resultsCh <- sourceResult{source: s.id, info: info, err: err}
		}(s)
	}
	wg.Wait()
	close(resultsCh)

	bySource := make(map[string]*JavInfo, len(sources))
	var failures []error
	for r := range resultsCh {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if r.err != nil {
			if !errors.Is(r.err, ErrNotFound) {
				failures = append(failures, fmt.Errorf("aggregate source=%s code=%s: %w", r.source, code, r.err))
			}
			continue
		}
		if r.info != nil {
			bySource[r.source] = r.info
		}
	}
	if len(bySource) == 0 {
		if len(failures) > 0 {
			return nil, errors.Join(failures...)
		}
		return nil, ErrNotFound
	}

	return mergeJavInfos(code, sources, bySource), nil
}

// mergeJavInfos fills each field from the highest-priority source that has a
// value. Cover URLs from JavDB are kept as a fallback to avoid watermarks.
func mergeJavInfos(code string, sources []lookupSource, bySource map[string]*JavInfo) *JavInfo {
	merged := &JavInfo{Code: code}
	var coverFallback string
	for _, s := range sources {
		info := bySource[s.id]
		if info == nil {
			continue
		}
		if merged.Provider == ProviderUnknown {
			merged.Provider = info.Provider
			if info.Code != "" {
				merged.Code = info.Code
			}
		}
		if merged.Title == "" {
			merged.Title = info.Title
		}
		if merged.ZhTitle == "" {
			merged.ZhTitle = info.ZhTitle
		}
		if merged.Studio == "" {
			merged.Studio = info.Studio
		}
		if merged.Series == "" {
			merged.Series = info.Series
		}
		if merged.ReleaseUnix == 0 {
			merged.ReleaseUnix = info.ReleaseUnix
		}
		if merged.DurationMin == 0 {
			merged.DurationMin = info.DurationMin
		}
		if merged.IsUncensored == nil {
			merged.IsUncensored = info.IsUncensored
		}
		merged.Tags = mergeUnique(merged.Tags, info.Tags)
		merged.Actors = mergeUnique(merged.Actors, info.Actors)
		merged.SampleImages = mergeSampleImages(merged.SampleImages, info.SampleImages)
		if info.CoverURL != "" {
			if strings.EqualFold(s.id, SourceJavDB) {
				if coverFallback == "" {
					coverFallback = info.CoverURL
				}
			} else if merged.CoverURL == "" {
				merged.CoverURL = info.CoverURL
			}
		}
	}
	if merged.CoverURL == "" {
		merged.CoverURL = coverFallback
	}
	return merged
}

func mergeUnique(dst, src []string) []string {
	seen := make(map[string]bool, len(dst)+len(src))
	for _, v := range dst {
		seen[strings.TrimSpace(v)] = true
	}
	for _, v := range src {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		dst = append(dst, v)
	}
	return dst
}

func mergeSampleImages(dst, src []SampleImage) []SampleImage {
	seen := make(map[string]bool, len(dst)+len(src))
	for _, v := range dst {
		seen[v.DetailURL] = true
	}
	for _, v := range src {
		if v.DetailURL == "" || seen[v.DetailURL] {
			continue
		}
		seen[v.DetailURL] = true
		dst = append(dst, v)
	}
	return dst
}

// resolveAggregated resolves a single code through the multi-source aggregator.
func (c *MetadataClient) resolveAggregated(ctx context.Context, code string) (*JavInfo, error) {
	return c.AggregateResolve(ctx, code, orderedSourcesForCode(code))
}
