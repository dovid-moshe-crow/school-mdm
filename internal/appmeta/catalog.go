package appmeta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/dwdmsh/school-mdm/internal/store"
)

// Catalog searches and caches App Store metadata (DB first, iTunes fallback).
type Catalog struct {
	Store  store.Store
	Client *http.Client
	// Country is the iTunes storefront (default us).
	Country string
}

func (c *Catalog) httpClient() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	// Bypass HTTP(S)_PROXY from the environment — Cursor/dev tooling often sets a
	// local proxy that isn't running, which breaks iTunes Search.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &http.Client{
		Timeout:   8 * time.Second,
		Transport: transport,
	}
}

func (c *Catalog) country() string {
	if c.Country != "" {
		return c.Country
	}
	return "us"
}

// Search returns apps matching query: local cache results merged with live iTunes search.
// Results are upserted into the store when fetched from iTunes.
func (c *Catalog) Search(ctx context.Context, query string, limit int) ([]store.AppMeta, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if limit <= 0 || limit > 25 {
		limit = 12
	}

	byBundle := map[string]store.AppMeta{}

	if local, err := c.Store.SearchAppMeta(ctx, query, limit); err == nil {
		for _, m := range local {
			m.Source = "cache"
			byBundle[m.BundleID] = m
		}
	}

	remote, err := c.searchiTunes(ctx, query, limit)
	if err != nil {
		// Fallback: cache-only if we have anything.
		if len(byBundle) > 0 {
			return rankApps(query, mapValues(byBundle, 0), limit), nil
		}
		return nil, fmt.Errorf("itunes search: %w", err)
	}
	for _, m := range remote {
		m.Source = "itunes"
		_ = c.Store.UpsertAppMeta(ctx, m)
		byBundle[m.BundleID] = m
	}
	return rankApps(query, mapValues(byBundle, 0), limit), nil
}

// LookupBundle loads one app by bundle ID from cache, falling back to iTunes lookup.
func (c *Catalog) LookupBundle(ctx context.Context, bundleID string) (store.AppMeta, error) {
	bundleID = strings.ToLower(strings.TrimSpace(bundleID))
	if bundleID == "" {
		return store.AppMeta{}, fmt.Errorf("bundle_id is required")
	}
	if meta, err := c.Store.GetAppMeta(ctx, bundleID); err == nil {
		meta.Source = "cache"
		return meta, nil
	}

	meta, err := c.lookupiTunes(ctx, bundleID)
	if err != nil {
		return store.AppMeta{}, err
	}
	meta.Source = "itunes"
	_ = c.Store.UpsertAppMeta(ctx, meta)
	return meta, nil
}

type itunesResponse struct {
	ResultCount int `json:"resultCount"`
	Results     []struct {
		BundleID          string `json:"bundleId"`
		TrackID           int64  `json:"trackId"`
		TrackName         string `json:"trackName"`
		TrackCensoredName string `json:"trackCensoredName"`
		ArtistName        string `json:"artistName"`
		SellerName        string `json:"sellerName"`
		ArtworkURL100     string `json:"artworkUrl100"`
		ArtworkURL512     string `json:"artworkUrl512"`
		TrackViewURL      string `json:"trackViewUrl"`
		PrimaryGenreName  string `json:"primaryGenreName"`
		WrapperType       string `json:"wrapperType"`
		Kind              string `json:"kind"`
	} `json:"results"`
}

func (c *Catalog) searchiTunes(ctx context.Context, query string, limit int) ([]store.AppMeta, error) {
	u := fmt.Sprintf(
		"https://itunes.apple.com/search?term=%s&entity=software&country=%s&limit=%d",
		url.QueryEscape(query),
		url.QueryEscape(c.country()),
		limit,
	)
	return c.fetchiTunes(ctx, u)
}

func (c *Catalog) lookupiTunes(ctx context.Context, bundleID string) (store.AppMeta, error) {
	u := fmt.Sprintf(
		"https://itunes.apple.com/lookup?bundleId=%s&country=%s",
		url.QueryEscape(bundleID),
		url.QueryEscape(c.country()),
	)
	list, err := c.fetchiTunes(ctx, u)
	if err != nil {
		return store.AppMeta{}, err
	}
	if len(list) == 0 {
		return store.AppMeta{}, fmt.Errorf("app not found in App Store: %s", bundleID)
	}
	return list[0], nil
}

func (c *Catalog) fetchiTunes(ctx context.Context, rawURL string) ([]store.AppMeta, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "school-mdm/1.0")
	res, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, fmt.Errorf("itunes http %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var parsed itunesResponse
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	out := make([]store.AppMeta, 0, len(parsed.Results))
	now := time.Now().UTC()
	for _, r := range parsed.Results {
		title := strings.TrimSpace(r.TrackName)
		if title == "" {
			title = strings.TrimSpace(r.TrackCensoredName)
		}
		if r.BundleID == "" || title == "" {
			continue
		}
		art := r.ArtworkURL512
		if art == "" {
			art = r.ArtworkURL100
		}
		developer := strings.TrimSpace(r.ArtistName)
		if developer == "" {
			developer = strings.TrimSpace(r.SellerName)
		}
		out = append(out, store.AppMeta{
			BundleID:   strings.ToLower(r.BundleID),
			TrackID:    r.TrackID,
			Name:       title,
			Artist:     developer,
			ArtworkURL: art,
			StoreURL:   r.TrackViewURL,
			UpdatedAt:  now,
		})
	}
	return out, nil
}

func mapValues(m map[string]store.AppMeta, limit int) []store.AppMeta {
	out := make([]store.AppMeta, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

func rankApps(query string, apps []store.AppMeta, limit int) []store.AppMeta {
	q := strings.ToLower(strings.TrimSpace(query))
	sort.SliceStable(apps, func(i, j int) bool {
		return scoreApp(q, apps[i]) > scoreApp(q, apps[j])
	})
	if limit > 0 && len(apps) > limit {
		apps = apps[:limit]
	}
	return apps
}

func scoreApp(q string, app store.AppMeta) int {
	name := strings.ToLower(app.Name)
	artist := strings.ToLower(app.Artist)
	bundle := strings.ToLower(app.BundleID)
	switch {
	case name == q:
		return 100
	case strings.HasPrefix(name, q):
		return 80
	case strings.Contains(name, q):
		return 60
	case strings.Contains(bundle, strings.ReplaceAll(q, " ", "")):
		return 40
	case strings.Contains(artist, q):
		return 10 // developer match is weakest — avoids ranking "Google" app for "youtube"
	default:
		return 0
	}
}
