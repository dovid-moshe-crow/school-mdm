package appmeta

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dwdmsh/school-mdm/internal/store"
	"github.com/dwdmsh/school-mdm/internal/store/memory"
)

func TestFetchAndiTunesUpsert(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"resultCount":1,"results":[{
			"bundleId":"com.example.remote",
			"trackId":123,
			"trackName":"Remote App",
			"artistName":"Example",
			"artworkUrl100":"https://example.com/a.png",
			"trackViewUrl":"https://apps.apple.com/app/id123"
		}]}`))
	}))
	defer srv.Close()

	mem := memory.New()
	cat := &Catalog{Store: mem, Client: srv.Client()}
	list, err := cat.fetchiTunes(context.Background(), srv.URL, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "Remote App" {
		t.Fatalf("%+v", list)
	}
	_ = mem.UpsertAppMeta(context.Background(), list[0])
	got, err := mem.GetAppMeta(context.Background(), "com.example.remote")
	if err != nil {
		t.Fatal(err)
	}
	if got.Artist != "Example" {
		t.Fatalf("%+v", got)
	}
}

func TestLookupBundleUsesCache(t *testing.T) {
	mem := memory.New()
	_ = mem.UpsertAppMeta(context.Background(), store.AppMeta{
		BundleID:    "com.school.cached",
		Name:        "Cached",
		Genre:       "Education",
		Description: "תיאור בעברית",
		UpdatedAt:   time.Now().UTC(),
	})
	cat := &Catalog{Store: mem, Client: http.DefaultClient}
	got, err := cat.LookupBundle(context.Background(), "com.school.cached")
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "cache" || got.Name != "Cached" {
		t.Fatalf("%+v", got)
	}
}

func TestSearchUsesItunesNotStaleDB(t *testing.T) {
	mem := memory.New()
	// Stale/partial DB hit that used to short-circuit Search and hide iTunes results.
	_ = mem.UpsertAppMeta(context.Background(), store.AppMeta{
		BundleID:  "com.stale.only",
		Name:      "YouTube Clone Stale",
		Artist:    "Nobody",
		UpdatedAt: time.Now().UTC(),
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"resultCount":2,"results":[
			{"bundleId":"com.google.ios.youtube","trackId":1,"trackName":"YouTube","artistName":"Google","artworkUrl100":"https://example.com/a.png"},
			{"bundleId":"com.google.ios.youtubemusic","trackId":2,"trackName":"YouTube Music","artistName":"Google","artworkUrl100":"https://example.com/b.png"}
		]}`))
	}))
	defer srv.Close()

	base, _ := url.Parse(srv.URL)
	cat := &Catalog{
		Store: mem,
		Client: &http.Client{
			Timeout: 2 * time.Second,
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				cloned := req.Clone(req.Context())
				cloned.URL.Scheme = base.Scheme
				cloned.URL.Host = base.Host
				return http.DefaultTransport.RoundTrip(cloned)
			}),
		},
	}

	list, err := cat.Search(context.Background(), "youtube", 25)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 2 {
		t.Fatalf("expected live iTunes results, got %#v", list)
	}
	if list[0].Name != "YouTube" {
		t.Fatalf("want YouTube first, got %#v", list)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestItunesURLIncludesCountryAndOmitsInvalidLang(t *testing.T) {
	cat := &Catalog{Country: "il", Lang: "he_il"}
	params := url.Values{}
	params.Set("term", "test")
	raw := cat.itunesURL("search", params)
	if !strings.Contains(raw, "itunes.apple.com/il/search?") {
		t.Fatalf("want country-prefixed path: %s", raw)
	}
	if !strings.Contains(raw, "country=il") {
		t.Fatalf("url missing country: %s", raw)
	}
	if strings.Contains(raw, "lang=") {
		t.Fatalf("invalid lang must be omitted: %s", raw)
	}

	cat.Lang = "en_us"
	raw = cat.itunesURL("search", url.Values{"term": []string{"x"}})
	if !strings.Contains(raw, "lang=en_us") {
		t.Fatalf("en_us should be kept: %s", raw)
	}
}

func TestRankAppsPrefersTitleMatchOverDeveloper(t *testing.T) {
	apps := []store.AppMeta{
		{BundleID: "com.google.googlemobile", Name: "Google", Artist: "Google"},
		{BundleID: "com.google.ios.youtube", Name: "YouTube", Artist: "Google"},
		{BundleID: "com.google.ios.youtubemusic", Name: "YouTube Music", Artist: "Google"},
	}
	ranked := rankApps("youtube", apps, 3)
	if ranked[0].Name != "YouTube" {
		t.Fatalf("want YouTube first, got %#v", ranked)
	}
}
