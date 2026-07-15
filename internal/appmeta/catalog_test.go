package appmeta

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	list, err := cat.fetchiTunes(context.Background(), srv.URL)
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
		BundleID:  "com.school.cached",
		Name:      "Cached",
		UpdatedAt: time.Now().UTC(),
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
