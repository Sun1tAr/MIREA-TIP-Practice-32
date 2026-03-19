package health

import (
	"encoding/json"
	"net/http"
	"time"
)

type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Service   string    `json:"service,omitempty"`
	Version   string    `json:"version,omitempty"`
}

// HealthHandler возвращает 200 OK с базовой информацией
func HealthHandler(serviceName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := HealthResponse{
			Status:    "ok",
			Timestamp: time.Now().UTC(),
			Service:   serviceName,
		}

		json.NewEncoder(w).Encode(response)
	}
}

// ReadyHandler для readiness probes (можно расширить проверкой зависимостей)
func ReadyHandler(serviceName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Здесь можно добавить проверку соединения с БД, Redis и т.д.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := HealthResponse{
			Status:    "ready",
			Timestamp: time.Now().UTC(),
			Service:   serviceName,
		}

		json.NewEncoder(w).Encode(response)
	}
}
