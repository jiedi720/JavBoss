package scrape

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func withTestPriority(t *testing.T) {
	t.Helper()
	SetPriorityConfig(map[string]string{
		ConfigKeyForCode(CodeCensored):   "javbus,javdb",
		ConfigKeyForCode(CodeUncensored): "javbus,javdb",
		ConfigKeyForCode(CodeAmateur):    "javbus,javdb",
		ConfigKeyForCode(CodeFC2):        "javbus,javdb",
	})
	t.Cleanup(func() { SetPriorityConfig(nil) })
}

func TestResolveJavByCodesAggregates(t *testing.T) {
	withTestPriority(t)
	client := NewMetadataClient(map[Provider]any{
		ProviderJavBus: movieLookupFunc(func(_ context.Context, code string) (*JavInfo, error) {
			return &JavInfo{Code: code, Provider: ProviderJavBus, Title: "bus title", Tags: []string{"A"}, CoverURL: "bus"}, nil
		}),
		ProviderJavDB: movieLookupFunc(func(_ context.Context, code string) (*JavInfo, error) {
			return &JavInfo{Code: code, Provider: ProviderJavDB, Studio: "db studio", Actors: []string{"X"}, CoverURL: "db"}, nil
		}),
		ProviderAvmoo: movieLookupFunc(func(context.Context, string) (*JavInfo, error) { return nil, ErrNotFound }),
		ProviderAvsox: movieLookupFunc(func(context.Context, string) (*JavInfo, error) { return nil, ErrNotFound }),
	}, nil)
	info, err := client.ResolveJavByCodes(context.Background(), []string{"IPX-228"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "bus title" || info.Studio != "db studio" {
		t.Fatalf("Title/Studio = %q/%q", info.Title, info.Studio)
	}
	if !reflect.DeepEqual(info.Tags, []string{"A"}) || !reflect.DeepEqual(info.Actors, []string{"X"}) {
		t.Fatalf("Tags/Actors = %v/%v", info.Tags, info.Actors)
	}
	if info.CoverURL != "bus" {
		t.Fatalf("CoverURL = %q, want bus (JavDB fallback)", info.CoverURL)
	}
}

func TestResolveJavByCodesAdvancesOnNotFound(t *testing.T) {
	withTestPriority(t)
	client := NewMetadataClient(map[Provider]any{
		ProviderJavBus: movieLookupFunc(func(context.Context, string) (*JavInfo, error) { return nil, ErrNotFound }),
		ProviderJavDB: movieLookupFunc(func(_ context.Context, code string) (*JavInfo, error) {
			if code != "SECOND-002" {
				return nil, ErrNotFound
			}
			return &JavInfo{Code: code, Provider: ProviderJavDB, Title: "second code"}, nil
		}),
		ProviderAvmoo: movieLookupFunc(func(context.Context, string) (*JavInfo, error) { return nil, ErrNotFound }),
		ProviderAvsox: movieLookupFunc(func(context.Context, string) (*JavInfo, error) { return nil, ErrNotFound }),
	}, nil)
	info, err := client.ResolveJavByCodes(context.Background(), []string{"FIRST-001", "SECOND-002"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if info == nil || info.Title != "second code" || info.Code != "SECOND-002" {
		t.Fatalf("info = %+v", info)
	}
}

func TestResolveJavByCodesRetainsFailures(t *testing.T) {
	withTestPriority(t)
	requestErr := errors.New("request failed")
	client := NewMetadataClient(map[Provider]any{
		ProviderJavBus: movieLookupFunc(func(context.Context, string) (*JavInfo, error) { return nil, requestErr }),
		ProviderJavDB:  movieLookupFunc(func(context.Context, string) (*JavInfo, error) { return nil, ErrNotFound }),
		ProviderAvmoo:  movieLookupFunc(func(context.Context, string) (*JavInfo, error) { return nil, ErrNotFound }),
		ProviderAvsox:  movieLookupFunc(func(context.Context, string) (*JavInfo, error) { return nil, ErrNotFound }),
	}, nil)
	info, err := client.ResolveJavByCodes(context.Background(), []string{"IPX-228"}, nil)
	if info != nil {
		t.Fatalf("info = %+v", info)
	}
	if !errors.Is(err, requestErr) || errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want retained request failure", err)
	}
	if !strings.Contains(err.Error(), "source=javbus") {
		t.Fatalf("err = %v, want source context", err)
	}
}

func TestResolveJavByCodesAllNotFound(t *testing.T) {
	withTestPriority(t)
	client := NewMetadataClient(map[Provider]any{
		ProviderJavBus: movieLookupFunc(func(context.Context, string) (*JavInfo, error) { return nil, ErrNotFound }),
		ProviderJavDB:  movieLookupFunc(func(context.Context, string) (*JavInfo, error) { return nil, ErrNotFound }),
		ProviderAvmoo:  movieLookupFunc(func(context.Context, string) (*JavInfo, error) { return nil, ErrNotFound }),
		ProviderAvsox:  movieLookupFunc(func(context.Context, string) (*JavInfo, error) { return nil, ErrNotFound }),
	}, nil)
	info, err := client.ResolveJavByCodes(context.Background(), []string{"IPX-228"}, nil)
	if info != nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("result = %+v, %v", info, err)
	}
}

func TestResolveJavByCodesEmptyCandidates(t *testing.T) {
	client := NewMetadataClient(map[Provider]any{}, nil)
	info, err := client.ResolveJavByCodes(context.Background(), nil, nil)
	if info != nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("result = %+v, %v; want not found", info, err)
	}
}

func TestResolveJavByCodesUncensoredCandidatesOnly(t *testing.T) {
	want := &JavInfo{Code: "123456_001", Provider: ProviderAvsox}
	client := NewMetadataClient(map[Provider]any{
		ProviderAvsox: movieLookupFunc(func(_ context.Context, code string) (*JavInfo, error) {
			if code != want.Code {
				t.Fatalf("code = %q, want %q", code, want.Code)
			}
			return want, nil
		}),
	}, nil)
	info, err := client.ResolveJavByCodes(context.Background(), nil, []string{want.Code})
	if err != nil || info != want {
		t.Fatalf("result = %+v, %v; want uncensored metadata", info, err)
	}
}

func TestResolveJavByCodesCancellation(t *testing.T) {
	withTestPriority(t)
	for _, tc := range []struct {
		name   string
		before bool
	}{
		{name: "already cancelled", before: true},
		{name: "cancelled during lookup"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.before {
				cancel()
			}
			var mu sync.Mutex
			calls := 0
			provider := movieLookupFunc(func(got context.Context, _ string) (*JavInfo, error) {
				mu.Lock()
				calls++
				mu.Unlock()
				cancel()
				return nil, got.Err()
			})
			client := NewMetadataClient(map[Provider]any{
				ProviderJavBus: provider,
				ProviderJavDB:  provider,
				ProviderAvmoo:  provider,
				ProviderAvsox:  provider,
			}, nil)
			info, err := client.ResolveJavByCodes(ctx, []string{"IPX-228"}, nil)
			if info != nil || !errors.Is(err, context.Canceled) {
				t.Fatalf("result = %+v, %v; want cancellation", info, err)
			}
			mu.Lock()
			defer mu.Unlock()
			if tc.before && calls != 0 {
				t.Fatalf("lookup calls = %d, want 0", calls)
			}
		})
	}
}

func TestProvidersForCode(t *testing.T) {
	tests := []struct {
		name string
		code string
		want []Provider
	}{
		{
			name: "gana uses javmenu then javbus",
			code: "gana-1234",
			want: []Provider{ProviderJavMenu, ProviderJavBus},
		},
		{
			name: "stars uses javbus then avmoo",
			code: " STARS-001 ",
			want: []Provider{ProviderJavBus, ProviderAvmoo},
		},
		{
			name: "ap uses avmoo",
			code: "ap-001",
			want: []Provider{ProviderAvmoo},
		},
		{
			name: "other uses javbus then javdb",
			code: "IPX-228",
			want: []Provider{ProviderJavBus, ProviderJavDB},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := providersForCode(tt.code)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("providersForCode(%q) = %#v, want %#v", tt.code, got, tt.want)
			}
		})
	}
}
