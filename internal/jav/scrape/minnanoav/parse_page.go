package minnanoav

import (
	"strings"

	"javboss/internal/jav/scrape/internal/htmlutil"
	"javboss/internal/jav/scrape/metadata"
)

// ParseCapturedPage parses HTML that the user's browser fetched on the app's
// behalf. The site blocks server-side requests (Cloudflare), but the browser
// already holds the clearance cookie, so the page is fetched there and the markup
// is handed back here for parsing.
//
// A profile page yields the actress info. A search result page yields the
// candidate profile URL so the caller can ask the browser for one more page.
func ParseCapturedPage(body []byte, pageURL, name string) (*metadata.ActressInfo, string, error) {
	if len(body) == 0 {
		return nil, "", metadata.ErrNotFound
	}
	doc, err := htmlutil.ParseHTMLDocument(body)
	if err != nil {
		return nil, "", err
	}
	pageURL = strings.TrimSpace(pageURL)
	if canonical := canonicalMinnanoAVActressURL(pageURL, minnanoAVBaseURL); canonical != "" {
		info, err := finalizeMinnanoAVActressInfo(
			name,
			canonical,
			parseMinnanoAVActressInfo(doc),
			parseMinnanoAVActressAliases(doc)...,
		)
		if err != nil {
			return nil, "", err
		}
		return info, canonical, nil
	}
	candidate := findMinnanoAVActressSearchResultURL(doc, name, pageURL, minnanoAVBaseURL)
	if candidate == "" {
		return nil, "", metadata.ErrNotFound
	}
	return nil, candidate, nil
}

// SearchURLFor builds the actress search URL used by the browser helper.
func SearchURLFor(name string) (string, error) {
	url, err := buildMinnanoAVActressSearchURL(minnanoAVBaseURL, name)
	if err != nil {
		return "", err
	}
	return url, nil
}
