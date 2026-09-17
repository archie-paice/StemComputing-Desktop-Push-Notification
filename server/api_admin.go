package main

// api_admin.go - the JSON API behind the web console (and for scripts using
// an API token). Full reference: docs/API.md

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type apiError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, apiError{Error: code, Message: msg})
}

func readJSON(w http.ResponseWriter, r *http.Request, v any, limit int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		writeErr(w, http.StatusBadRequest, "bad_json", "The request body is not valid JSON.")
		return false
	}
	return true
}

func (a *App) save() {
	if err := a.store.flush(); err != nil {
		log.Printf("ERROR saving data: %v", err)
	}
}

// ---------- sign in / out / own account ----------

type userView struct {
	ID              string    `json:"id"`
	Username        string    `json:"username"`
	DisplayName     string    `json:"display_name"`
	Role            string    `json:"role"`
	MustChange      bool      `json:"must_change"`
	Disabled        bool      `json:"disabled"`
	AllowedGroupIDs []string  `json:"allowed_group_ids"`
	LastLogin       time.Time `json:"last_login"`
}

func viewUser(u *User) userView {
	g := u.AllowedGroupIDs
	if g == nil {
		g = []string{}
	}
	return userView{u.ID, u.Username, u.DisplayName, u.Role, u.MustChange, u.Disabled, g, u.LastLogin}
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct{ Username, Password string }
	if !readJSON(w, r, &req, 8<<10) {
		return
	}
	if r.Header.Get("X-Foghorn") == "" {
		writeErr(w, http.StatusForbidden, "csrf", "Missing X-Foghorn header.")
		return
	}
	ip := a.remoteIP(r)
	keys := []string{"ip:" + ip, "user:" + strings.ToLower(req.Username)}
	for _, k := range keys {
		if d := a.throttle.blocked(k); d > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(d.Seconds())+1))
			writeErr(w, http.StatusTooManyRequests, "locked",
				"Too many failed sign-ins. Try again in "+strconv.Itoa(int(d.Minutes())+1)+" minutes.")
			return
		}
	}

	a.store.mu.Lock()
	u := a.store.userByName(strings.TrimSpace(req.Username))
	hash := dummyHash
	if u != nil {
		hash = u.PassHash
	}
	a.store.mu.Unlock()

	ok := verifyPassword(hash, req.Password) && u != nil
	if ok {
		a.store.mu.Lock()
		ok = !u.Disabled
		if ok {
			u.LastLogin = time.Now()
			a.store.dirtyState = true
		}
		a.store.mu.Unlock()
	}
	if !ok {
		for _, k := range keys {
			a.throttle.fail(k)
		}
		log.Printf("sign-in FAILED for %q from %s", req.Username, ip)
		writeErr(w, http.StatusUnauthorized, "bad_login", "That username and password do not match.")
		return
	}
	for _, k := range keys {
		a.throttle.reset(k)
	}
	sid := a.sessions.create(u.ID)
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: sid, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteStrictMode, Secure: a.isHTTPS(r),
		MaxAge: int(sessionLifetime.Seconds()),
	})
	log.Printf("sign-in ok: %s from %s", u.Username, ip)
	a.store.mu.Lock()
	v := viewUser(u)
	a.store.mu.Unlock()
	writeJSON(w, http.StatusOK, v)
}

func (a *App) isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return a.cfg.TrustProxyHeaders && strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		a.sessions.drop(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request, id *identity) {
	a.store.mu.Lock()
	org := a.store.State.Settings.OrgName
	a.store.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"user": viewUser(&id.User), "org_name": org, "version": version,
	})
}

func (a *App) handleChangeOwnPassword(w http.ResponseWriter, r *http.Request, id *identity) {
	if id.ViaToken {
		writeErr(w, http.StatusBadRequest, "bad_request", "API tokens do not have passwords.")
		return
	}
	var req struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if !readJSON(w, r, &req, 8<<10) {
		return
	}
	if len(req.New) < minPasswordLen {
		writeErr(w, http.StatusBadRequest, "weak_password", "Use at least "+strconv.Itoa(minPasswordLen)+" characters.")
		return
	}
	if !verifyPassword(id.User.PassHash, req.Current) {
		writeErr(w, http.StatusForbidden, "bad_password", "Your current password is not right.")
		return
	}
	newHash := hashPassword(req.New)
	a.store.mu.Lock()
	if u := a.store.userByID(id.User.ID); u != nil {
		u.PassHash = newHash
		u.MustChange = false
		a.store.dirtyState = true
	}
	a.store.mu.Unlock()
	a.save()
	// The first-run file holds the temporary password; it has done its job.
	_ = os.Remove(filepath.Join(a.store.dir, firstRunFile))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---------- alerts ----------

type alertView struct {
	ID            string      `json:"id"`
	Spec          AlertSpec   `json:"spec"`
	TargetSummary string      `json:"target_summary"`
	Sender        string      `json:"sender"`
	SenderName    string      `json:"sender_name"`
	CreatedAt     time.Time   `json:"created_at"`
	ExpiresAt     time.Time   `json:"expires_at"`
	RecalledAt    *time.Time  `json:"recalled_at,omitempty"`
	RecalledBy    string      `json:"recalled_by,omitempty"`
	Status        string      `json:"status"`
	Totals        Totals      `json:"totals"`
	Archived      bool        `json:"archived"`
	Deliveries    []*Delivery `json:"deliveries,omitempty"`
}

func viewAlert(a *Alert, now time.Time, withDeliveries bool) alertView {
	v := alertView{
		ID: a.ID, Spec: a.Spec, TargetSummary: a.TargetSummary, Sender: a.Sender,
		SenderName: a.SenderName, CreatedAt: a.CreatedAt, ExpiresAt: a.ExpiresAt,
		RecalledAt: a.RecalledAt, RecalledBy: a.RecalledBy, Status: a.status(now),
		Totals: a.totals(), Archived: a.Archived != nil,
	}
	if withDeliveries {
		v.Deliveries = make([]*Delivery, 0, len(a.Deliveries))
		for _, d := range a.Deliveries {
			cp := *d
			v.Deliveries = append(v.Deliveries, &cp)
		}
		sort.Slice(v.Deliveries, func(i, j int) bool {
			return strings.ToLower(v.Deliveries[i].Hostname) < strings.ToLower(v.Deliveries[j].Hostname)
		})
	}
	return v
}

func (a *App) handleListAlerts(w http.ResponseWriter, r *http.Request, id *identity) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > maxAlertsKept {
		limit = 100
	}
	now := time.Now()
	a.store.mu.Lock()
	out := make([]alertView, 0, limit)
	for i := len(a.store.Alerts) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, viewAlert(a.store.Alerts[i], now, false))
	}
	a.store.mu.Unlock()
	writeJSON(w, http.StatusOK, out)
}

func (a *App) handleGetAlert(w http.ResponseWriter, r *http.Request, id *identity) {
	a.store.mu.Lock()
	al := a.store.alertByID(r.PathValue("id"))
	var v alertView
	if al != nil {
		v = viewAlert(al, time.Now(), true)
	}
	a.store.mu.Unlock()
	if al == nil {
		writeErr(w, http.StatusNotFound, "not_found", "That alert is no longer in the history.")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// validateSpec tidies and checks an alert. It returns a message for the person
// if something needs fixing.
func validateSpec(s *AlertSpec) string {
	s.Title = cleanText(s.Title, 120)
	s.Message = cleanTextOpt(s.Message, 2000, true)
	s.Link = strings.TrimSpace(s.Link)
	if s.Title == "" {
		return "Give the alert a title."
	}
	switch s.Level {
	case "info", "warning", "critical":
	case "":
		s.Level = "info"
	default:
		return "Level must be info, warning or critical."
	}
	switch s.Display {
	case "corner", "center", "fullscreen":
	case "":
		s.Display = "corner"
	default:
		return "Display must be corner, center or fullscreen."
	}
	if s.DisplaySeconds < 0 || s.DisplaySeconds > 86400 {
		return "Auto-close must be between 0 and 86400 seconds."
	}
	if s.RequireAck {
		s.DisplaySeconds = 0 // an alert that needs acknowledging never closes itself
	}
	if s.ExpiresMinutes == 0 {
		s.ExpiresMinutes = 10
	}
	if s.ExpiresMinutes < 1 || s.ExpiresMinutes > 7*24*60 {
		return "Keep-delivering time must be between 1 minute and 7 days."
	}
	if s.Link != "" {
		u, err := url.Parse(s.Link)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || len(s.Link) > 500 {
			return "The link must be a full http:// or https:// address."
		}
	}
	s.Target.Rules = cleanRules(s.Target.Rules)
	if s.Target.GroupIDs == nil {
		s.Target.GroupIDs = []string{}
	}
	if !s.Target.All && len(s.Target.GroupIDs) == 0 && len(s.Target.Rules) == 0 {
		return "Choose who should receive the alert."
	}
	if s.Target.All {
		s.Target.GroupIDs, s.Target.Rules = []string{}, []Rule{}
	}
	return ""
}

// checkAllowed enforces per-sender restrictions. Caller holds the lock.
func (s *Store) checkAllowed(u *User, t Target) string {
	if u.Role == "admin" || len(u.AllowedGroupIDs) == 0 {
		return ""
	}
	if t.All || len(t.Rules) > 0 {
		return "Your account can only send to the groups an administrator has chosen for you."
	}
	for _, gid := range t.GroupIDs {
		ok := false
		for _, allowed := range u.AllowedGroupIDs {
			if gid == allowed {
				ok = true
			}
		}
		if !ok {
			return "Your account is not allowed to send to one of those groups."
		}
	}
	return ""
}

func (a *App) handleSendAlert(w http.ResponseWriter, r *http.Request, id *identity) {
	var spec AlertSpec
	if !readJSON(w, r, &spec, 64<<10) {
		return
	}
	if msg := validateSpec(&spec); msg != "" {
		writeErr(w, http.StatusBadRequest, "invalid", msg)
		return
	}
	now := time.Now()
	st := a.store
	st.mu.Lock()
	if msg := st.checkAllowed(&id.User, spec.Target); msg != "" {
		st.mu.Unlock()
		writeErr(w, http.StatusForbidden, "forbidden", msg)
		return
	}
	for _, gid := range spec.Target.GroupIDs {
		if st.groupByID(gid) == nil {
			st.mu.Unlock()
			writeErr(w, http.StatusBadRequest, "invalid", "One of the chosen groups no longer exists. Reload the page.")
			return
		}
	}
	rules, summary := st.flattenTarget(spec.Target)
	name := id.User.DisplayName
	if name == "" {
		name = id.User.Username
	}
	al := &Alert{
		ID: newID(), Spec: spec, Rules: rules, TargetSummary: summary,
		Sender: id.User.Username, SenderName: name, CreatedAt: now,
		ExpiresAt:  now.Add(time.Duration(spec.ExpiresMinutes) * time.Minute),
		Deliveries: map[string]*Delivery{},
	}
	st.Alerts = append(st.Alerts, al)
	st.dirtyAlerts = true
	reach := 0
	for _, c := range st.Clients {
		if c.online(now) && alertMatches(al, c) {
			reach++
		}
	}
	v := viewAlert(al, now, false)
	st.mu.Unlock()

	a.hub.wake()
	a.save()
	log.Printf("ALERT %s sent by %s to [%s] level=%s display=%s title=%q (%d online now)",
		al.ID, id.User.Username, summary, spec.Level, spec.Display, spec.Title, reach)
	writeJSON(w, http.StatusCreated, map[string]any{"alert": v, "online_reach": reach})
}

func (a *App) handleRecallAlert(w http.ResponseWriter, r *http.Request, id *identity) {
	now := time.Now()
	a.store.mu.Lock()
	al := a.store.alertByID(r.PathValue("id"))
	if al == nil {
		a.store.mu.Unlock()
		writeErr(w, http.StatusNotFound, "not_found", "That alert is no longer in the history.")
		return
	}
	if id.User.Role != "admin" && al.Sender != id.User.Username {
		a.store.mu.Unlock()
		writeErr(w, http.StatusForbidden, "forbidden", "You can only recall alerts you sent.")
		return
	}
	if al.RecalledAt == nil {
		al.RecalledAt = &now
		al.RecalledBy = id.User.Username
		a.store.dirtyAlerts = true
	}
	v := viewAlert(al, now, false)
	a.store.mu.Unlock()
	a.hub.wake()
	a.save()
	log.Printf("ALERT %s recalled by %s", al.ID, id.User.Username)
	writeJSON(w, http.StatusOK, v)
}

// handleReach answers "how many computers would this reach right now?"
func (a *App) handleReach(w http.ResponseWriter, r *http.Request, id *identity) {
	var t Target
	if !readJSON(w, r, &t, 64<<10) {
		return
	}
	t.Rules = cleanRules(t.Rules)
	now := time.Now()
	a.store.mu.Lock()
	rules, _ := a.store.flattenTarget(t)
	probe := &Alert{Spec: AlertSpec{Target: t}, Rules: rules}
	online, known := 0, 0
	sample := []string{}
	for _, c := range a.store.Clients {
		if !alertMatches(probe, c) {
			continue
		}
		known++
		if c.online(now) {
			online++
			if len(sample) < 8 {
				sample = append(sample, c.Hostname)
			}
		}
	}
	a.store.mu.Unlock()
	sort.Strings(sample)
	writeJSON(w, http.StatusOK, map[string]any{"online": online, "known": known, "sample": sample})
}

// ---------- computers ----------

type clientView struct {
	ClientInfo
	Online bool `json:"online"`
}

func (a *App) handleListClients(w http.ResponseWriter, r *http.Request, id *identity) {
	now := time.Now()
	a.store.mu.Lock()
	out := make([]clientView, 0, len(a.store.Clients))
	for _, c := range a.store.Clients {
		cp := *c
		if cp.Groups == nil {
			cp.Groups = []string{}
		}
		out = append(out, clientView{ClientInfo: cp, Online: c.online(now)})
	}
	a.store.mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Online != out[j].Online {
			return out[i].Online
		}
		return strings.ToLower(out[i].Hostname) < strings.ToLower(out[j].Hostname)
	})
	writeJSON(w, http.StatusOK, out)
}

func (a *App) handleForgetClient(w http.ResponseWriter, r *http.Request, id *identity) {
	a.store.mu.Lock()
	delete(a.store.Clients, r.PathValue("id"))
	a.store.dirtyClients = true
	a.store.mu.Unlock()
	a.save()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---------- saved groups ----------

func (a *App) handleListGroups(w http.ResponseWriter, r *http.Request, id *identity) {
	now := time.Now()
	type gv struct {
		*Group
		Online int `json:"online"`
		Known  int `json:"known"`
	}
	a.store.mu.Lock()
	out := make([]gv, 0, len(a.store.State.Groups))
	for _, g := range a.store.State.Groups {
		v := gv{Group: g}
		for _, c := range a.store.Clients {
			if rulesMatch(g.Rules, c) {
				v.Known++
				if c.online(now) {
					v.Online++
				}
			}
		}
		out = append(out, v)
	}
	b, _ := json.Marshal(out) // marshal under the lock: gv points at live data
	a.store.mu.Unlock()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(b)
}

func (a *App) handleSaveGroup(w http.ResponseWriter, r *http.Request, id *identity) {
	var req Group
	if !readJSON(w, r, &req, 64<<10) {
		return
	}
	req.Name = cleanText(req.Name, 60)
	req.Rules = cleanRules(req.Rules)
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "invalid", "Give the group a name.")
		return
	}
	if len(req.Rules) == 0 {
		writeErr(w, http.StatusBadRequest, "invalid", "Add at least one rule so the group matches some computers.")
		return
	}
	gid := r.PathValue("id")
	a.store.mu.Lock()
	for _, g := range a.store.State.Groups {
		if g.ID != gid && equalFold(g.Name, req.Name) {
			a.store.mu.Unlock()
			writeErr(w, http.StatusConflict, "exists", "There is already a group with that name.")
			return
		}
	}
	var g *Group
	if gid == "" {
		g = &Group{ID: newID()}
		a.store.State.Groups = append(a.store.State.Groups, g)
	} else if g = a.store.groupByID(gid); g == nil {
		a.store.mu.Unlock()
		writeErr(w, http.StatusNotFound, "not_found", "That group no longer exists.")
		return
	}
	g.Name, g.Rules = req.Name, req.Rules
	sort.Slice(a.store.State.Groups, func(i, j int) bool {
		return strings.ToLower(a.store.State.Groups[i].Name) < strings.ToLower(a.store.State.Groups[j].Name)
	})
	a.store.dirtyState = true
	out := *g
	a.store.mu.Unlock()
	a.save()
	writeJSON(w, http.StatusOK, out)
}

func (a *App) handleDeleteGroup(w http.ResponseWriter, r *http.Request, id *identity) {
	gid := r.PathValue("id")
	a.store.mu.Lock()
	gs := a.store.State.Groups[:0]
	for _, g := range a.store.State.Groups {
		if g.ID != gid {
			gs = append(gs, g)
		}
	}
	a.store.State.Groups = gs
	for _, u := range a.store.State.Users {
		u.AllowedGroupIDs = without(u.AllowedGroupIDs, gid)
	}
	a.store.dirtyState = true
	a.store.mu.Unlock()
	a.save()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func without(list []string, x string) []string {
	out := list[:0]
	for _, v := range list {
		if v != x {
			out = append(out, v)
		}
	}
	return out
}

// ---------- templates ----------

func (a *App) handleListTemplates(w http.ResponseWriter, r *http.Request, id *identity) {
	a.store.mu.Lock()
	b, _ := json.Marshal(a.store.State.Templates)
	a.store.mu.Unlock()
	if string(b) == "null" {
		b = []byte("[]")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(b)
}

func (a *App) handleSaveTemplate(w http.ResponseWriter, r *http.Request, id *identity) {
	var req Template
	if !readJSON(w, r, &req, 64<<10) {
		return
	}
	req.Name = cleanText(req.Name, 60)
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "invalid", "Give the template a name.")
		return
	}
	// Templates carry wording and appearance, never the audience - picking who
	// receives an alert should always be a deliberate act at send time.
	req.Spec.Target = Target{All: true}
	if msg := validateSpec(&req.Spec); msg != "" {
		writeErr(w, http.StatusBadRequest, "invalid", msg)
		return
	}
	req.Spec.Target = Target{GroupIDs: []string{}, Rules: []Rule{}}
	a.store.mu.Lock()
	var t *Template
	for _, existing := range a.store.State.Templates {
		if equalFold(existing.Name, req.Name) {
			t = existing
		}
	}
	if t == nil {
		t = &Template{ID: newID()}
		a.store.State.Templates = append(a.store.State.Templates, t)
	}
	t.Name, t.Spec = req.Name, req.Spec
	a.store.dirtyState = true
	out := *t
	a.store.mu.Unlock()
	a.save()
	writeJSON(w, http.StatusOK, out)
}

func (a *App) handleDeleteTemplate(w http.ResponseWriter, r *http.Request, id *identity) {
	tid := r.PathValue("id")
	a.store.mu.Lock()
	ts := a.store.State.Templates[:0]
	for _, t := range a.store.State.Templates {
		if t.ID != tid {
			ts = append(ts, t)
		}
	}
	a.store.State.Templates = ts
	a.store.dirtyState = true
	a.store.mu.Unlock()
	a.save()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---------- users (admin) ----------

func (a *App) handleListUsers(w http.ResponseWriter, r *http.Request, id *identity) {
	a.store.mu.Lock()
	out := make([]userView, 0, len(a.store.State.Users))
	for _, u := range a.store.State.Users {
		out = append(out, viewUser(u))
	}
	a.store.mu.Unlock()
	writeJSON(w, http.StatusOK, out)
}

type userRequest struct {
	Username        string   `json:"username"`
	DisplayName     string   `json:"display_name"`
	Role            string   `json:"role"`
	Disabled        bool     `json:"disabled"`
	AllowedGroupIDs []string `json:"allowed_group_ids"`
	ResetPassword   bool     `json:"reset_password"`
}

func (a *App) handleSaveUser(w http.ResponseWriter, r *http.Request, id *identity) {
	var req userRequest
	if !readJSON(w, r, &req, 16<<10) {
		return
	}
	uid := r.PathValue("id")
	req.Username = cleanText(req.Username, 64)
	req.DisplayName = cleanText(req.DisplayName, 80)
	if req.Role != "admin" && req.Role != "sender" {
		writeErr(w, http.StatusBadRequest, "invalid", "Role must be admin or sender.")
		return
	}
	if uid == "" && (req.Username == "" || strings.ContainsAny(req.Username, " :/\\")) {
		writeErr(w, http.StatusBadRequest, "invalid", "Usernames cannot be empty or contain spaces, colons or slashes.")
		return
	}

	st := a.store
	st.mu.Lock()
	var u *User
	tempPassword := ""
	if uid == "" {
		if st.userByName(req.Username) != nil {
			st.mu.Unlock()
			writeErr(w, http.StatusConflict, "exists", "That username is already taken.")
			return
		}
		u = &User{ID: newID(), Username: req.Username, CreatedAt: time.Now()}
		st.State.Users = append(st.State.Users, u)
		req.ResetPassword = true
	} else if u = st.userByID(uid); u == nil {
		st.mu.Unlock()
		writeErr(w, http.StatusNotFound, "not_found", "That account no longer exists.")
		return
	}
	// Never let the last working administrator be demoted or disabled.
	losingAdmin := u.Role == "admin" && !u.Disabled && (req.Role != "admin" || req.Disabled)
	if losingAdmin && st.adminCount() <= 1 {
		st.mu.Unlock()
		writeErr(w, http.StatusConflict, "last_admin", "This is the only administrator. Make someone else an administrator first.")
		return
	}
	u.DisplayName, u.Role, u.Disabled = req.DisplayName, req.Role, req.Disabled
	allowed := []string{}
	for _, gid := range req.AllowedGroupIDs {
		if st.groupByID(gid) != nil {
			allowed = append(allowed, gid)
		}
	}
	u.AllowedGroupIDs = allowed
	if req.ResetPassword {
		tempPassword = generatePassword()
		st.mu.Unlock()
		h := hashPassword(tempPassword) // slow on purpose; do it outside the lock
		st.mu.Lock()
		u.PassHash, u.MustChange = h, true
	}
	st.dirtyState = true
	v := viewUser(u)
	userID, disabled := u.ID, u.Disabled
	st.mu.Unlock()

	if req.ResetPassword || disabled {
		a.sessions.dropUser(userID)
	}
	a.save()
	log.Printf("account %s saved by %s (role=%s disabled=%v reset=%v)", v.Username, id.User.Username, v.Role, v.Disabled, req.ResetPassword)
	writeJSON(w, http.StatusOK, map[string]any{"user": v, "temp_password": tempPassword})
}

func (a *App) handleDeleteUser(w http.ResponseWriter, r *http.Request, id *identity) {
	uid := r.PathValue("id")
	if uid == id.User.ID {
		writeErr(w, http.StatusConflict, "self", "You cannot delete the account you are signed in with.")
		return
	}
	st := a.store
	st.mu.Lock()
	u := st.userByID(uid)
	if u != nil && u.Role == "admin" && !u.Disabled && st.adminCount() <= 1 {
		st.mu.Unlock()
		writeErr(w, http.StatusConflict, "last_admin", "This is the only administrator.")
		return
	}
	us := st.State.Users[:0]
	for _, x := range st.State.Users {
		if x.ID != uid {
			us = append(us, x)
		}
	}
	st.State.Users = us
	st.dirtyState = true
	st.mu.Unlock()
	a.sessions.dropUser(uid)
	a.save()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---------- settings & tokens (admin) ----------

func (a *App) handleGetSettings(w http.ResponseWriter, r *http.Request, id *identity) {
	a.store.mu.Lock()
	s := a.store.State.Settings
	type tv struct {
		ID        string    `json:"id"`
		Name      string    `json:"name"`
		CreatedBy string    `json:"created_by"`
		CreatedAt time.Time `json:"created_at"`
		LastUsed  time.Time `json:"last_used"`
	}
	toks := make([]tv, 0, len(a.store.State.Tokens))
	for _, t := range a.store.State.Tokens {
		toks = append(toks, tv{t.ID, t.Name, t.CreatedBy, t.CreatedAt, t.LastUsed})
	}
	a.store.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"org_name": s.OrgName, "client_key": s.ClientKey, "tokens": toks,
		"version": version, "data_dir": a.store.dir, "https": a.cfg.TLSCert != "",
	})
}

func (a *App) handleSaveSettings(w http.ResponseWriter, r *http.Request, id *identity) {
	var req struct {
		OrgName string `json:"org_name"`
	}
	if !readJSON(w, r, &req, 8<<10) {
		return
	}
	a.store.mu.Lock()
	a.store.State.Settings.OrgName = cleanText(req.OrgName, 80)
	a.store.dirtyState = true
	a.store.mu.Unlock()
	a.save()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleRotateKey(w http.ResponseWriter, r *http.Request, id *identity) {
	key := randomSecret(24)
	a.store.mu.Lock()
	a.store.State.Settings.ClientKey = key
	a.store.dirtyState = true
	a.store.mu.Unlock()
	a.save()
	log.Printf("client key rotated by %s", id.User.Username)
	writeJSON(w, http.StatusOK, map[string]string{"client_key": key})
}

func (a *App) handleCreateToken(w http.ResponseWriter, r *http.Request, id *identity) {
	var req struct {
		Name string `json:"name"`
	}
	if !readJSON(w, r, &req, 8<<10) {
		return
	}
	req.Name = cleanText(req.Name, 60)
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "invalid", "Name the token after what will use it, e.g. \"Fire panel script\".")
		return
	}
	secret := "fh_" + randomSecret(24)
	t := &Token{ID: newID(), Name: req.Name, Hash: hashToken(secret), CreatedBy: id.User.Username, CreatedAt: time.Now()}
	a.store.mu.Lock()
	a.store.State.Tokens = append(a.store.State.Tokens, t)
	a.store.dirtyState = true
	a.store.mu.Unlock()
	a.save()
	log.Printf("API token %q created by %s", req.Name, id.User.Username)
	writeJSON(w, http.StatusCreated, map[string]string{"id": t.ID, "name": t.Name, "token": secret})
}

func (a *App) handleDeleteToken(w http.ResponseWriter, r *http.Request, id *identity) {
	tid := r.PathValue("id")
	a.store.mu.Lock()
	ts := a.store.State.Tokens[:0]
	for _, t := range a.store.State.Tokens {
		if t.ID != tid {
			ts = append(ts, t)
		}
	}
	a.store.State.Tokens = ts
	a.store.dirtyState = true
	a.store.mu.Unlock()
	a.save()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
