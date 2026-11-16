package minnanoav

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"javboss/internal/common/logging"
	"javboss/internal/jav/scrape/internal/winfetch"
	"javboss/internal/jav/scrape/metadata"
	"javboss/internal/util"
)

// The site's own search endpoint (search_result.php) sits behind a Cloudflare
// challenge that rejects every server-side client, but actress profile pages
// (/actressNNN.html) are served normally. So instead of asking a browser for the
// page, the provider finds the profile URL through a public search engine and
// then reads that page over plain HTTP.
const (
	minnanoAVSearchEngineURL = "https://search.yahoo.co.jp/search"
	// Three candidates is enough to absorb a stale/duplicate result without
	// hammering the site.
	minnanoAVDiscoveryCandidates = 3
)

var minnanoAVDiscoveryActressPattern = regexp.MustCompile(`minnano-av\.com/actress(\d+)\.html`)

// ErrUnavailable reports that no transport could reach the site at all, which is
// different from "this actress does not exist".
var ErrUnavailable = errors.New("minnanoav: site unreachable")

func buildMinnanoAVDiscoveryURL(name string) string {
	query := url.Values{}
	query.Set("p", "site:minnano-av.com "+strings.TrimSpace(name))
	return minnanoAVSearchEngineURL + "?" + query.Encode()
}

// lookupMinnanoAVActressByDiscovery resolves a name to a profile page via a web
// search, then parses that page with the same parser used everywhere else.
func (p *MinnanoAVClient) lookupMinnanoAVActressByDiscovery(ctx context.Context, name string) (*metadata.ActressInfo, error) {
	searchURL := buildMinnanoAVDiscoveryURL(name)
	body, status, err := p.fetchMinnanoAVBytes(ctx, searchURL, "")
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("minnanoav: discovery http %d", status)
	}
	candidates := extractMinnanoAVActressURLs(body, minnanoAVDiscoveryCandidates)
	if len(candidates) == 0 {
		return nil, metadata.ErrNotFound
	}

	var lastErr error
	for _, candidate := range candidates {
		page, pageStatus, fetchErr := p.fetchMinnanoAVBytes(ctx, candidate, searchURL)
		if fetchErr != nil {
			lastErr = fetchErr
			continue
		}
		if pageStatus != http.StatusOK {
			lastErr = fmt.Errorf("minnanoav: profile http %d", pageStatus)
			continue
		}
		info, _, parseErr := ParseCapturedPage(page, candidate, name)
		if parseErr == nil && info != nil {
			logging.Info("minnanoav discovery ok: %s -> %s", name, candidate)
			return info, nil
		}
		lastErr = parseErr
	}
	if lastErr == nil || errors.Is(lastErr, metadata.ErrNotFound) {
		return nil, metadata.ErrNotFound
	}
	return nil, lastErr
}

func extractMinnanoAVActressURLs(body []byte, limit int) []string {
	if limit <= 0 {
		return nil
	}
	seen := make(map[string]struct{}, limit)
	urls := make([]string, 0, limit)
	for _, match := range minnanoAVDiscoveryActressPattern.FindAllSubmatch(body, -1) {
		target := minnanoAVBaseURL + "/actress" + string(match[1]) + ".html"
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}
		urls = append(urls, target)
		if len(urls) >= limit {
			break
		}
	}
	return urls
}

// fetchMinnanoAVBytes reads a page with plain HTTP first and falls back to the
// Windows curl bridge, which is the only transport that works when the backend
// runs inside WSL behind a proxy.
func (p *MinnanoAVClient) fetchMinnanoAVBytes(ctx context.Context, targetURL, referer string) ([]byte, int, error) {
	var firstErr error
	if p.httpClient != nil {
		if err := p.limiter.Wait(ctx); err != nil {
			return nil, 0, err
		}
		req, err := buildMinnanoAVRequest(ctx, targetURL, referer)
		if err != nil {
			firstErr = err
		} else {
			resp, doErr := p.httpClient.Do(req)
			if doErr != nil {
				firstErr = doErr
			} else {
				body, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
				resp.Body.Close()
				switch {
				case readErr != nil:
					firstErr = readErr
				case resp.StatusCode == http.StatusOK && !minnanoAVLooksBlocked(body):
					return body, resp.StatusCode, nil
				default:
					firstErr = fmt.Errorf("minnanoav: http %d", resp.StatusCode)
				}
			}
		}
	}
	if winfetch.Available() {
		status, body, err := winfetch.Get(ctx, targetURL, util.ProxyURL())
		if err == nil && status == http.StatusOK && !minnanoAVLooksBlocked(body) {
			return body, status, nil
		}
		if firstErr == nil {
			if err != nil {
				firstErr = err
			} else {
				firstErr = fmt.Errorf("minnanoav: http %d", status)
			}
		}
	}
	if firstErr == nil {
		firstErr = ErrUnavailable
	}
	return nil, 0, firstErr
}

// minnanoAVLooksBlocked detects the Cloudflare interstitial that replaces the
// real page.
func minnanoAVLooksBlocked(body []byte) bool {
	text := string(body)
	if len(text) > 20000 && (strings.Contains(text, "生年月日") || strings.Contains(text, "検索結果")) {
		return false
	}
	return strings.Contains(text, "Just a moment") ||
		strings.Contains(text, "challenge-platform") ||
		strings.Contains(text, "请稍候")
}
