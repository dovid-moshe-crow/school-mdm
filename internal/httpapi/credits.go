package httpapi

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dwdmsh/school-mdm/internal/activity"
	"github.com/dwdmsh/school-mdm/internal/credits"
	"github.com/dwdmsh/school-mdm/internal/nedarim"
	"github.com/dwdmsh/school-mdm/internal/store"
)

func (a *API) handleCreditBalance(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credits unavailable"})
		return
	}
	enrollment := enrollmentFromRequest(r)
	if enrollment == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "enrollment_id is required"})
		return
	}
	bal, err := a.Credits.Balance(r.Context(), enrollment)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enrollment_id":      bal.EnrollmentID,
		"balance":            bal.Balance,
		"allotment_balance":  bal.AllotmentBalance,
		"available":          bal.Available(),
		"access_cost":        a.Credits.AccessRequestCost(r.Context()),
		"enabled":            a.Credits.CreditsEnabled(r.Context()),
		"updated_at":         bal.UpdatedAt,
	})
}

func (a *API) handleCreditSettings(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credits unavailable"})
		return
	}
	settings, err := a.Credits.GetSettings(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_request_cost": settings.AccessRequestCost,
		"enabled":             settings.Enabled,
	})
}

func (a *API) handleCreditPackages(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credits unavailable"})
		return
	}
	pkgs, err := a.Credits.Packages(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	if pkgs == nil {
		pkgs = []store.CreditPackage{}
	}
	writeJSON(w, http.StatusOK, pkgs)
}

type checkoutBody struct {
	PackageID    string `json:"package_id"`
	EnrollmentID string `json:"enrollment_id"`
}

func (a *API) handleCreditCheckout(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credits unavailable"})
		return
	}
	if !a.Credits.CreditsEnabled(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "credit purchases disabled"})
		return
	}
	var body checkoutBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	enrollment := strings.TrimSpace(body.EnrollmentID)
	if enrollment == "" {
		enrollment = enrollmentFromRequest(r)
	}
	result, err := a.Credits.StartCheckout(r.Context(), enrollment, body.PackageID)
	if err != nil {
		a.audit(r, activity.Event{
			Category:     store.ActivityCategoryCredits,
			Action:       "checkout_start",
			ActorType:    store.ActivityActorDevice,
			Actor:        enrollment,
			EnrollmentID: enrollment,
			Result:       store.ActivityResultError,
			Summary:      "התחלת רכישת קרדיטים נכשלה",
			Detail:       map[string]any{"package_id": body.PackageID, "error": err.Error()},
		})
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	a.audit(r, activity.Event{
		Category:     store.ActivityCategoryCredits,
		Action:       "checkout_start",
		ActorType:    store.ActivityActorDevice,
		Actor:        enrollment,
		EnrollmentID: enrollment,
		Result:       store.ActivityResultOK,
		Summary:      "התחילה רכישת קרדיטים",
		Detail: map[string]any{
			"purchase_id":   result.Purchase.ID,
			"credits":       result.Purchase.Credits,
			"amount_agorot": result.Purchase.AmountAgorot,
			"mode":          result.Mode,
		},
	})
	writeJSON(w, http.StatusCreated, map[string]any{
		"purchase_id":   result.Purchase.ID,
		"iframe_url":    result.IframeURL,
		"mode":          result.Mode,
		"credits":       result.Purchase.Credits,
		"amount_agorot": result.Purchase.AmountAgorot,
	})
}

type confirmBody struct {
	PurchaseID   string `json:"purchase_id"`
	EnrollmentID string `json:"enrollment_id"`
}

func (a *API) handleCreditConfirm(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credits unavailable"})
		return
	}
	var body confirmBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	enrollment := strings.TrimSpace(body.EnrollmentID)
	if enrollment == "" {
		enrollment = enrollmentFromRequest(r)
	}
	p, bal, err := a.Credits.ConfirmPayment(r.Context(), body.PurchaseID, enrollment)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"purchase":           p,
		"balance":            bal.Balance,
		"allotment_balance":  bal.AllotmentBalance,
		"available":          bal.Available(),
	})
}

func (a *API) handleFakeIframe(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil || a.Cfg.NedarimMode != nedarim.ModeFake {
		http.Error(w, "fake iframe only available when NEDARIM_MODE=fake", http.StatusNotFound)
		return
	}
	token := strings.TrimSpace(r.URL.Query().Get("t"))
	if token == "" {
		http.Error(w, "missing t", http.StatusBadRequest)
		return
	}
	p, err := a.Credits.GetPurchaseByClientUnique(r.Context(), token)
	if err != nil {
		http.Error(w, "purchase not found", http.StatusNotFound)
		return
	}
	amountILS := fmt.Sprintf("%.2f", float64(p.AmountAgorot)/100.0)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, fakeIframeHTML,
		html.EscapeString(amountILS),
		p.Credits,
		token,
		p.ID,
	)
}

func (a *API) handleFakePay(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil || a.Cfg.NedarimMode != nedarim.ModeFake {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "fake pay disabled"})
		return
	}
	var body struct {
		Token string `json:"t"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	token := strings.TrimSpace(body.Token)
	if token == "" {
		token = strings.TrimSpace(r.URL.Query().Get("t"))
	}
	if token == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "t is required"})
		return
	}
	p, err := a.Credits.FakePay(r.Context(), token)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"purchase_id": p.ID,
		"status":      p.Status,
		"credits":     p.Credits,
	})
}

func (a *API) handleNedarimBridge(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil || a.Cfg.NedarimMode != nedarim.ModeLive {
		http.Error(w, "nedarim bridge only available when NEDARIM_MODE=live", http.StatusNotFound)
		return
	}
	token := strings.TrimSpace(r.URL.Query().Get("t"))
	if token == "" {
		http.Error(w, "missing t", http.StatusBadRequest)
		return
	}
	params, p, err := a.Credits.LiveBridgeParams(r.Context(), token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, nedarimBridgeHTML,
		html.EscapeString(fmt.Sprintf("%.2f", float64(p.AmountAgorot)/100.0)),
		p.Credits,
		html.EscapeString(nedarim.IframeEmbedURL()),
		html.EscapeString(params["Mosad"]),
		html.EscapeString(params["ApiValid"]),
		html.EscapeString(params["Amount"]),
		html.EscapeString(params["Param2"]),
		html.EscapeString(params["CallBack"]),
		html.EscapeString(params["Comment"]),
		html.EscapeString(params["TransactionId"]),
		html.EscapeString(p.ID),
	)
}

func (a *API) handleNedarimWebhook(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credits unavailable"})
		return
	}
	payload, err := parseNedarimWebhook(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	p, err := a.Credits.HandleWebhook(r.Context(), payload)
	if err != nil {
		if a.Log != nil {
			a.Log.Warn("nedarim webhook", "err", err)
		}
		a.audit(r, activity.Event{
			Category:     store.ActivityCategoryCredits,
			Action:       "nedarim_webhook",
			ActorType:    store.ActivityActorWebhook,
			Actor:        "nedarim",
			EnrollmentID: "",
			Result:       store.ActivityResultError,
			Summary:      "Webhook נדרים נכשל",
			Detail:       map[string]any{"client_unique_id": payload.ClientUniqueID, "error": err.Error()},
		})
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	action := "nedarim_paid"
	summary := "תשלום נדרים התקבל — קרדיטים נוספו"
	result := store.ActivityResultOK
	status := strings.ToLower(strings.TrimSpace(payload.Status))
	if status == "error" || status == "fail" || status == "failed" || status == "refusal" || status == "refused" {
		action = "nedarim_refused"
		summary = "תשלום נדרים נדחה / נכשל"
		result = store.ActivityResultInfo
	} else if p.Status != store.PurchasePaid {
		action = "nedarim_webhook"
		summary = "Webhook נדרים התקבל ללא חיוב"
		result = store.ActivityResultInfo
	}
	a.audit(r, activity.Event{
		Category:     store.ActivityCategoryCredits,
		Action:       action,
		ActorType:    store.ActivityActorWebhook,
		Actor:        "nedarim",
		EnrollmentID: p.EnrollmentID,
		Result:       result,
		Summary:      summary,
		Detail: map[string]any{
			"purchase_id":       p.ID,
			"credits":           p.Credits,
			"client_unique_id":  payload.ClientUniqueID,
			"transaction_id":    payload.TransactionID,
			"status":            payload.Status,
			"purchase_status":   p.Status,
		},
	})
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "purchase_id": p.ID})
}

func parseNedarimWebhook(r *http.Request) (credits.WebhookPayload, error) {
	ct := r.Header.Get("Content-Type")
	raw := map[string]any{}
	payload := credits.WebhookPayload{Raw: raw}

	if strings.Contains(ct, "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			return payload, fmt.Errorf("invalid JSON")
		}
	} else {
		if err := r.ParseForm(); err != nil {
			return payload, fmt.Errorf("invalid form")
		}
		for k, vals := range r.Form {
			if len(vals) > 0 {
				raw[k] = vals[0]
			}
		}
	}

	get := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := raw[k]; ok {
				switch t := v.(type) {
				case string:
					if strings.TrimSpace(t) != "" {
						return strings.TrimSpace(t)
					}
				case float64:
					return strconv.FormatInt(int64(t), 10)
				}
			}
		}
		return ""
	}

	payload.ClientUniqueID = get("ClientUniqueId", "clientUniqueId", "Param2", "param2", "CallbackParam", "ClientUniqueID")
	payload.TransactionID = get("TransactionId", "transactionId", "Id", "id", "ID")
	payload.Amount = get("Amount", "amount")
	payload.Status = get("Status", "status", "Result", "result")
	if payload.ClientUniqueID == "" {
		return payload, fmt.Errorf("missing ClientUniqueId / Param2")
	}
	return payload, nil
}

type giftBody struct {
	EnrollmentID string `json:"enrollment_id"`
	Amount       int    `json:"amount"`
	Note         string `json:"note"`
}

func (a *API) handleAdminGiftCredits(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credits unavailable"})
		return
	}
	var body giftBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	res, err := a.Credits.Gift(r.Context(), body.EnrollmentID, body.Amount, body.Note)
	actorType, actor := a.adminActor(r)
	if err != nil {
		a.audit(r, activity.Event{
			Category: store.ActivityCategoryCredits, Action: "credit_gift",
			ActorType: actorType, Actor: actor, EnrollmentID: body.EnrollmentID,
			Result: store.ActivityResultError, Summary: "הענקת קרדיטים נכשלה",
			Detail: map[string]any{"amount": body.Amount, "error": err.Error()},
		})
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	a.audit(r, activity.Event{
		Category: store.ActivityCategoryCredits, Action: "credit_gift",
		ActorType: actorType, Actor: actor, EnrollmentID: body.EnrollmentID,
		Result: store.ActivityResultOK, Summary: "הוענקו קרדיטים למכשיר",
		Detail: map[string]any{"amount": body.Amount, "note": body.Note, "balance": res.Balance},
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"enrollment_id":      body.EnrollmentID,
		"balance":            res.Balance,
		"allotment_balance":  res.AllotmentBalance,
		"available":          res.Balance + res.AllotmentBalance,
		"applied":            res.Applied,
		"entry":              res.Entry,
	})
}

type adjustBody struct {
	EnrollmentID string `json:"enrollment_id"`
	Amount       int    `json:"amount"`
	Note         string `json:"note"`
}

func (a *API) handleAdminAdjustCredits(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credits unavailable"})
		return
	}
	var body adjustBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	res, err := a.Credits.Adjust(r.Context(), body.EnrollmentID, body.Amount, body.Note)
	actorType, actor := a.adminActor(r)
	if err != nil {
		a.audit(r, activity.Event{
			Category: store.ActivityCategoryCredits, Action: "credit_adjust",
			ActorType: actorType, Actor: actor, EnrollmentID: body.EnrollmentID,
			Result: store.ActivityResultError, Summary: "התאמת קרדיטים נכשלה",
			Detail: map[string]any{"amount": body.Amount, "error": err.Error()},
		})
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	a.audit(r, activity.Event{
		Category: store.ActivityCategoryCredits, Action: "credit_adjust",
		ActorType: actorType, Actor: actor, EnrollmentID: body.EnrollmentID,
		Result: store.ActivityResultOK, Summary: "התאמת קרדיטים במכשיר",
		Detail: map[string]any{"amount": body.Amount, "note": body.Note, "balance": res.Balance},
	})
	ledger, _ := a.Credits.Ledger(r.Context(), body.EnrollmentID, 20)
	if ledger == nil {
		ledger = []store.CreditLedgerEntry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enrollment_id":      body.EnrollmentID,
		"balance":            res.Balance,
		"allotment_balance":  res.AllotmentBalance,
		"available":          res.Balance + res.AllotmentBalance,
		"applied":            res.Applied,
		"entry":              res.Entry,
		"ledger":             ledger,
	})
}

func (a *API) handleAdminListCredits(w http.ResponseWriter, r *http.Request) {
	if a.Store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "store unavailable"})
		return
	}
	if enrollment := strings.TrimSpace(r.URL.Query().Get("enrollment_id")); enrollment != "" {
		bal, err := a.Store.GetCreditBalance(r.Context(), enrollment)
		if err != nil {
			writeErr(w, err)
			return
		}
		limit := 20
		if v := strings.TrimSpace(r.URL.Query().Get("ledger_limit")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
			}
		}
		ledger, err := a.Store.ListCreditLedger(r.Context(), enrollment, limit)
		if err != nil {
			writeErr(w, err)
			return
		}
		if ledger == nil {
			ledger = []store.CreditLedgerEntry{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"enrollment_id":     bal.EnrollmentID,
			"balance":           bal.Balance,
			"allotment_balance": bal.AllotmentBalance,
			"available":         bal.Available(),
			"updated_at":        bal.UpdatedAt,
			"ledger":            ledger,
		})
		return
	}
	list, err := a.Store.ListCreditBalances(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	if list == nil {
		list = []store.DeviceCredits{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) handleAdminListPurchases(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.CreditPurchaseFilter{
		EnrollmentID: strings.TrimSpace(q.Get("enrollment_id")),
		Status:       strings.TrimSpace(q.Get("status")),
	}
	if v := strings.TrimSpace(q.Get("limit")); v != "" {
		f.Limit, _ = strconv.Atoi(v)
	}
	if v := strings.TrimSpace(q.Get("offset")); v != "" {
		f.Offset, _ = strconv.Atoi(v)
	}
	purchases, err := a.Store.ListCreditPurchases(r.Context(), f)
	if err != nil {
		writeErr(w, err)
		return
	}
	type row struct {
		store.CreditPurchase
		DeviceName  string `json:"device_name,omitempty"`
		PackageName string `json:"package_name,omitempty"`
	}
	out := make([]row, 0, len(purchases))
	for _, p := range purchases {
		item := row{CreditPurchase: p}
		if d, err := a.Store.GetDevice(r.Context(), p.EnrollmentID); err == nil {
			item.DeviceName = d.Name
		}
		if pkg, err := a.Store.GetCreditPackage(r.Context(), p.PackageID); err == nil {
			item.PackageName = pkg.NameHe
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"purchases": out,
		"limit":     f.Limit,
		"offset":    f.Offset,
	})
}

func (a *API) handleAdminCreditLedger(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credits unavailable"})
		return
	}
	enrollment := strings.TrimSpace(r.URL.Query().Get("enrollment_id"))
	if enrollment == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "enrollment_id is required"})
		return
	}
	limit := 20
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	bal, err := a.Credits.Balance(r.Context(), enrollment)
	if err != nil {
		writeErr(w, err)
		return
	}
	ledger, err := a.Credits.Ledger(r.Context(), enrollment, limit)
	if err != nil {
		writeErr(w, err)
		return
	}
	if ledger == nil {
		ledger = []store.CreditLedgerEntry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enrollment_id":     bal.EnrollmentID,
		"balance":           bal.Balance,
		"allotment_balance": bal.AllotmentBalance,
		"available":         bal.Available(),
		"updated_at":        bal.UpdatedAt,
		"ledger":            ledger,
	})
}

func (a *API) handleAdminGetCreditSettings(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credits unavailable"})
		return
	}
	settings, err := a.Credits.GetSettings(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *API) handleAdminPutCreditSettings(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credits unavailable"})
		return
	}
	var body struct {
		AccessRequestCost int   `json:"access_request_cost"`
		Enabled           *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	settings, err := a.Credits.UpdateSettings(r.Context(), body.AccessRequestCost, body.Enabled)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *API) handleAdminListPackages(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credits unavailable"})
		return
	}
	pkgs, err := a.Credits.AdminPackages(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	if pkgs == nil {
		pkgs = []store.CreditPackage{}
	}
	writeJSON(w, http.StatusOK, pkgs)
}

func (a *API) handleAdminCreatePackage(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credits unavailable"})
		return
	}
	var body struct {
		NameHe      string `json:"name_he"`
		Credits     int    `json:"credits"`
		PriceAgorot int    `json:"price_agorot"`
		Active      *bool  `json:"active"`
		SortOrder   int    `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	active := true
	if body.Active != nil {
		active = *body.Active
	}
	created, err := a.Credits.CreatePackage(r.Context(), store.CreditPackage{
		NameHe:      body.NameHe,
		Credits:     body.Credits,
		PriceAgorot: body.PriceAgorot,
		Active:      active,
		SortOrder:   body.SortOrder,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *API) handleAdminUpdatePackage(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credits unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}
	existing, err := a.Store.GetCreditPackage(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var body struct {
		NameHe      *string `json:"name_he"`
		Credits     *int    `json:"credits"`
		PriceAgorot *int    `json:"price_agorot"`
		Active      *bool   `json:"active"`
		SortOrder   *int    `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if body.NameHe != nil {
		existing.NameHe = *body.NameHe
	}
	if body.Credits != nil {
		existing.Credits = *body.Credits
	}
	if body.PriceAgorot != nil {
		existing.PriceAgorot = *body.PriceAgorot
	}
	if body.Active != nil {
		existing.Active = *body.Active
	}
	if body.SortOrder != nil {
		existing.SortOrder = *body.SortOrder
	}
	updated, err := a.Credits.UpdatePackage(r.Context(), existing)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) handleAdminDeletePackage(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credits unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}
	pkg, err := a.Credits.DeactivatePackage(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, pkg)
}

func (a *API) handleAdminListAllotments(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credits unavailable"})
		return
	}
	list, err := a.Credits.ListAllotmentRulesView(r.Context(), time.Now().UTC())
	if err != nil {
		writeErr(w, err)
		return
	}
	if list == nil {
		list = []credits.AllotmentRuleView{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) handleAdminCreateAllotment(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credits unavailable"})
		return
	}
	var body struct {
		Name       string `json:"name"`
		Note       string `json:"note"`
		Amount     int    `json:"amount"`
		Interval   string `json:"interval"`
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
		Enabled    *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	created, err := a.Credits.CreateAllotmentRule(r.Context(), store.CreditAllotmentRule{
		Name:       body.Name,
		Note:       body.Note,
		Amount:     body.Amount,
		Interval:   store.AllotmentInterval(body.Interval),
		TargetType: store.AllotmentTargetType(body.TargetType),
		TargetID:   body.TargetID,
		Enabled:    enabled,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *API) handleAdminUpdateAllotment(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credits unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}
	existing, err := a.Credits.GetAllotmentRule(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var body struct {
		Name       *string `json:"name"`
		Note       *string `json:"note"`
		Amount     *int    `json:"amount"`
		Interval   *string `json:"interval"`
		TargetType *string `json:"target_type"`
		TargetID   *string `json:"target_id"`
		Enabled    *bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if body.Name != nil {
		existing.Name = *body.Name
	}
	if body.Note != nil {
		existing.Note = *body.Note
	}
	if body.Amount != nil {
		existing.Amount = *body.Amount
	}
	if body.Interval != nil {
		existing.Interval = store.AllotmentInterval(*body.Interval)
	}
	if body.TargetType != nil {
		existing.TargetType = store.AllotmentTargetType(*body.TargetType)
	}
	if body.TargetID != nil {
		existing.TargetID = *body.TargetID
	}
	if body.Enabled != nil {
		existing.Enabled = *body.Enabled
	}
	updated, err := a.Credits.UpdateAllotmentRule(r.Context(), existing)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) handleAdminDeleteAllotment(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credits unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}
	if err := a.Credits.DeleteAllotmentRule(r.Context(), id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "deleted"})
}

func (a *API) handleAdminRunAllotments(w http.ResponseWriter, r *http.Request) {
	if a.Credits == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credits unavailable"})
		return
	}
	result, err := a.Credits.RunAllotments(r.Context(), time.Now().UTC())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func enrollmentFromRequest(r *http.Request) string {
	if v := strings.TrimSpace(r.URL.Query().Get("enrollment_id")); v != "" {
		return v
	}
	if v := strings.TrimSpace(r.Header.Get("X-Device-ID")); v != "" {
		return v
	}
	return ""
}

const fakeIframeHTML = `<!DOCTYPE html>
<html lang="he" dir="rtl">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<title>סימולציית נדרים פלוס</title>
<style>
body{font-family:Assistant,Rubik,Arial,sans-serif;margin:0;padding:24px;background:linear-gradient(160deg,#f4faf7,#e8f2ec);color:#123}
.box{max-width:360px;margin:0 auto;background:#fff;border:1px solid #d7e5dd;border-radius:12px;padding:20px}
h1{font-size:1.15rem;margin:0 0 8px}
.muted{color:#567;font-size:.9rem;margin-bottom:16px}
.row{display:flex;justify-content:space-between;margin:8px 0;font-size:.95rem}
.btns{display:flex;gap:8px;margin-top:18px}
button{flex:1;padding:10px 12px;border-radius:8px;border:0;font:inherit;cursor:pointer}
.pay{background:#0b6e4f;color:#fff}
.cancel{background:#eef2f0;color:#234}
.err{color:#b00020;margin-top:10px;font-size:.85rem;display:none}
</style>
</head>
<body>
<div class="box">
  <h1>סימולציית נדרים פלוס</h1>
  <p class="muted">מצב בדיקה מקומי — לא מחויב באשראי אמיתי.</p>
  <div class="row"><span>סכום</span><strong>₪%s</strong></div>
  <div class="row"><span>קרדיטים</span><strong>%d</strong></div>
  <div class="btns">
    <button class="pay" id="pay">תשלום</button>
    <button class="cancel" id="cancel">ביטול</button>
  </div>
  <div class="err" id="err"></div>
</div>
<script>
const token = %q;
const purchaseId = %q;
document.getElementById('pay').onclick = async function() {
  const err = document.getElementById('err');
  err.style.display = 'none';
  try {
    const res = await fetch('/api/credits/fake-pay', {
      method: 'POST',
      headers: {'Content-Type':'application/json'},
      body: JSON.stringify({t: token})
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || 'תשלום נכשל');
    window.parent.postMessage({
      type: 'nedarim-success',
      Name: 'TransactionResponse',
      Value: { Status: 'OK', purchase_id: purchaseId, ClientUniqueId: token }
    }, '*');
  } catch (e) {
    err.textContent = e.message || String(e);
    err.style.display = 'block';
    window.parent.postMessage({ type: 'nedarim-error', error: e.message || String(e) }, '*');
  }
};
document.getElementById('cancel').onclick = function() {
  window.parent.postMessage({ type: 'nedarim-cancel' }, '*');
};
</script>
</body>
</html>
`

const nedarimBridgeHTML = `<!DOCTYPE html>
<html lang="he" dir="rtl">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<title>נדרים פלוס</title>
<style>
body{margin:0;padding:12px;font-family:Assistant,Arial,sans-serif;background:#f7faf8;color:#123}
.meta{display:flex;justify-content:space-between;gap:12px;margin:0 0 10px;font-size:.95rem}
iframe{border:1px solid #d7e5dd;border-radius:8px;width:100%%;min-height:280px;background:#fff}
.btns{display:flex;gap:8px;margin-top:12px}
button{flex:1;padding:12px;border:0;border-radius:8px;font:inherit;cursor:pointer}
.pay{background:#0b6e4f;color:#fff}
.cancel{background:#eef2f0;color:#234}
.err{color:#b00020;margin-top:8px;font-size:.9rem;min-height:1.2em}
.wait{display:none;text-align:center;color:#567;margin-top:8px;font-size:.9rem}
</style>
</head>
<body>
<div class="meta"><span>סכום לתשלום</span><strong>₪%s · %d קרדיטים</strong></div>
<iframe id="NedarimFrame" src="%s"></iframe>
<div class="btns">
  <button class="pay" id="pay" type="button">ביצוע תשלום</button>
  <button class="cancel" id="cancel" type="button">ביטול</button>
</div>
<div class="wait" id="wait">מבצע חיוב, נא להמתין…</div>
<div class="err" id="err"></div>
<script>
const mosad=%q, apiValid=%q, amount=%q, param2=%q, callback=%q, comment=%q, transactionId=%q, purchaseId=%q;
function PostNedarim(Data){
  document.getElementById('NedarimFrame').contentWindow.postMessage(Data, '*');
}
function setBusy(busy){
  document.getElementById('pay').disabled = busy;
  document.getElementById('wait').style.display = busy ? 'block' : 'none';
}
window.addEventListener('message', function(event){
  if (!event.data) return;
  if (event.data.Name === 'Height') {
    document.getElementById('NedarimFrame').style.height = (parseInt(event.data.Value,10)+15)+'px';
  }
  if (event.data.Name === 'TransactionResponse') {
    setBusy(false);
    const value = event.data.Value || {};
    if (value.BackMessage === 'NEED CAPTCHA' || value.BackMessage === 'CAPTCHA ERROR') {
      document.getElementById('err').textContent = 'יש לאמת CAPTCHA ואז ללחוץ שוב על ביצוע תשלום';
      return;
    }
    const failed = value.Status === 'Error';
    if (failed) {
      document.getElementById('err').textContent = value.Message || 'התשלום נכשל';
    }
    window.parent.postMessage({
      type: failed ? 'nedarim-error' : 'nedarim-success',
      Name: 'TransactionResponse',
      Value: Object.assign({}, value, { purchase_id: purchaseId, ClientUniqueId: param2 }),
      error: failed ? (value.Message || 'התשלום נכשל') : undefined
    }, '*');
  }
});
document.getElementById('NedarimFrame').onload = function(){
  PostNedarim({Name:'GetHeight'});
};
document.getElementById('pay').onclick = function(){
  document.getElementById('err').textContent = '';
  setBusy(true);
  // PCI iframe only collects card fields; parent must charge after the user fills them
  // (see matara.pro/nedarimplus/iframe/sample2.html). Prefer CreateTransaction id when present.
  if (transactionId) {
    PostNedarim({Name:'FinishTransaction', Value: transactionId});
    return;
  }
  PostNedarim({
    Name:'FinishTransaction2',
    Value:{
      Mosad: mosad, ApiValid: apiValid, PaymentType:'Ragil', Currency:'1',
      Amount: amount, Tashlumim:'1', Param2: param2, CallBack: callback, Comment: comment,
      FirstName:'', LastName:'', Street:'', City:'', Phone:'', Mail:'', Zeout:'',
      Groupe:'', CallBackMailError:'', Param1:'', Day:'', ThirdPartyReceipt:'', ForceUpdateMatching:''
    }
  });
};
document.getElementById('cancel').onclick = function(){
  window.parent.postMessage({ type: 'nedarim-cancel' }, '*');
};
</script>
</body>
</html>
`
