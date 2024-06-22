// Package httpapi wires cards-api's routes: chi for the mux,
// cauri-go-kit/auth for the Bearer guard on every /v1 route (plus
// RequireRole("service") on POST /v1/authorizations, which only
// service-to-service callers use), cauri-go-kit/metrics for /metrics.
package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/demo-org-np-migration/cauri-go-kit/auth"

	"github.com/demo-org-np-migration/cards-api/internal/ledger"
	"github.com/demo-org-np-migration/cards-api/internal/metrics"
	"github.com/demo-org-np-migration/cards-api/internal/processor"
	"github.com/demo-org-np-migration/cards-api/internal/store"
)

// NewRouter builds the full mux: /health and /metrics are public, every
// /v1 route requires a valid Bearer token for keycloakIssuer, and
// /v1/authorizations additionally requires the "service" role.
func NewRouter(st store.Store, proc processor.Processor, ledgerClient ledger.Client, logger *slog.Logger, keycloakIssuer string) http.Handler {
	h := &Handlers{Store: st, Processor: proc, Ledger: ledgerClient, Logger: logger}

	r := chi.NewRouter()
	r.Use(requestLogging(logger))
	r.Use(metrics.Middleware)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Handle("/metrics", metrics.Handler())

	r.Route("/v1", func(r chi.Router) {
		r.Use(auth.Middleware(keycloakIssuer))

		r.Post("/cards", h.CreateCard)
		r.Get("/cards/{id}", h.GetCard)
		r.Post("/cards/{id}/block", h.BlockCard)
		r.Get("/accounts/{account_id}/cards", h.ListCardsByAccount)

		r.With(auth.RequireRole("service")).Post("/authorizations", h.CreateAuthorization)
	})

	return r
}
