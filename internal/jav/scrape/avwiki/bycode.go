package avwiki

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/jav/scrape/metadata"

	"github.com/PuerkitoBio/goquery"
)

// ActressRef is one actress credited on a work page, together with the profile
// link the page exposes. A work sometimes lists an actress without a link, so
// ProfileURL may be empty.
type ActressRef struct {
	Name       string `json:"name"`
	ProfileURL string `json:"profile_url"`
}

const (
	// byCodeMaxWorkPages bounds how many search results are opened when the site
	// answers a code with a result list instead of redirecting to the work page.
	byCodeMaxWorkPages = 3
	byCodeMaxBodyBytes = 4 << 20
	// byCodePageTimeout is longer than the shared provider timeout: this call is
	// user triggered and the site answers noticeably slower than the tag API.
	byCodePageTimeout = 60 * time.Second
)

// LookupActressesByCode lists the actresses credited on a work. AV Wiki's
// product search resolves an exact code to the work page; older codes return a
// result list instead, so the first few work pages are inspected as a fallback.
func (p *AVWikiClient) LookupActressesByCode(ctx context.Context, code string) ([]ActressRef, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, metadata.ErrNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	searchURL := p.baseURL + "/?s=" + url.QueryEscape(code) + "&post_type=product"
	doc, err := p.fetchDocument(ctx, searchURL)
	if err != nil {
		return nil, err
	}
	if refs := collectActressRefs(doc, searchURL); len(refs) > 0 {
		return refs, nil
	}

	var refs []ActressRef
	seen := map[string]bool{}
	for _, page := range collectWorkLinks(doc, searchURL, byCodeMaxWorkPages) {
		if ctx.Err() != nil {
			break
		}
		pageDoc, err := p.fetchDocument(ctx, page)
		if err != nil {
			logging.Info("avwiki: actress page failed url=%s err=%v", page, err)
			continue
		}
		for _, ref := range collectActressRefs(pageDoc, page) {
			key := strings.ToLower(ref.Name)
			if seen[key] {
				continue
			}
			seen[key] = true
			refs = append(refs, ref)
		}
		if len(refs) > 0 {
			break
		}
	}
	if len(refs) == 0 {
		return nil, metadata.ErrNotFound
	}
	return refs, nil
}

func (p *AVWikiClient) fetchDocument(ctx context.Context, target string) (*goquery.Document, error) {
	resp, err := p.requestURL(ctx, target)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, byCodeMaxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("avwiki: read page: %w", err)
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("avwiki: parse page: %w", err)
	}
	return doc, nil
}

func (p *AVWikiClient) requestURL(ctx context.Context, target string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("avwiki: build request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "ja-JP,ja;q=0.9,en;q=0.7")
	if err := p.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	logging.Info("avwiki request: %s", target)
	resp, err := p.pageHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("avwiki: request page: %w", err)
	}
	if resp.Header.Get("cf-mitigated") == "challenge" {
		resp.Body.Close()
		return nil, fmt.Errorf("avwiki: cloudflare browser verification required")
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("avwiki: http %d", resp.StatusCode)
	}
	return resp, nil
}

// pageHTTPClient returns the provider client with a timeout large enough for a
// work page. The client is copied so the shared transport and proxy settings stay
// untouched.
func (p *AVWikiClient) pageHTTPClient() *http.Client {
	if p.httpClient == nil {
		return &http.Client{Timeout: byCodePageTimeout}
	}
	if p.httpClient.Timeout >= byCodePageTimeout {
		return p.httpClient
	}
	cloned := *p.httpClient
	cloned.Timeout = byCodePageTimeout
	return &cloned
}

// collectActressRefs reads the actress block of a work page. The list markup
// changed over time, so the generic profile link is used as a fallback.
func collectActressRefs(doc *goquery.Document, pageURL string) []ActressRef {
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}
	refs := make([]ActressRef, 0, 4)
	seen := map[string]bool{}
	add := func(name, href string) {
		name = strings.Join(strings.Fields(name), " ")
		if name == "" || len([]rune(name)) > 40 {
			return
		}
		if strings.Contains(name, "続きを読む") || strings.Contains(name, "女優情報") {
			return
		}
		key := strings.ToLower(name)
		if seen[key] {
			return
		}
		seen[key] = true
		profileURL := ""
		if href != "" {
			if abs, err := base.Parse(href); err == nil && profilePath.MatchString(abs.Path) {
				abs.RawQuery, abs.Fragment = "", ""
				profileURL = abs.String()
			}
		}
		refs = append(refs, ActressRef{Name: name, ProfileURL: profileURL})
	}
	doc.Find("li.actress-name a, .actress-name a").Each(func(_ int, sel *goquery.Selection) {
		href, _ := sel.Attr("href")
		add(sel.Text(), href)
	})
	if len(refs) == 0 {
		doc.Find(`a[href*="/av-actress/"]`).Each(func(_ int, sel *goquery.Selection) {
			href, _ := sel.Attr("href")
			add(sel.Text(), href)
		})
	}
	return refs
}

// collectWorkLinks returns the work pages of a search result list.
func collectWorkLinks(doc *goquery.Document, pageURL string, limit int) []string {
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}
	links := make([]string, 0, limit)
	seen := map[string]bool{}
	doc.Find(`a[href*="/product/"]`).EachWithBreak(func(_ int, sel *goquery.Selection) bool {
		href, _ := sel.Attr("href")
		abs, err := base.Parse(href)
		if err != nil {
			return true
		}
		abs.RawQuery, abs.Fragment = "", ""
		clean := abs.String()
		if seen[clean] {
			return true
		}
		seen[clean] = true
		links = append(links, clean)
		return len(links) < limit
	})
	return links
}
