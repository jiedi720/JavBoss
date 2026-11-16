package scrape

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"javboss/internal/jav/scrape/javdbapi"
	"javboss/internal/util"
)

func TestResolveFC2FilenameThroughJavDBAPI(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/api/v2/search":
			if got := r.URL.Query().Get("q"); got != "FC2-4810487" {
				t.Errorf("query = %q, want FC2-4810487", got)
			}
			fmt.Fprint(w, `{"success":1,"data":{"movies":[{"id":"wrong","number":"FC2-4810488"},{"id":"match","number":"FC2-4810487"}]}}`)
		case "/api/v4/movies/match":
			fmt.Fprint(w, `{"success":1,"data":{"movie":{"number":"FC2-4810487","origin_title":"Title","cover_url":"https://images.test/fc2.jpg","duration":120,"type":3}}}`)
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := NewMetadataClient(map[Provider]any{ProviderJavDBAPI: javdbapi.New(server.Client(), server.URL)}, nil)
	filename := "FC2PPV-4810487.mp4"
	info, err := client.ResolveJavByCodes(context.Background(), util.ExtractCodeFromName(filename), util.ExtractUncensoredCodesFromName(filename))
	if err != nil {
		t.Fatal(err)
	}
	if info.Code != "FC2-PPV-4810487" || info.Provider != ProviderJavDBAPI || info.Title != "Title" || info.DurationMin != 120 || info.CoverURL != "https://images.test/fc2.jpg" {
		t.Fatalf("unexpected metadata: %+v", info)
	}
	if len(paths) != 2 {
		t.Fatalf("request paths = %v, want search then detail", paths)
	}
}

func TestResolveFC2ProviderFallback(t *testing.T) {
	for _, tc := range []struct {
		name        string
		apiErr      error
		wantResult  Provider
		wantQueried []Provider
	}{
		{"api hit", nil, ProviderJavDBAPI, []Provider{ProviderJavDBAPI, ProviderAvsox}},
		{"api miss", ErrNotFound, ProviderAvsox, []Provider{ProviderJavDBAPI, ProviderAvsox}},
		{"api failure", errors.New("request failed"), ProviderAvsox, []Provider{ProviderJavDBAPI, ProviderAvsox}},
	} {
		for _, forced := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/forced=%t", tc.name, forced), func(t *testing.T) {
				queried := make(map[Provider]bool)
				var mu sync.Mutex
				client := NewMetadataClient(map[Provider]any{
					ProviderJavDBAPI: movieLookupFunc(func(_ context.Context, code string) (*JavInfo, error) {
						mu.Lock()
						queried[ProviderJavDBAPI] = true
						mu.Unlock()
						if code != "FC2-PPV-1234567" {
							t.Fatalf("API input = %q", code)
						}
						if tc.apiErr != nil {
							return nil, tc.apiErr
						}
						return &JavInfo{Code: code, Provider: ProviderJavDBAPI}, nil
					}),
					ProviderAvsox: movieLookupFunc(func(_ context.Context, code string) (*JavInfo, error) {
						mu.Lock()
						queried[ProviderAvsox] = true
						mu.Unlock()
						if code != "FC2-PPV-1234567" {
							t.Fatalf("Avsox input = %q", code)
						}
						return &JavInfo{Code: code, Provider: ProviderAvsox}, nil
					}),
				}, nil)
				filename := "FC2PPV-1234567.mp4"
				codes := util.ExtractCodeFromName(filename)
				uncensoredCodes := util.ExtractUncensoredCodesFromName(filename)
				if forced {
					codes = []string{"FC2-PPV-1234567"}
					uncensoredCodes = codes
				}
				info, err := client.ResolveJavByCodes(context.Background(), codes, uncensoredCodes)
				if err != nil || info == nil || info.Code != "FC2-PPV-1234567" || info.Provider != tc.wantResult {
					t.Fatalf("result = %+v, %v; want provider %s", info, err, tc.wantResult)
				}
				for _, p := range tc.wantQueried {
					mu.Lock()
					got := queried[p]
					mu.Unlock()
					if !got {
						t.Errorf("provider %s was not queried", p)
					}
				}
			})
		}
	}
}
