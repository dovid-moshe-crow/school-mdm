package appmeta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/store"
)

// Catalog searches App Store metadata via iTunes and caches lookups in the DB.
type Catalog struct {
	Store  store.Store
	Client *http.Client
	Log    *slog.Logger
	// Country is the iTunes storefront (default il — Israel).
	Country string
	// Lang is optional iTunes language (only en_us / ja_jp are accepted by Apple).
	// Hebrew descriptions come from Country=il, not lang.
	Lang string
}

func (c *Catalog) log() *slog.Logger {
	if c.Log != nil {
		return c.Log
	}
	return slog.Default()
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
		Timeout:   4 * time.Second,
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
	// Empty: omit lang. Apple only accepts en_us / ja_jp; Hebrew comes from country=il.
	return ""
}

func effectiveItunesLang(lang string) string {
	switch lang {
	case "en_us", "ja_jp":
		return lang
	default:
		return ""
	}
}

// Search returns live App Store matches for query (iTunes is the source of truth).
// Results are upserted into the DB in the background for later lookups — the
// search response itself is never served from a sticky in-memory/DB-only cache,
// which previously hid iTunes hits and returned incomplete lists.
func (c *Catalog) Search(ctx context.Context, query string, limit int) ([]store.AppMeta, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if limit <= 0 || limit > 25 {
		limit = 25
	}

	start := time.Now()
	c.log().Info("app search start",
		"query", query,
		"limit", limit,
		"country", c.country(),
		"lang", effectiveItunesLang(c.lang()),
	)

	apps, err := c.searchiTunes(ctx, query, limit)
	itunesMS := time.Since(start).Milliseconds()
	if err != nil {
		c.log().Warn("app search itunes failed",
			"query", query,
			"err", err,
			"itunes_ms", itunesMS,
		)
		// Soft fallback: if iTunes is down, show whatever we already know locally.
		local := SearchKnown(query)
		if cached, lerr := c.Store.SearchAppMeta(ctx, query, limit); lerr == nil {
			local = append(local, cached...)
		}
		if len(local) > 0 {
			for i := range local {
				if local[i].Source == "" {
					local[i].Source = "cache"
				}
			}
			out := rankApps(query, local, limit)
			c.log().Info("app search fallback to local cache",
				"query", query,
				"results", len(out),
				"total_ms", time.Since(start).Milliseconds(),
				"names", appNames(out, 8),
			)
			return out, nil
		}
		return nil, fmt.Errorf("itunes search: %w", err)
	}
	apps = append(SearchKnown(query), apps...)
	for i := range apps {
		if apps[i].Source == "" {
			apps[i].Source = "itunes"
		}
	}
	out := rankApps(query, apps, limit)
	c.log().Info("app search ok",
		"query", query,
		"itunes_raw", len(apps),
		"results", len(out),
		"itunes_ms", itunesMS,
		"total_ms", time.Since(start).Milliseconds(),
		"names", appNames(out, 8),
	)
	go c.upsertAll(apps)
	return out, nil
}

func appNames(apps []store.AppMeta, max int) []string {
	n := len(apps)
	if n > max {
		n = max
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, apps[i].Name)
	}
	return out
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
	return c.lookupBundleOpt(ctx, bundleID, refresh)
}

func (c *Catalog) lookupBundleOpt(ctx context.Context, bundleID string, refresh bool) (store.AppMeta, error) {
	bundleID = strings.TrimSpace(bundleID)
	if bundleID == "" {
		return store.AppMeta{}, fmt.Errorf("bundle_id is required")
	}
	if !refresh {
		if meta, err := c.Store.GetAppMeta(ctx, bundleID); err == nil {
			// Prefer cache when we already have rich details.
			if meta.Description != "" || meta.Genre != "" {
				meta.Source = "cache"
				return overlayKnown(meta), nil
			}
		}
		if known, ok := Known(bundleID); ok {
			return known, nil
		}
	}

	meta, err := c.lookupiTunes(ctx, bundleID)
	if err != nil {
		if cached, cerr := c.Store.GetAppMeta(ctx, bundleID); cerr == nil {
			cached.Source = "cache"
			return overlayKnown(cached), nil
		}
		if known, ok := Known(bundleID); ok {
			return known, nil
		}
		return store.AppMeta{}, err
	}
	meta.Source = "itunes"
	_ = c.Store.UpsertAppMeta(ctx, meta)
	return overlayKnown(meta), nil
}

func overlayKnown(meta store.AppMeta) store.AppMeta {
	known, ok := Known(meta.BundleID)
	if !ok {
		return meta
	}
	if strings.TrimSpace(meta.Name) == "" {
		meta.Name = known.Name
	}
	return meta
}

// Resolve returns the best display metadata without failing: cache, well-known
// system apps, then optional iTunes. Name is never empty (falls back to the bundle id).
func (c *Catalog) Resolve(ctx context.Context, bundleID string, remote bool) store.AppMeta {
	bundleID = strings.TrimSpace(bundleID)
	if bundleID == "" {
		return store.AppMeta{}
	}
	if c != nil && c.Store != nil {
		if meta, err := c.Store.GetAppMeta(ctx, bundleID); err == nil {
			meta = overlayKnown(meta)
			if strings.TrimSpace(meta.Name) != "" {
				if !remote || meta.ArtworkURL != "" || meta.Source == "local" {
					if meta.Source == "" {
						meta.Source = "cache"
					}
					return meta
				}
			}
		}
	}
	if known, ok := Known(bundleID); ok && !remote {
		return known
	}
	if remote && c != nil {
		if meta, err := c.LookupBundle(ctx, bundleID); err == nil {
			return overlayKnown(meta)
		}
	}
	if known, ok := Known(bundleID); ok {
		return known
	}
	return store.AppMeta{BundleID: bundleID, Name: bundleID, Source: "unknown"}
}

// LookupMany resolves many bundle IDs. When remote is true, App Store misses are
// fetched (capped) so admin lists can show icons and titles.
func (c *Catalog) LookupMany(ctx context.Context, ids []string, remote bool) []store.AppMeta {
	seen := map[string]int{}
	slots := make([]store.AppMeta, 0, len(ids))
	var missing []string
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		key := policy.AppKey(id)
		if _, ok := seen[key]; ok {
			continue
		}
		m := c.Resolve(ctx, id, false)
		seen[key] = len(slots)
		slots = append(slots, m)
		if remote && m.Source != "local" && (m.Source == "unknown" || m.ArtworkURL == "") {
			missing = append(missing, m.BundleID)
		}
	}
	if remote && len(missing) > 0 && c != nil {
		if len(missing) > 25 {
			missing = missing[:25]
		}
		ch := make(chan store.AppMeta, len(missing))
		sem := make(chan struct{}, 6)
		var wg sync.WaitGroup
		for _, id := range missing {
			wg.Add(1)
			go func(bundleID string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				ch <- c.Resolve(ctx, bundleID, true)
			}(id)
		}
		go func() {
			wg.Wait()
			close(ch)
		}()
		for meta := range ch {
			if meta.BundleID == "" {
				continue
			}
			if i, ok := seen[policy.AppKey(meta.BundleID)]; ok {
				slots[i] = meta
			}
		}
	}
	return slots
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
	cc := c.country()
	params.Set("country", cc)
	// Apple only accepts en_us / ja_jp for lang; he_il etc. return HTTP 400.
	switch lang := c.lang(); lang {
	case "en_us", "ja_jp":
		params.Set("lang", lang)
	}
	// Country-prefixed path returns storefront-localized metadata (Hebrew for il).
	return fmt.Sprintf("https://itunes.apple.com/%s/%s?%s", url.PathEscape(cc), path, params.Encode())
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
		BundleID:   strings.TrimSpace(r.BundleID), // keep Apple casing (MDM allow-list is case-sensitive)
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

func rankApps(query string, apps []store.AppMeta, limit int) []store.AppMeta {
	q := strings.ToLower(strings.TrimSpace(query))
	sort.SliceStable(apps, func(i, j int) bool {
		return scoreApp(q, apps[i]) > scoreApp(q, apps[j])
	})
	seen := map[string]struct{}{}
	out := make([]store.AppMeta, 0, len(apps))
	for _, app := range apps {
		key := policy.AppKey(app.BundleID)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, app)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
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
		return 10
	default:
		return 0
	}
}
