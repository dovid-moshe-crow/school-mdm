package httpapi

import (
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/dwdmsh/school-mdm/internal/approvals"
	"github.com/dwdmsh/school-mdm/internal/config"
	"github.com/dwdmsh/school-mdm/internal/mdm"
	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/store"
)

// API serves product HTTP endpoints.
type API struct {
	Cfg      config.Config
	Service  *approvals.Service
	Store    store.Store
	Stub     *mdm.StubEnqueuer
	Log      *slog.Logger
}

// Mount registers routes on mux.
func (a *API) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", a.handleHealthz)
	mux.HandleFunc("GET /api/allowlist", a.handleAllowlist)
	mux.HandleFunc("POST /api/requests", a.handleCreateRequest)
	mux.HandleFunc("GET /api/requests", a.requireAdmin(a.handleListRequests))
	mux.HandleFunc("POST /api/requests/{id}/approve", a.requireAdmin(a.handleApprove))
	mux.HandleFunc("POST /api/requests/{id}/deny", a.requireAdmin(a.handleDeny))
	mux.HandleFunc("GET /api/stub-commands", a.requireAdmin(a.handleStubCommands))
	mux.HandleFunc("GET /", a.handlePortal)
	mux.HandleFunc("GET /admin", a.handleAdminPage)
}

func (a *API) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if err := a.Store.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"store": a.Store.Kind(),
		"time":  time.Now().UTC().Format(time.RFC3339),
	})
}

func (a *API) handleAllowlist(w http.ResponseWriter, r *http.Request) {
	enrollment := r.URL.Query().Get("enrollment_id")
	apps, urls, err := a.Service.EffectiveAllowlist(r.Context(), enrollment)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"apps": apps, "urls": urls})
}

type createRequestBody struct {
	Kind         string `json:"kind"`
	Value        string `json:"value"`
	EnrollmentID string `json:"enrollment_id"`
	Reason       string `json:"reason"`
}

func (a *API) handleCreateRequest(w http.ResponseWriter, r *http.Request) {
	var body createRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	req, err := a.Service.CreateRequest(r.Context(), approvals.CreateRequestInput{
		Kind:         policy.Kind(body.Kind),
		Value:        body.Value,
		EnrollmentID: body.EnrollmentID,
		Reason:       body.Reason,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, req)
}

func (a *API) handleListRequests(w http.ResponseWriter, r *http.Request) {
	var statusFilter *store.RequestStatus
	if s := r.URL.Query().Get("status"); s != "" {
		st := store.RequestStatus(s)
		statusFilter = &st
	}
	list, err := a.Store.ListRequests(r.Context(), statusFilter)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

type decideBody struct {
	Duration string `json:"duration"`
}

func (a *API) handleApprove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body decideBody
	_ = json.NewDecoder(r.Body).Decode(&body)
	req, err := a.Service.Decide(r.Context(), approvals.DecideInput{
		RequestID: id,
		Approve:   true,
		Duration:  body.Duration,
	})
	if err != nil {
		writeDecideErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, req)
}

func (a *API) handleDeny(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	req, err := a.Service.Decide(r.Context(), approvals.DecideInput{
		RequestID: id,
		Approve:   false,
	})
	if err != nil {
		writeDecideErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, req)
}

func (a *API) handleStubCommands(w http.ResponseWriter, r *http.Request) {
	if a.Stub == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	cmds := a.Stub.Snapshot()
	type view struct {
		EnrollmentID string    `json:"enrollment_id"`
		At           time.Time `json:"at"`
		ProfileBytes int       `json:"profile_bytes"`
		ProfilePreview string  `json:"profile_preview"`
	}
	out := make([]view, 0, len(cmds))
	for _, c := range cmds {
		preview := string(c.Profile)
		if len(preview) > 400 {
			preview = preview[:400] + "…"
		}
		out = append(out, view{
			EnrollmentID:   c.EnrollmentID,
			At:             c.At,
			ProfileBytes:   len(c.Profile),
			ProfilePreview: preview,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		token = strings.TrimSpace(token)
		if token == "" {
			token = r.URL.Query().Get("token")
		}
		if !a.Cfg.ValidAdminToken(token) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (a *API) handlePortal(w http.ResponseWriter, r *http.Request) {
	urlPrefill := r.URL.Query().Get("url")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = portalTmpl.Execute(w, map[string]string{"URL": urlPrefill})
}

func (a *API) handleAdminPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	token := r.URL.Query().Get("token")
	if token == "" && len(a.Cfg.AdminTokens) > 0 {
		token = a.Cfg.AdminTokens[0]
	}
	_ = adminTmpl.Execute(w, map[string]string{"Token": token})
}

func writeDecideErr(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
}

func writeErr(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

var portalTmpl = template.Must(template.New("portal").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Request Access</title>
  <style>
    body{font-family:ui-sans-serif,system-ui,sans-serif;max-width:32rem;margin:2rem auto;padding:0 1rem;line-height:1.4}
    label{display:block;margin:.75rem 0 .25rem;font-weight:600}
    input,select,textarea,button{width:100%;padding:.6rem;font:inherit;box-sizing:border-box}
    button{margin-top:1rem;background:#111;color:#fff;border:0;cursor:pointer}
    #msg{margin-top:1rem}
  </style>
</head>
<body>
  <h1>Request Access</h1>
  <p>Ask to open an app or website. An admin can approve with one tap.</p>
  <form id="f">
    <label for="kind">Type</label>
    <select id="kind" name="kind">
      <option value="url">Website</option>
      <option value="app">App</option>
    </select>
    <label for="value">App bundle ID or URL</label>
    <input id="value" name="value" required value="{{.URL}}"/>
    <label for="enrollment_id">Device ID (optional)</label>
    <input id="enrollment_id" name="enrollment_id" placeholder="enrollment id"/>
    <label for="reason">Reason</label>
    <textarea id="reason" name="reason" rows="3"></textarea>
    <button type="submit">Submit request</button>
  </form>
  <div id="msg"></div>
  <script>
    document.getElementById('f').addEventListener('submit', async (e) => {
      e.preventDefault();
      const body = {
        kind: document.getElementById('kind').value,
        value: document.getElementById('value').value,
        enrollment_id: document.getElementById('enrollment_id').value,
        reason: document.getElementById('reason').value,
      };
      const res = await fetch('/api/requests', {method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify(body)});
      const data = await res.json();
      document.getElementById('msg').textContent = res.ok
        ? ('Submitted. Status: ' + data.status + ' (id ' + data.id + ')')
        : (data.error || 'failed');
    });
  </script>
</body>
</html>`))

var adminTmpl = template.Must(template.New("admin").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Admin — Access Requests</title>
  <style>
    body{font-family:ui-sans-serif,system-ui,sans-serif;max-width:48rem;margin:2rem auto;padding:0 1rem}
    .card{border:1px solid #ddd;border-radius:8px;padding:1rem;margin:.75rem 0}
    button{margin-right:.5rem;padding:.45rem .8rem;font:inherit;cursor:pointer}
    .approve{background:#0a7;color:#fff;border:0}
    .deny{background:#b33;color:#fff;border:0}
    code{font-size:.85em}
  </style>
</head>
<body>
  <h1>Access requests</h1>
  <p>Token used: <code id="token">{{.Token}}</code></p>
  <div id="list">Loading…</div>
  <script>
    const token = document.getElementById('token').textContent.trim();
    async function load() {
      const res = await fetch('/api/requests?status=pending', {headers:{Authorization:'Bearer '+token}});
      const data = await res.json();
      const el = document.getElementById('list');
      if (!res.ok) { el.textContent = data.error || 'failed'; return; }
      if (!data || data.length === 0) { el.textContent = 'No pending requests.'; return; }
      el.innerHTML = data.map(r =>
        '<div class="card" data-id="'+r.id+'">' +
          '<div><strong>'+r.kind+'</strong>: '+r.value+'</div>' +
          '<div>Device: '+(r.enrollment_id || '—')+' · '+(r.reason || '')+'</div>' +
          '<div>' +
            '<button class="approve" onclick="decide(\''+r.id+'\', true)">Approve 1h</button>' +
            '<button class="approve" onclick="decide(\''+r.id+'\', true, \'permanent\')">Approve permanent</button>' +
            '<button class="deny" onclick="decide(\''+r.id+'\', false)">Deny</button>' +
          '</div>' +
        '</div>').join('');
    }
    async function decide(id, approve, duration='1h') {
      const path = approve ? 'approve' : 'deny';
      const res = await fetch('/api/requests/'+id+'/'+path, {
        method:'POST',
        headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},
        body: JSON.stringify({duration})
      });
      const data = await res.json();
      if (!res.ok) alert(data.error || 'failed');
      load();
    }
    load();
  </script>
</body>
</html>`))
