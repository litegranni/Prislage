package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"prislage/internal/config"
	"prislage/internal/service"
	"prislage/internal/store"
)

type API struct {
	Svc    *service.Service
	Cfg    config.Config
	Stores *store.Stores
}

func (a *API) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(a.cors)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/api/vapid-public-key", a.vapidPublicKey)

	r.Group(func(r chi.Router) {
		r.Use(a.requireToken)
		r.Get("/api/items", a.listItems)
		r.Post("/api/items", a.createItem)
		r.Get("/api/items/{id}", a.getItem)
		r.Patch("/api/items/{id}", a.patchItem)
		r.Delete("/api/items/{id}", a.deleteItem)
		r.Get("/api/items/{id}/prices", a.listPrices)
		r.Get("/api/items/{id}/notifications", a.listNotifications)
		r.Post("/api/capture", a.capture)
		r.Post("/api/analyze", a.analyze)
		r.Post("/api/watch", a.watchFromAnalyze)
		r.Get("/api/stores", a.listStores)
		r.Post("/api/push/subscribe", a.pushSubscribe)
		r.Delete("/api/push/subscribe", a.pushUnsubscribe)
	})
	return r
}

func (a *API) cors(next http.Handler) http.Handler {
	allowed := parseCORSAllowlist(a.Cfg.CORSOrigin)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqOrigin := r.Header.Get("Origin")
		if reqOrigin != "" && originAllowed(reqOrigin, allowed) {
			w.Header().Set("Access-Control-Allow-Origin", reqOrigin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			if reqOrigin != "" && !originAllowed(reqOrigin, allowed) {
				writeErr(w, http.StatusForbidden, "origin not allowed")
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func parseCORSAllowlist(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "*" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" && p != "*" {
			out = append(out, strings.TrimRight(p, "/"))
		}
	}
	return out
}

func originAllowed(origin string, allowlist []string) bool {
	if len(allowlist) == 0 {
		return false
	}
	origin = strings.TrimRight(origin, "/")
	for _, a := range allowlist {
		if origin == a {
			return true
		}
	}
	return false
}

func (a *API) requireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == "" {
			token = r.Header.Get("X-API-Token")
		}
		if token == "" {
			token = r.URL.Query().Get("token")
		}
		if token != a.Cfg.APIToken {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) listItems(w http.ResponseWriter, r *http.Request) {
	items, err := a.Svc.Items.ListActive(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if items == nil {
		items = []store.ItemSummary{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *API) createItem(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL         string          `json:"url"`
		TargetPrice decimal.Decimal `json:"target_price"`
		Category    string          `json:"category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.URL == "" || body.TargetPrice.IsZero() {
		writeErr(w, http.StatusBadRequest, "url and target_price required")
		return
	}
	item, err := a.Svc.CreateFromURL(r.Context(), service.CreateFromURLInput{
		URL: body.URL, TargetPrice: body.TargetPrice, Category: body.Category,
	})
	if err != nil {
		if service.IsCaptureOnly(err) {
			writeErr(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) getItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	item, err := a.Svc.Items.Get(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) patchItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Name        *string          `json:"name"`
		Category    *string          `json:"category"`
		TargetPrice *decimal.Decimal `json:"target_price"`
		Active      *bool            `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	item, err := a.Svc.Items.Update(r.Context(), id, store.UpdateItemParams{
		Name: body.Name, Category: body.Category, TargetPrice: body.TargetPrice, Active: body.Active,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) deleteItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := a.Svc.Items.SoftDelete(r.Context(), id); errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listPrices(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	prices, err := a.Svc.Items.ListPrices(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if prices == nil {
		prices = nil
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, prices)
}

func (a *API) listNotifications(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	list, err := a.Svc.Items.ListNotifications(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) analyze(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	out, err := a.Svc.Analyze(r.Context(), body.Query)
	if err != nil {
		msg := err.Error()
		status := http.StatusBadRequest
		if strings.Contains(msg, "ANTHROPIC_API_KEY") {
			status = http.StatusServiceUnavailable
		} else if strings.Contains(msg, "anthropic") || strings.Contains(msg, "schema:") {
			status = http.StatusBadGateway
		}
		writeErr(w, status, msg)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) watchFromAnalyze(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL          string           `json:"url"`
		Name         string           `json:"name"`
		Brand        string           `json:"brand"`
		GTIN         string           `json:"gtin"`
		TargetPrice  decimal.Decimal  `json:"target_price"`
		CurrentPrice *decimal.Decimal `json:"current_price"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	cur := decimal.Zero
	if body.CurrentPrice != nil {
		cur = *body.CurrentPrice
	}
	item, err := a.Svc.WatchFromAnalyze(r.Context(), service.WatchFromAnalyzeInput{
		URL: body.URL, Name: body.Name, Brand: body.Brand, GTIN: body.GTIN,
		TargetPrice: body.TargetPrice, CurrentPrice: cur,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) capture(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL         string           `json:"url"`
		HTML        string           `json:"html"`
		JSONLD      string           `json:"jsonld"`
		TargetPrice *decimal.Decimal `json:"target_price"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	item, err := a.Svc.Capture(r.Context(), service.CaptureInput{
		URL: body.URL, HTML: body.HTML, JSONLD: body.JSONLD, TargetPrice: body.TargetPrice,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) listStores(w http.ResponseWriter, r *http.Request) {
	list, err := a.Stores.List(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) vapidPublicKey(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"publicKey": a.Cfg.VAPIDPublicKey})
}

func (a *API) pushSubscribe(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Endpoint == "" {
		writeErr(w, http.StatusBadRequest, "invalid subscription")
		return
	}
	if err := a.Svc.Items.SavePushSubscription(r.Context(), body.Endpoint, body.Keys.P256dh, body.Keys.Auth); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (a *API) pushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Endpoint string `json:"endpoint"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Endpoint == "" {
		writeErr(w, http.StatusBadRequest, "endpoint required")
		return
	}
	_ = a.Svc.Items.DeletePushSubscription(r.Context(), body.Endpoint)
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
