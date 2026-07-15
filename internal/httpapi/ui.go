package httpapi

const sharedCSS = `
:root{
  --ink:#101820;
  --muted:#5a6670;
  --paper:#eef2f4;
  --card:#ffffff;
  --line:#cfd8de;
  --accent:#0b6e4f;
  --danger:#a33b3b;
  --shadow:0 16px 36px rgba(16,24,32,.08);
}
*{box-sizing:border-box}
body{
  margin:0;
  font-family:"Sora",ui-sans-serif,system-ui,sans-serif;
  color:var(--ink);
  background:linear-gradient(180deg, #dfecef 0%, var(--paper) 42%, #e8edf0 100%);
  min-height:100vh;
}
.shell{max-width:40rem;margin:0 auto;padding:2.5rem 1.25rem 4rem}
.brand{font-weight:700;font-size:clamp(1.75rem,4vw,2.25rem);letter-spacing:-.03em;margin:0 0 .35rem}
.lede{color:var(--muted);margin:0 0 1.5rem;line-height:1.5}
.panel{background:var(--card);border:1px solid var(--line);border-radius:16px;padding:1.25rem;box-shadow:var(--shadow)}
label{display:block;margin:.9rem 0 .35rem;font-weight:600;font-size:.92rem}
input,select,textarea,button{
  width:100%;padding:.75rem .85rem;font:inherit;border-radius:12px;border:1px solid var(--line);background:#fff;
}
textarea{resize:vertical;min-height:5rem}
button{margin-top:1.1rem;background:var(--ink);color:#fff;border:0;cursor:pointer;font-weight:600}
button:disabled{opacity:.55;cursor:not-allowed}
button.secondary{background:transparent;color:var(--ink);border:1px solid var(--line);margin-top:0;width:auto}
button.approve{background:var(--accent);color:#fff;border:0;width:auto;margin:.35rem .4rem .05rem 0}
button.deny{background:var(--danger);color:#fff;border:0;width:auto;margin:.35rem .4rem .05rem 0}
button.tiny{margin-top:.5rem;padding:.45rem .7rem;font-size:.85rem;width:auto}
.device-chip{
  display:inline-flex;align-items:center;gap:.4rem;background:#d9efe6;color:var(--accent);
  border-radius:999px;padding:.35rem .75rem;font-size:.85rem;font-weight:600;margin-bottom:1rem;
}
.search-row{display:grid;grid-template-columns:1fr auto;gap:.5rem}
.results{margin-top:.75rem;display:grid;gap:.55rem;max-height:22rem;overflow:auto;padding-right:.15rem}
.app-row{
  display:grid;grid-template-columns:52px 1fr auto;gap:.75rem;align-items:center;
  border:1px solid var(--line);border-radius:14px;padding:.65rem;background:#fff;
  cursor:pointer;text-align:left;margin:0;width:100%;
  color:var(--ink);
  overflow:visible;
}
.app-row img{width:52px;height:52px;border-radius:12px;object-fit:cover;background:#eee}
.app-row strong{display:block;color:var(--ink);font-weight:700;line-height:1.25}
.app-row span{display:block;color:var(--muted);font-size:.85rem;line-height:1.3}
.app-row .meta{color:var(--muted)}
.picked{
  display:flex;gap:.85rem;align-items:center;margin-top:.75rem;padding:.85rem;
  border-radius:14px;background:#e7f5ef;border:1px solid #bfe3d4;
}
.picked img{width:52px;height:52px;border-radius:12px;object-fit:cover}
.picked .actions{margin-left:auto}
.msg{margin-top:1rem;font-weight:600}
.msg.ok{color:var(--accent)}
.msg.err{color:var(--danger)}
.card{border:1px solid var(--line);border-radius:16px;padding:1rem;margin:.85rem 0;background:#fff}
.badge{display:inline-block;padding:.2rem .55rem;border-radius:999px;background:#e8eef2;font-size:.75rem;margin-right:.3rem;font-weight:600}
.meta{color:var(--muted);font-size:.9rem;margin:.35rem 0}
code{font-family:"IBM Plex Mono",ui-monospace,monospace;font-size:.85em}
.filters{display:grid;gap:.65rem;margin:1rem 0 1.25rem}
.filters-row{display:grid;grid-template-columns:repeat(auto-fit,minmax(9rem,1fr));gap:.55rem}
.filters input,.filters select{margin:0}
.tabs{display:flex;gap:.4rem;flex-wrap:wrap;margin:1rem 0}
.tab{
  width:auto;margin:0;padding:.55rem .9rem;border-radius:999px;border:1px solid var(--line);
  background:#fff;color:var(--ink);font-weight:600;cursor:pointer;
}
.tab.active{background:var(--ink);color:#fff;border-color:var(--ink)}
.count{color:var(--muted);font-size:.9rem;margin:.25rem 0 1rem}
.actions-row{display:flex;flex-wrap:wrap;gap:.25rem;margin-top:.45rem}
`

var homeTmpl = mustTemplate("home", `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>School MDM</title>
  <link href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono&family=Sora:wght@400;600;700&display=swap" rel="stylesheet"/>
  <style>`+sharedCSS+`</style>
</head>
<body>
  <main class="shell">
    <h1 class="brand">School MDM</h1>
    <p class="lede">Open this portal from your device link so your requests are tied to the right iPad.</p>
    <div class="panel">
      <p>Use a URL like <code>/d/&lt;device-id&gt;</code> (the Web Clip on managed devices does this for you).</p>
      <p class="meta">Blocked websites can deep-link with <code>?url=https://example.com</code>.</p>
      <p><a href="/admin">Admin console</a></p>
    </div>
  </main>
</body>
</html>`)

var portalTmpl = mustTemplate("portal", `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Submit a request</title>
  <link href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono&family=Sora:wght@400;600;700&display=swap" rel="stylesheet"/>
  <script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.14.8/dist/cdn.min.js"></script>
  <style>`+sharedCSS+`</style>
</head>
<body>
  <main class="shell" x-data="portalApp({{json .}})" x-cloak>
    <h1 class="brand">Ask for help</h1>
    <p class="lede">Request an app or website, ask a general question, or report a bug.</p>
    <div class="device-chip">Device <code x-text="deviceID"></code></div>

    <form class="panel" @submit.prevent="submit">
      <label for="category">What is this?</label>
      <select id="category" x-model="category" @change="onCategoryChange">
        <option value="access-url">Website access</option>
        <option value="access-app">App access</option>
        <option value="general">General request</option>
        <option value="bug">Bug report</option>
      </select>

      <div x-show="category === 'access-url'">
        <label for="url">Website URL</label>
        <input id="url" x-model="url" :required="category === 'access-url'" placeholder="https://"/>
      </div>

      <div x-show="category === 'access-app'">
        <div x-show="!selected">
          <label for="app-q">Search the App Store</label>
          <div class="search-row">
            <input id="app-q" x-model="query" @input.debounce.350ms="search" placeholder="e.g. YouTube" autocomplete="off"/>
            <button type="button" class="secondary" @click="search" :disabled="searching">Search</button>
          </div>
          <p class="meta" x-show="searching">Searching…</p>
          <p class="meta" x-show="!searching && searched && results.length === 0">No apps found.</p>
          <div class="results" x-show="results.length > 0">
            <template x-for="item in results" :key="item.bundle_id">
              <button type="button" class="app-row" @click="pick(item)">
                <img :src="item.artwork_url || ''" alt=""/>
                <div>
                  <strong x-text="item.app_name"></strong>
                  <span x-text="item.developer ? ('by ' + item.developer) : ''"></span>
                </div>
                <div class="meta">Pick</div>
              </button>
            </template>
          </div>
        </div>

        <div x-show="selected">
          <label>Selected app</label>
          <div class="picked">
            <img :src="selected && selected.artwork_url ? selected.artwork_url : ''" alt=""/>
            <div>
              <strong x-text="selected && selected.app_name"></strong>
              <div class="meta" x-text="selected && selected.developer ? ('by ' + selected.developer) : ''"></div>
              <code x-text="selected && selected.bundle_id"></code>
            </div>
            <div class="actions">
              <button type="button" class="secondary tiny" @click="clearApp">Change</button>
            </div>
          </div>
        </div>
      </div>

      <div x-show="category === 'general' || category === 'bug'">
        <label for="subject" x-text="category === 'bug' ? 'Bug title' : 'Subject'"></label>
        <input id="subject" x-model="subject" :required="category === 'general' || category === 'bug'"/>
      </div>

      <label for="reason" x-text="reasonLabel"></label>
      <textarea id="reason" x-model="reason" rows="3"></textarea>

      <button type="submit" :disabled="submitting" x-text="submitting ? 'Sending…' : 'Submit'"></button>
      <div class="msg" :class="msgOk ? 'ok' : 'err'" x-text="msg" x-show="msg"></div>
    </form>
  </main>

  <script>
    function portalApp(boot) {
      return {
        deviceID: boot.DeviceID || '',
        category: (boot.URL ? 'access-url' : 'access-app'),
        url: boot.URL || '',
        subject: '',
        reason: '',
        query: '',
        results: [],
        selected: null,
        searching: false,
        searched: false,
        submitting: false,
        msg: '',
        msgOk: true,
        get reasonLabel() {
          if (this.category === 'access-url') return 'Why do you need it?';
          if (this.category === 'access-app') return 'Why do you need this app?';
          if (this.category === 'bug') return 'What happened? (steps to reproduce)';
          return 'Details';
        },
        onCategoryChange() {
          this.msg = '';
          this.clearApp();
          this.results = [];
          this.searched = false;
          this.query = '';
        },
        clearApp() {
          this.selected = null;
        },
        async search() {
          const q = this.query.trim();
          this.results = [];
          this.searched = false;
          if (!q) return;
          this.searching = true;
          try {
            const res = await fetch('/api/apps/search?q=' + encodeURIComponent(q));
            const data = await res.json();
            if (!res.ok) throw new Error(data.error || 'search failed');
            this.results = Array.isArray(data) ? data : [];
            this.searched = true;
          } catch (e) {
            this.msg = e.message || 'search failed';
            this.msgOk = false;
          } finally {
            this.searching = false;
          }
        },
        pick(item) {
          this.selected = item;
          this.results = [];
          this.query = '';
          this.searched = false;
          this.msg = '';
        },
        async submit() {
          this.msg = '';
          let type = 'general', kind = '', value = '';
          if (this.category === 'access-url') {
            type = 'access'; kind = 'url'; value = this.url.trim();
          } else if (this.category === 'access-app') {
            type = 'access'; kind = 'app';
            if (!this.selected) {
              this.msg = 'Pick an app from search results.';
              this.msgOk = false;
              return;
            }
            value = this.selected.bundle_id;
          } else if (this.category === 'bug') {
            type = 'bug'; value = this.subject.trim();
          } else {
            type = 'general'; value = this.subject.trim();
          }
          this.submitting = true;
          try {
            const res = await fetch('/api/requests', {
              method: 'POST',
              headers: {'Content-Type': 'application/json'},
              body: JSON.stringify({
                type, kind, value,
                enrollment_id: this.deviceID,
                reason: this.reason,
              }),
            });
            const data = await res.json();
            if (!res.ok) throw new Error(data.error || 'failed');
            this.msg = 'Submitted · ' + data.type + ' · ' + data.status;
            this.msgOk = true;
            this.reason = '';
            if (this.category === 'access-app') this.clearApp();
            if (this.category === 'access-url') this.url = '';
            if (this.category === 'general' || this.category === 'bug') this.subject = '';
          } catch (e) {
            this.msg = e.message || 'failed';
            this.msgOk = false;
          } finally {
            this.submitting = false;
          }
        },
      };
    }
  </script>
</body>
</html>`)

var adminTmpl = mustTemplate("admin", `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Admin — School MDM</title>
  <link href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono&family=Sora:wght@400;600;700&display=swap" rel="stylesheet"/>
  <script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.14.8/dist/cdn.min.js"></script>
  <style>`+sharedCSS+`</style>
</head>
<body>
  <main class="shell" style="max-width:56rem" x-data="adminApp({{json .}})" x-init="boot()" x-cloak>
    <h1 class="brand">Admin</h1>
    <p class="lede">Requests, device groups, and scoped allowlists (global ∪ groups ∪ device).</p>

    <div class="tabs">
      <button type="button" class="tab" :class="tab === 'requests' && 'active'" @click="tab = 'requests'; loadRequests()">Requests</button>
      <button type="button" class="tab" :class="tab === 'groups' && 'active'" @click="tab = 'groups'; loadGroups()">Groups</button>
      <button type="button" class="tab" :class="tab === 'allowances' && 'active'" @click="tab = 'allowances'; loadAllowances()">Allowlists</button>
    </div>

    <div class="panel filters" x-show="tab === 'requests'">
      <div class="filters-row">
        <div>
          <label>Status</label>
          <select x-model="reqStatus" @change="loadRequests()">
            <option value="all">All</option>
            <option value="pending">Pending</option>
            <option value="open">Open (pending)</option>
            <option value="closed">Closed</option>
            <option value="approved">Approved</option>
            <option value="denied">Denied</option>
            <option value="resolved">Resolved</option>
          </select>
        </div>
        <div>
          <label>Type</label>
          <select x-model="reqType" @change="loadRequests()">
            <option value="all">All</option>
            <option value="access">Access</option>
            <option value="general">General</option>
            <option value="bug">Bug</option>
          </select>
        </div>
        <div>
          <label>Device</label>
          <select x-model="reqDevice" @change="loadRequests()">
            <option value="">All devices</option>
            <template x-for="d in devices" :key="d">
              <option :value="d" x-text="d"></option>
            </template>
          </select>
        </div>
        <div>
          <label>Sort</label>
          <select x-model="reqSort" @change="loadRequests()">
            <option value="created_desc">Newest first</option>
            <option value="created_asc">Oldest first</option>
            <option value="status">Status</option>
            <option value="type">Type</option>
            <option value="device">Device</option>
          </select>
        </div>
      </div>
      <div>
        <label>Search</label>
        <input x-model="reqQ" @input.debounce.300ms="loadRequests()" placeholder="Search value, reason, device, id…"/>
      </div>
    </div>

    <div class="panel filters" x-show="tab === 'allowances'">
      <div class="filters-row">
        <div>
          <label>Scope</label>
          <select x-model="allowScope" @change="onAllowScopeChange()">
            <option value="global">Everyone (global)</option>
            <option value="group">Group</option>
            <option value="device">Device (effective)</option>
            <option value="all">All sources</option>
          </select>
        </div>
        <div>
          <label>Kind</label>
          <select x-model="allowKind" @change="loadAllowances()">
            <option value="all">Apps + URLs</option>
            <option value="app">Apps only</option>
            <option value="url">URLs only</option>
          </select>
        </div>
        <div x-show="allowScope === 'group'">
          <label>Group</label>
          <select x-model="allowGroup" @change="loadAllowances()">
            <option value="">Select group…</option>
            <template x-for="g in groups" :key="'af-'+g.id">
              <option :value="g.id" x-text="g.name"></option>
            </template>
          </select>
        </div>
        <div x-show="allowScope === 'device'">
          <label>Device</label>
          <select x-model="allowDevice" @change="loadAllowances()">
            <option value="">Select device…</option>
            <template x-for="d in devices" :key="'a-'+d">
              <option :value="d" x-text="d"></option>
            </template>
          </select>
        </div>
      </div>
      <div>
        <label>Search</label>
        <input x-model="allowQ" @input.debounce.300ms="loadAllowances()" placeholder="Search bundle ID, URL, app name…"/>
      </div>
    </div>

    <p class="count" x-show="tab === 'requests'" x-text="loadingReq ? 'Loading…' : (requests.length + ' request(s)')"></p>
    <p class="count" x-show="tab === 'allowances'" x-text="loadingAllow ? 'Loading…' : (allowances.length + ' allowlist item(s)')"></p>
    <p class="count" x-show="tab === 'groups'" x-text="loadingGroups ? 'Loading…' : (groups.length + ' group(s)')"></p>
    <div class="msg err" x-show="error" x-text="error"></div>
    <div class="msg ok" x-show="okMsg" x-text="okMsg"></div>

    <template x-if="tab === 'requests'">
      <div>
        <template x-for="r in requests" :key="r.id">
          <div class="card">
            <div>
              <span class="badge" x-text="r.status"></span>
              <span class="badge" x-text="r.type"></span>
              <span class="badge" x-show="r.kind" x-text="r.kind"></span>
            </div>
            <div style="margin-top:.55rem" x-show="r.app">
              <div style="display:flex;gap:.75rem;align-items:center">
                <img x-show="r.app && r.app.artwork_url" :src="r.app && r.app.artwork_url" alt="" style="width:44px;height:44px;border-radius:10px"/>
                <div>
                  <strong x-text="r.app && (r.app.app_name || r.value)"></strong>
                  <div class="meta">
                    <span x-text="r.app && r.app.developer ? ('by ' + r.app.developer + ' · ') : ''"></span>
                    <code x-text="r.value"></code>
                  </div>
                </div>
              </div>
            </div>
            <div style="margin-top:.55rem" x-show="!r.app">
              <strong x-text="r.value"></strong>
            </div>
            <div class="meta">Device <code x-text="r.enrollment_id || '—'"></code></div>
            <div class="meta" x-text="r.reason || ''"></div>
            <div class="meta" x-text="'Created ' + fmtTime(r.created_at)"></div>
            <div class="actions-row" x-show="r.status === 'pending'">
              <template x-if="r.type === 'access'">
                <div style="width:100%">
                  <div class="filters-row" style="margin-top:.5rem">
                    <div>
                      <label>Grant scope</label>
                      <select x-model="approveScope[r.id]">
                        <option value="device">This device</option>
                        <option value="group">A group</option>
                        <option value="global">Everyone</option>
                      </select>
                    </div>
                    <div x-show="(approveScope[r.id] || 'device') === 'group'">
                      <label>Group</label>
                      <select x-model="approveGroup[r.id]">
                        <option value="">Select…</option>
                        <template x-for="g in groups" :key="'ap-'+r.id+'-'+g.id">
                          <option :value="g.id" x-text="g.name"></option>
                        </template>
                      </select>
                    </div>
                  </div>
                  <div class="actions-row">
                    <button class="approve" @click="decide(r.id, true, '1h')">Approve 1h</button>
                    <button class="approve" @click="decide(r.id, true, 'permanent')">Permanent</button>
                    <button class="deny" @click="decide(r.id, false)">Deny</button>
                  </div>
                </div>
              </template>
              <template x-if="r.type === 'bug'">
                <div class="actions-row">
                  <button class="approve" @click="decide(r.id, true)">Resolve</button>
                  <button class="deny" @click="decide(r.id, false)">Deny</button>
                </div>
              </template>
              <template x-if="r.type === 'general'">
                <div class="actions-row">
                  <button class="approve" @click="decide(r.id, true)">Approve</button>
                  <button class="deny" @click="decide(r.id, false)">Deny</button>
                </div>
              </template>
            </div>
          </div>
        </template>
        <div class="panel" x-show="!loadingReq && !requests.length">No requests match these filters.</div>
      </div>
    </template>

    <div x-show="tab === 'groups'">
      <div class="panel" style="margin-bottom:1rem">
        <strong>Create a group</strong>
        <p class="meta" style="margin:.35rem 0 0">Name a set of devices, then attach allowlists to the group.</p>
        <label>Name</label>
        <input x-model="newGroupName" placeholder="e.g. Grade 7 Math" @keydown.enter.prevent="createGroup()"/>
        <label>Description (optional)</label>
        <input x-model="newGroupDesc" placeholder="Short note" @keydown.enter.prevent="createGroup()"/>
        <button class="approve" type="button" style="width:100%;margin-top:1rem" @click="createGroup()" :disabled="!(newGroupName || '').trim()">
          Create group
        </button>
      </div>
      <template x-for="g in groups" :key="g.id">
        <div class="card">
          <div style="display:flex;justify-content:space-between;gap:.75rem;align-items:flex-start;flex-wrap:wrap">
            <div>
              <strong x-text="g.name"></strong>
              <div class="meta" x-text="g.description || ''"></div>
              <div class="meta"><code x-text="g.id"></code></div>
            </div>
            <div class="actions-row">
              <button class="tiny secondary" type="button" @click="openGroup(g)">Members</button>
              <button class="tiny secondary" type="button" @click="viewGroupAllowlist(g)">View allowlist</button>
              <button class="tiny deny" type="button" @click="deleteGroup(g)">Delete</button>
            </div>
          </div>
          <div x-show="selectedGroup && selectedGroup.id === g.id" style="margin-top:1rem;border-top:1px solid var(--line);padding-top:.85rem">
            <label>Members</label>
            <div style="display:flex;flex-wrap:wrap;gap:.45rem;margin-top:.5rem">
              <template x-for="d in devices" :key="'m-'+g.id+'-'+d">
                <label style="display:flex;align-items:center;gap:.35rem;font-weight:500;margin:0;font-size:.9rem">
                  <input type="checkbox" style="width:auto;margin:0" :value="d" :checked="groupMembers.includes(d)" @change="toggleMember(d, $event.target.checked)"/>
                  <span x-text="d"></span>
                </label>
              </template>
            </div>
            <p class="meta" x-show="!devices.length">No known devices yet — they appear after portal requests.</p>
            <div class="actions-row">
              <button class="tiny approve" type="button" @click="saveMembers()">Save members</button>
              <button class="tiny secondary" type="button" @click="selectedGroup = null">Close</button>
            </div>
          </div>
        </div>
      </template>
      <div class="panel" x-show="!loadingGroups && !groups.length">No groups yet — use Create group above.</div>
    </div>

    <template x-if="tab === 'allowances'">
      <div>
        <div class="panel" style="margin-bottom:1rem">
          <strong>Add allow</strong>
          <div class="filters-row" style="margin-top:.65rem">
            <div>
              <label>Kind</label>
              <select x-model="addKind" @change="addApp = null; addResults = []; addValue = ''">
                <option value="url">URL</option>
                <option value="app">App</option>
              </select>
            </div>
            <div>
              <label>Scope</label>
              <select x-model="addScope">
                <option value="global">Everyone</option>
                <option value="group">Group</option>
                <option value="device">Device</option>
              </select>
            </div>
            <div>
              <label>Duration</label>
              <select x-model="addDuration">
                <option value="permanent">Permanent</option>
                <option value="1h">1 hour</option>
                <option value="24h">24 hours</option>
                <option value="15m">15 minutes</option>
                <option value="today">Today</option>
              </select>
            </div>
          </div>
          <div class="filters-row" x-show="addScope === 'group'">
            <div>
              <label>Group</label>
              <select x-model="addGroup">
                <option value="">Select…</option>
                <template x-for="g in groups" :key="'addg-'+g.id">
                  <option :value="g.id" x-text="g.name"></option>
                </template>
              </select>
            </div>
          </div>
          <div class="filters-row" x-show="addScope === 'device'">
            <div>
              <label>Device</label>
              <select x-model="addDevice">
                <option value="">Select…</option>
                <template x-for="d in devices" :key="'addd-'+d">
                  <option :value="d" x-text="d"></option>
                </template>
              </select>
            </div>
          </div>
          <div x-show="addKind === 'url'">
            <label>URL or host</label>
            <input x-model="addValue" placeholder="khanacademy.org"/>
          </div>
          <div x-show="addKind === 'app'">
            <label>Search App Store</label>
            <div class="search-row">
              <input x-model="addAppQ" @keydown.enter.prevent="searchAddApp()" placeholder="App name…"/>
              <button class="tiny" type="button" @click="searchAddApp()">Search</button>
            </div>
            <div class="results" x-show="addResults.length">
              <template x-for="app in addResults" :key="app.bundle_id">
                <button type="button" class="app-row" @click="pickAddApp(app)">
                  <img :src="app.artwork_url" alt=""/>
                  <div>
                    <strong x-text="app.app_name"></strong>
                    <span x-text="'by ' + (app.developer || '—')"></span>
                    <span class="meta" x-text="app.bundle_id"></span>
                  </div>
                </button>
              </template>
            </div>
            <div class="picked" x-show="addApp">
              <img x-show="addApp && addApp.artwork_url" :src="addApp && addApp.artwork_url" alt=""/>
              <div>
                <strong x-text="addApp && addApp.app_name"></strong>
                <div class="meta" x-text="addApp && addApp.bundle_id"></div>
              </div>
              <div class="actions"><button class="tiny secondary" type="button" @click="addApp=null; addValue=''">Clear</button></div>
            </div>
          </div>
          <button class="tiny approve" type="button" @click="createAllowance()">Add to allowlist</button>
        </div>
        <template x-for="(row, idx) in allowances" :key="row.kind + row.value + row.source + (row.target_id||'') + idx">
          <div class="card">
            <div>
              <span class="badge" x-text="row.kind"></span>
              <span class="badge" x-text="row.source"></span>
              <span class="badge" x-show="row.target_type" x-text="row.target_type"></span>
            </div>
            <div style="margin-top:.55rem;display:flex;gap:.75rem;align-items:center" x-show="row.app">
              <img x-show="row.app && row.app.artwork_url" :src="row.app && row.app.artwork_url" alt="" style="width:44px;height:44px;border-radius:10px"/>
              <div>
                <strong x-text="row.app && (row.app.app_name || row.value)"></strong>
                <div class="meta">
                  <span x-text="row.app && row.app.developer ? ('by ' + row.app.developer + ' · ') : ''"></span>
                  <code x-text="row.value"></code>
                </div>
              </div>
            </div>
            <div style="margin-top:.55rem" x-show="!row.app">
              <strong x-text="row.value"></strong>
            </div>
            <div class="meta" x-show="row.enrollment_id">Device <code x-text="row.enrollment_id"></code></div>
            <div class="meta" x-show="row.group_id">Group <code x-text="groupName(row.group_id)"></code></div>
            <div class="meta" x-show="row.expires_at" x-text="'Expires ' + fmtTime(row.expires_at)"></div>
          </div>
        </template>
        <div class="panel" x-show="!loadingAllow && !allowances.length">No allowlist items match these filters.</div>
      </div>
    </template>
  </main>
  <script>
    function adminApp(boot) {
      return {
        token: boot.Token || '',
        tab: 'requests',
        devices: [],
        groups: [],
        requests: [],
        allowances: [],
        loadingReq: false,
        loadingAllow: false,
        loadingGroups: false,
        error: '',
        okMsg: '',
        reqStatus: 'pending',
        reqType: 'all',
        reqDevice: '',
        reqSort: 'created_desc',
        reqQ: '',
        allowScope: 'global',
        allowKind: 'all',
        allowDevice: '',
        allowGroup: '',
        allowQ: '',
        approveScope: {},
        approveGroup: {},
        newGroupName: '',
        newGroupDesc: '',
        selectedGroup: null,
        groupMembers: [],
        addKind: 'url',
        addScope: 'global',
        addGroup: '',
        addDevice: '',
        addDuration: 'permanent',
        addValue: '',
        addAppQ: '',
        addResults: [],
        addApp: null,
        async boot() {
          await Promise.all([this.loadDevices(), this.loadGroups()]);
          await this.loadRequests();
        },
        authHeaders() {
          return {Authorization: 'Bearer ' + this.token};
        },
        fmtTime(v) {
          if (!v) return '';
          try { return new Date(v).toLocaleString(); } catch (_) { return v; }
        },
        groupName(id) {
          const g = this.groups.find(x => x.id === id);
          return g ? g.name : id;
        },
        async loadDevices() {
          try {
            const res = await fetch('/api/devices', {headers: this.authHeaders()});
            const data = await res.json();
            if (res.ok) this.devices = Array.isArray(data) ? data : [];
          } catch (_) {}
        },
        async loadGroups() {
          this.loadingGroups = true;
          try {
            const res = await fetch('/api/groups', {headers: this.authHeaders()});
            const data = await res.json();
            if (!res.ok) throw new Error(data.error || 'failed');
            this.groups = Array.isArray(data) ? data : [];
          } catch (e) {
            this.error = e.message || 'failed';
          } finally {
            this.loadingGroups = false;
          }
        },
        async loadRequests() {
          this.loadingReq = true;
          this.error = '';
          try {
            const params = new URLSearchParams({
              status: this.reqStatus,
              type: this.reqType,
              sort: this.reqSort,
            });
            if (this.reqDevice) params.set('enrollment_id', this.reqDevice);
            if (this.reqQ.trim()) params.set('q', this.reqQ.trim());
            const res = await fetch('/api/requests?' + params.toString(), {headers: this.authHeaders()});
            const data = await res.json();
            if (!res.ok) throw new Error(data.error || 'failed');
            this.requests = Array.isArray(data) ? data : [];
            for (const r of this.requests) {
              if (this.approveScope[r.id] === undefined) this.approveScope[r.id] = 'device';
            }
          } catch (e) {
            this.error = e.message || 'failed';
          } finally {
            this.loadingReq = false;
          }
        },
        onAllowScopeChange() {
          if (this.allowScope === 'device' && !this.allowDevice && this.devices.length) {
            this.allowDevice = this.devices[0];
          }
          if (this.allowScope === 'group' && !this.allowGroup && this.groups.length) {
            this.allowGroup = this.groups[0].id;
          }
          this.loadAllowances();
        },
        async loadAllowances() {
          this.loadingAllow = true;
          this.error = '';
          try {
            if (this.allowScope === 'device' && !this.allowDevice) {
              this.allowances = [];
              return;
            }
            if (this.allowScope === 'group' && !this.allowGroup) {
              this.allowances = [];
              return;
            }
            const params = new URLSearchParams({
              scope: this.allowScope,
              kind: this.allowKind,
            });
            if (this.allowScope === 'device') params.set('enrollment_id', this.allowDevice);
            if (this.allowScope === 'group') params.set('group_id', this.allowGroup);
            if (this.allowQ.trim()) params.set('q', this.allowQ.trim());
            const res = await fetch('/api/allowances?' + params.toString(), {headers: this.authHeaders()});
            const data = await res.json();
            if (!res.ok) throw new Error(data.error || 'failed');
            this.allowances = Array.isArray(data) ? data : [];
          } catch (e) {
            this.error = e.message || 'failed';
          } finally {
            this.loadingAllow = false;
          }
        },
        async decide(id, approve, duration) {
          this.error = '';
          this.okMsg = '';
          try {
            const body = {duration: duration || ''};
            if (approve) {
              body.scope = this.approveScope[id] || 'device';
              if (body.scope === 'group') body.group_id = this.approveGroup[id] || '';
            }
            const res = await fetch('/api/requests/' + id + '/' + (approve ? 'approve' : 'deny'), {
              method: 'POST',
              headers: Object.assign({'Content-Type': 'application/json'}, this.authHeaders()),
              body: JSON.stringify(body),
            });
            const data = await res.json();
            if (!res.ok) throw new Error(data.error || 'failed');
            await Promise.all([this.loadRequests(), this.loadDevices(), this.loadAllowances()]);
          } catch (e) {
            this.error = e.message || 'failed';
          }
        },
        async createGroup() {
          this.error = '';
          this.okMsg = '';
          const name = (this.newGroupName || '').trim();
          if (!name) {
            this.error = 'Enter a group name';
            return;
          }
          try {
            const res = await fetch('/api/groups', {
              method: 'POST',
              headers: Object.assign({'Content-Type': 'application/json'}, this.authHeaders()),
              body: JSON.stringify({name: name, description: (this.newGroupDesc || '').trim()}),
            });
            const data = await res.json();
            if (!res.ok) throw new Error(data.error || 'failed');
            this.newGroupName = '';
            this.newGroupDesc = '';
            this.okMsg = 'Group created';
            await this.loadGroups();
          } catch (e) {
            this.error = e.message || 'failed';
          }
        },
        async openGroup(g) {
          this.selectedGroup = g;
          this.error = '';
          try {
            const res = await fetch('/api/groups/' + g.id + '/members', {headers: this.authHeaders()});
            const data = await res.json();
            if (!res.ok) throw new Error(data.error || 'failed');
            this.groupMembers = Array.isArray(data) ? data.slice() : [];
          } catch (e) {
            this.error = e.message || 'failed';
          }
        },
        toggleMember(id, on) {
          if (on) {
            if (!this.groupMembers.includes(id)) this.groupMembers.push(id);
          } else {
            this.groupMembers = this.groupMembers.filter(x => x !== id);
          }
        },
        async saveMembers() {
          if (!this.selectedGroup) return;
          this.error = '';
          this.okMsg = '';
          try {
            const res = await fetch('/api/groups/' + this.selectedGroup.id + '/members', {
              method: 'PUT',
              headers: Object.assign({'Content-Type': 'application/json'}, this.authHeaders()),
              body: JSON.stringify({enrollment_ids: this.groupMembers}),
            });
            const data = await res.json();
            if (!res.ok) throw new Error(data.error || 'failed');
            this.groupMembers = Array.isArray(data) ? data : [];
            this.okMsg = 'Members saved';
          } catch (e) {
            this.error = e.message || 'failed';
          }
        },
        async deleteGroup(g) {
          if (!confirm('Delete group "' + g.name + '"?')) return;
          this.error = '';
          try {
            const res = await fetch('/api/groups/' + g.id, {method: 'DELETE', headers: this.authHeaders()});
            const data = await res.json();
            if (!res.ok) throw new Error(data.error || 'failed');
            if (this.selectedGroup && this.selectedGroup.id === g.id) this.selectedGroup = null;
            await this.loadGroups();
          } catch (e) {
            this.error = e.message || 'failed';
          }
        },
        viewGroupAllowlist(g) {
          this.tab = 'allowances';
          this.allowScope = 'group';
          this.allowGroup = g.id;
          this.loadAllowances();
        },
        async searchAddApp() {
          this.error = '';
          this.addResults = [];
          if (!this.addAppQ.trim()) return;
          try {
            const res = await fetch('/api/apps/search?q=' + encodeURIComponent(this.addAppQ.trim()));
            const data = await res.json();
            if (!res.ok) throw new Error(data.error || 'search failed');
            this.addResults = Array.isArray(data) ? data : [];
          } catch (e) {
            this.error = e.message || 'failed';
          }
        },
        pickAddApp(app) {
          this.addApp = app;
          this.addValue = app.bundle_id;
          this.addResults = [];
        },
        async createAllowance() {
          this.error = '';
          this.okMsg = '';
          try {
            const value = this.addKind === 'app' ? (this.addApp && this.addApp.bundle_id) || this.addValue : this.addValue;
            const body = {
              kind: this.addKind,
              value: value,
              scope: this.addScope,
              duration: this.addDuration,
              group_id: this.addGroup,
              enrollment_id: this.addDevice,
            };
            const res = await fetch('/api/allowances', {
              method: 'POST',
              headers: Object.assign({'Content-Type': 'application/json'}, this.authHeaders()),
              body: JSON.stringify(body),
            });
            const data = await res.json();
            if (!res.ok) throw new Error(data.error || 'failed');
            this.okMsg = 'Allowlist updated';
            this.addValue = '';
            this.addApp = null;
            await this.loadAllowances();
          } catch (e) {
            this.error = e.message || 'failed';
          }
        },
      };
    }
  </script>
</body>
</html>`)
