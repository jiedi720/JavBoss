package scrape

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type fakeMovieProvider struct {
	info *JavInfo
	err  error
}

func (f fakeMovieProvider) LookupJavByCode(ctx context.Context, code string) (*JavInfo, error) {
	return f.info, f.err
}

func TestClassifyCode(t *testing.T) {
	cases := []struct{ code, want string }{
		{"FC2-PPV-123456", CodeFC2},
		{"FC2-123456", CodeFC2},
		{"030919-047", CodeUncensored},
		{"123456_001", CodeUncensored},
		{"259LUXU-1905", CodeAmateur},
		{"LUXU-1905", CodeCensored},
		{"IPX-228", CodeCensored},
	}
	for _, tc := range cases {
		if got := ClassifyCode(tc.code); got != tc.want {
			t.Errorf("ClassifyCode(%q) = %q, want %q", tc.code, got, tc.want)
		}
	}
}

func TestNormalizePriorityList(t *testing.T) {
	got := NormalizePriorityList([]string{"javdb", "javdb", "", "bogus", "javbus"})
	want := []string{"javdb", "javbus", "avmoo", "avsox"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizePriorityList() = %v, want %v", got, want)
	}
}

func TestResolvePriorityUsesConfig(t *testing.T) {
	got := ResolvePriority(CodeCensored, "javbus,javdb")
	want := []string{"javbus", "javdb", "avmoo", "avsox"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolvePriority() = %v, want %v", got, want)
	}
}

func TestResolvePriorityFallsBackToDefault(t *testing.T) {
	got := ResolvePriority(CodeCensored, "")
	want := NormalizePriorityList(DefaultPriority[CodeCensored])
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolvePriority() = %v, want %v", got, want)
	}
}

func TestMergeJavInfosByPriority(t *testing.T) {
	sources := []lookupSource{{id: SourceJavBus, provider: ProviderJavBus}, {id: SourceJavDB, provider: ProviderJavDB}}
	bySource := map[string]*JavInfo{
		SourceJavBus: {
			Title:       "bus title",
			Studio:      "bus studio",
			Tags:        []string{"A", "B"},
			CoverURL:    "https://bus/cover.jpg",
			Provider:    ProviderJavBus,
			ReleaseUnix: 100,
		},
		SourceJavDB: {
			Title:       "db title",
			Series:      "db series",
			Tags:        []string{"B", "C"},
			Actors:      []string{"X"},
			CoverURL:    "https://db/cover.jpg",
			Provider:    ProviderJavDB,
			DurationMin: 71,
		},
	}
	got := mergeJavInfos("IPX-228", sources, bySource)
	if got.Title != "bus title" {
		t.Errorf("Title = %q, want bus title", got.Title)
	}
	if got.Studio != "bus studio" || got.Series != "db series" {
		t.Errorf("Studio/Series = %q/%q", got.Studio, got.Series)
	}
	if got.ReleaseUnix != 100 || got.DurationMin != 71 {
		t.Errorf("ReleaseUnix/DurationMin = %d/%d", got.ReleaseUnix, got.DurationMin)
	}
	if !reflect.DeepEqual(got.Tags, []string{"A", "B", "C"}) {
		t.Errorf("Tags = %v", got.Tags)
	}
	if !reflect.DeepEqual(got.Actors, []string{"X"}) {
		t.Errorf("Actors = %v", got.Actors)
	}
	if got.CoverURL != "https://bus/cover.jpg" {
		t.Errorf("CoverURL = %q, want bus cover (JavDB is fallback)", got.CoverURL)
	}
}

func TestAggregateResolveMergesPriority(t *testing.T) {
	client := NewMetadataClient(map[Provider]any{
		ProviderJavBus: fakeMovieProvider{info: &JavInfo{
			Title: "bus title", Tags: []string{"A"}, CoverURL: "bus", Provider: ProviderJavBus,
		}},
		ProviderJavDB: fakeMovieProvider{info: &JavInfo{
			Title: "db title", Actors: []string{"X"}, CoverURL: "db", Provider: ProviderJavDB,
		}},
	}, nil)
	sources := []lookupSource{{id: SourceJavBus, provider: ProviderJavBus}, {id: SourceJavDB, provider: ProviderJavDB}}
	info, err := client.AggregateResolve(context.Background(), "IPX-228", sources)
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "bus title" {
		t.Errorf("Title = %q, want bus title", info.Title)
	}
	if !reflect.DeepEqual(info.Actors, []string{"X"}) {
		t.Errorf("Actors = %v", info.Actors)
	}
	if info.CoverURL != "bus" {
		t.Errorf("CoverURL = %q, want bus (JavDB fallback)", info.CoverURL)
	}
}

func TestAggregateResolveNotFound(t *testing.T) {
	client := NewMetadataClient(map[Provider]any{
		ProviderJavBus: fakeMovieProvider{err: ErrNotFound},
		ProviderJavDB:  fakeMovieProvider{err: ErrNotFound},
	}, nil)
	sources := []lookupSource{{id: SourceJavBus, provider: ProviderJavBus}, {id: SourceJavDB, provider: ProviderJavDB}}
	_, err := client.AggregateResolve(context.Background(), "IPX-228", sources)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestSetPriorityConfig(t *testing.T) {
	SetPriorityConfig(map[string]string{ConfigKeyForCode(CodeCensored): "javdb,javbus"})
	defer SetPriorityConfig(nil)
	if got := priorityConfigFor(CodeCensored); got != "javdb,javbus" {
		t.Fatalf("priorityConfigFor() = %q", got)
	}
	if got := orderedSourcesForCode("LUXU-1905"); len(got) == 0 || got[0].id != SourceJavDB {
		t.Fatalf("orderedSourcesForCode() = %v, want javdb first", got)
	}
}
