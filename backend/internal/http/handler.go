package httpapi

import (
	"encoding/json"
	"net/http"

	"mvp-push-gateway/backend/internal/config"
)

type healthResponse struct {
	Status      string `json:"status"`
	AppName     string `json:"app_name"`
	Environment string `json:"environment"`
	APIPrefix   string `json:"api_prefix"`
}

func NewHandler(cfg config.Config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(cfg.Server.APIPrefix+"/health", healthHandler(cfg))
	return mux
}

func healthHandler(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		writeJSON(w, http.StatusOK, healthResponse{
			Status:      "ok",
			AppName:     cfg.App.Name,
			Environment: cfg.App.Environment,
			APIPrefix:   cfg.Server.APIPrefix,
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
