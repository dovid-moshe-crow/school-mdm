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

func TestSearchReturnsCacheImmediately(t *testing.T) {
	mem := memory.New()
	_ = mem.UpsertAppMeta(context.Background(), store.AppMeta{
		BundleID:  "com.google.ios.youtube",
		Name:      "YouTube",
		Artist:    "Google",
		UpdatedAt: time.Now().UTC(),
	})
	// Slow remote: Search must not wait on it when the DB already has hits.
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		_, _ = w.Write([]byte(`{"resultCount":0,"results":[]}`))
	}))
	defer slow.Close()

	cat := &Catalog{
		Store:  mem,
		Client: slow.Client(),
	}
	// Point Client at slow server; searchiTunes still targets itunes.apple.com.
	// Warm-cache path returns before any remote round-trip.
	start := time.Now()
	list, err := cat.Search(context.Background(), "youtube", 10)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 || list[0].Name != "YouTube" {
		t.Fatalf("%+v", list)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("Search blocked too long with warm cache: %v", elapsed)
	}
}

func TestItunesURLIncludesCountryAndLang(t *testing.T) {
	cat := &Catalog{Country: "il", Lang: "he_il"}
	params := url.Values{}
	params.Set("term", "test")
	raw := cat.itunesURL("search", params)
	if !strings.Contains(raw, "country=il") || !strings.Contains(raw, "lang=he_il") {
		t.Fatalf("url missing storefront params: %s", raw)
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
