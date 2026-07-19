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
	"sync"
	"time"

	"github.com/dwdmsh/school-mdm/internal/store"
)

// Catalog searches and caches App Store metadata (DB first, iTunes fallback).
type Catalog struct {
	Store  store.Store
	Client *http.Client
	// Country is the iTunes storefront (default il — Israel).
	Country string
	// Lang is the iTunes language tag (default he_il).
	Lang string

	mu     sync.Mutex
	recent map[string]recentSearch
}

type recentSearch struct {
	at   time.Time
	apps []store.AppMeta
}

const recentSearchTTL = 90 * time.Second

func (c *Catalog) httpClient() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	// Bypass HTTP(S)_PROXY from the environment — Cursor/dev tooling often sets a
	// local proxy that isn't running, which breaks iTunes Search.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &http.Client{
		Timeout:   5 * time.Second,
		Transport: transport,
	}
}

func (c *Catalog) country() string {
	if c.Country != "" {
		return c.Country
	}
	return "il"
}

func (c *Catalog) lang() string {
	if c.Lang != "" {
		return c.Lang
	}
	return "he_il"
}

func (c *Catalog) getRecent(query string) []store.AppMeta {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.recent == nil {
		return nil
	}
	ent, ok := c.recent[query]
	if !ok || time.Since(ent.at) > recentSearchTTL {
		return nil
	}
	out := make([]store.AppMeta, len(ent.apps))
	copy(out, ent.apps)
	return out
}

func (c *Catalog) putRecent(query string, apps []store.AppMeta) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.recent == nil {
		c.recent = map[string]recentSearch{}
	}
	cp := make([]store.AppMeta, len(apps))
	copy(cp, apps)
	c.recent[query] = recentSearch{at: time.Now(), apps: cp}
}

// Search returns apps matching query. Local DB / memory hits are returned
// immediately; iTunes is refreshed in the background so the UI stays snappy.
func (c *Catalog) Search(ctx context.Context, query string, limit int) ([]store.AppMeta, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if limit <= 0 || limit > 25 {
		limit = 12
	}
	qKey := strings.ToLower(query)

	if cached := c.getRecent(qKey); len(cached) > 0 {
		return rankApps(query, cached, limit), nil
	}

	byBundle := map[string]store.AppMeta{}
	if local, err := c.Store.SearchAppMeta(ctx, query, limit); err == nil {
		for _, m := range local {
			m.Source = "cache"
			byBundle[m.BundleID] = m
		}
	}

	if len(byBundle) > 0 {
		out := rankApps(query, mapValues(byBundle), limit)
		c.putRecent(qKey, out)
		// Refresh from iTunes without blocking the response.
		go c.backgroundSearchUpsert(query, limit)
		return out, nil
	}

	// Cold path: nothing in DB — wait for iTunes.
	apps, err := c.searchiTunes(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("itunes search: %w", err)
	}
	for _, m := range apps {
		m.Source = "itunes"
		byBundle[m.BundleID] = m
	}
	go c.upsertAll(apps)
	out := rankApps(query, mapValues(byBundle), limit)
	c.putRecent(qKey, out)
	return out, nil
}

func (c *Catalog) backgroundSearchUpsert(query string, limit int) {
	bg, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	apps, err := c.searchiTunes(bg, query, limit)
	if err != nil {
		return
	}
	c.upsertAll(apps)
	// Refresh in-memory recent so a soft client re-fetch picks up iTunes results.
	byBundle := map[string]store.AppMeta{}
	for _, m := range apps {
		m.Source = "itunes"
		byBundle[m.BundleID] = m
	}
	if local, err := c.Store.SearchAppMeta(bg, query, limit); err == nil {
		for _, m := range local {
			if _, ok := byBundle[m.BundleID]; !ok {
				m.Source = "cache"
				byBundle[m.BundleID] = m
			}
		}
	}
	c.putRecent(strings.ToLower(query), rankApps(query, mapValues(byBundle), limit))
}

func (c *Catalog) upsertAll(apps []store.AppMeta) {
	bg, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	for _, m := range apps {
		m.Source = "itunes"
		_ = c.Store.UpsertAppMeta(bg, m)
	}
}

// LookupBundle loads one app by bundle ID from cache, falling back to iTunes lookup.
func (c *Catalog) LookupBundle(ctx context.Context, bundleID string) (store.AppMeta, error) {
	return c.LookupBundleOpt(ctx, bundleID, false)
}

// LookupBundleOpt loads metadata; when refresh is true always hits iTunes Lookup
// so the client gets full description / ratings / screenshots (Hebrew storefront).
func (c *Catalog) LookupBundleOpt(ctx context.Context, bundleID string, refresh bool) (store.AppMeta, error) {
	bundleID = strings.ToLower(strings.TrimSpace(bundleID))
	if bundleID == "" {
		return store.AppMeta{}, fmt.Errorf("bundle_id is required")
	}
	if !refresh {
		if meta, err := c.Store.GetAppMeta(ctx, bundleID); err == nil {
			// Prefer cache when we already have rich details.
			if meta.Description != "" || meta.Genre != "" {
				meta.Source = "cache"
				return meta, nil
			}
		}
	}

	meta, err := c.lookupiTunes(ctx, bundleID)
	if err != nil {
		// Fall back to whatever we have cached.
		if cached, cerr := c.Store.GetAppMeta(ctx, bundleID); cerr == nil {
			cached.Source = "cache"
			return cached, nil
		}
		return store.AppMeta{}, err
	}
	meta.Source = "itunes"
	_ = c.Store.UpsertAppMeta(ctx, meta)
	return meta, nil
}

type itunesResponse struct {
	ResultCount int            `json:"resultCount"`
	Results     []itunesResult `json:"results"`
}

type itunesResult struct {
	BundleID              string   `json:"bundleId"`
	TrackID               int64    `json:"trackId"`
	TrackName             string   `json:"trackName"`
	TrackCensoredName     string   `json:"trackCensoredName"`
	ArtistName            string   `json:"artistName"`
	SellerName            string   `json:"sellerName"`
	ArtworkURL100         string   `json:"artworkUrl100"`
	ArtworkURL512         string   `json:"artworkUrl512"`
	TrackViewURL          string   `json:"trackViewUrl"`
	PrimaryGenreName      string   `json:"primaryGenreName"`
	Description           string   `json:"description"`
	Version               string   `json:"version"`
	AverageUserRating     float64  `json:"averageUserRating"`
	UserRatingCount       int      `json:"userRatingCount"`
	ContentAdvisoryRating string   `json:"contentAdvisoryRating"`
	ReleaseDate           string   `json:"releaseDate"`
	FormattedPrice        string   `json:"formattedPrice"`
	FileSizeBytes         string   `json:"fileSizeBytes"`
	ScreenshotURLs        []string `json:"screenshotUrls"`
	IpadScreenshotURLs    []string `json:"ipadScreenshotUrls"`
	WrapperType           string   `json:"wrapperType"`
	Kind                  string   `json:"kind"`
}

func (c *Catalog) itunesURL(path string, params url.Values) string {
	params.Set("country", c.country())
	params.Set("lang", c.lang())
	return "https://itunes.apple.com/" + path + "?" + params.Encode()
}

func (c *Catalog) searchiTunes(ctx context.Context, query string, limit int) ([]store.AppMeta, error) {
	params := url.Values{}
	params.Set("term", query)
	params.Set("entity", "software")
	params.Set("limit", fmt.Sprintf("%d", limit))
	return c.fetchiTunes(ctx, c.itunesURL("search", params), false)
}

func (c *Catalog) lookupiTunes(ctx context.Context, bundleID string) (store.AppMeta, error) {
	params := url.Values{}
	params.Set("bundleId", bundleID)
	list, err := c.fetchiTunes(ctx, c.itunesURL("lookup", params), true)
	if err != nil {
		return store.AppMeta{}, err
	}
	if len(list) == 0 {
		return store.AppMeta{}, fmt.Errorf("app not found in App Store: %s", bundleID)
	}
	return list[0], nil
}

func (c *Catalog) fetchiTunes(ctx context.Context, rawURL string, full bool) ([]store.AppMeta, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "school-mdm/1.0")
	req.Header.Set("Accept-Language", "he-IL,he;q=0.9")
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
		meta, ok := mapItunes(r, now, full)
		if !ok {
			continue
		}
		out = append(out, meta)
	}
	return out, nil
}

func mapItunes(r itunesResult, now time.Time, full bool) (store.AppMeta, bool) {
	title := strings.TrimSpace(r.TrackName)
	if title == "" {
		title = strings.TrimSpace(r.TrackCensoredName)
	}
	if r.BundleID == "" || title == "" {
		return store.AppMeta{}, false
	}
	art := r.ArtworkURL512
	if art == "" {
		art = r.ArtworkURL100
	}
	developer := strings.TrimSpace(r.ArtistName)
	if developer == "" {
		developer = strings.TrimSpace(r.SellerName)
	}
	meta := store.AppMeta{
		BundleID:   strings.ToLower(r.BundleID),
		TrackID:    r.TrackID,
		Name:       title,
		Artist:     developer,
		ArtworkURL: art,
		StoreURL:   r.TrackViewURL,
		UpdatedAt:  now,
		Genre:      strings.TrimSpace(r.PrimaryGenreName),
		SellerName: strings.TrimSpace(r.SellerName),
	}
	if full {
		meta.Description = strings.TrimSpace(r.Description)
		meta.Version = strings.TrimSpace(r.Version)
		meta.AverageRating = r.AverageUserRating
		meta.RatingCount = r.UserRatingCount
		meta.ContentRating = strings.TrimSpace(r.ContentAdvisoryRating)
		meta.ReleaseDate = strings.TrimSpace(r.ReleaseDate)
		meta.FormattedPrice = strings.TrimSpace(r.FormattedPrice)
		if n, err := parseInt64(r.FileSizeBytes); err == nil {
			meta.FileSizeBytes = n
		}
		shots := append([]string{}, r.ScreenshotURLs...)
		if len(shots) == 0 {
			shots = append(shots, r.IpadScreenshotURLs...)
		}
		if len(shots) > 6 {
			shots = shots[:6]
		}
		meta.Screenshots = shots
	}
	return meta, true
}

func parseInt64(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	var n int64
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("bad")
		}
		n = n*10 + int64(ch-'0')
	}
	return n, nil
}

func mapValues(m map[string]store.AppMeta) []store.AppMeta {
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
