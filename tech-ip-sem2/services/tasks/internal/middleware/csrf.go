package middleware

import (
	"net/http"

	"github.com/sun1tar/MIREA-TIP-Practice-32/tech-ip-sem2/shared/logger"
)

// CSRFMiddleware проверяет CSRF-токен для state-changing методов
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Только для POST, PATCH, DELETE
		if r.Method == http.MethodPost || r.Method == http.MethodPatch || r.Method == http.MethodDelete {

			// Логируем все заголовки для диагностики
			logger.Logger.WithFields(map[string]interface{}{
				"method":              r.Method,
				"path":                r.URL.Path,
				"headers":             r.Header,
				"content_type":        r.Header.Get("Content-Type"),
				"auth_header":         r.Header.Get("Authorization"),
				"x_csrf_token_header": r.Header.Get("X-CSRF-Token"),
				"all_cookies":         r.Header.Get("Cookie"),
			}).Info("CSRF middleware - request details")

			// Получаем CSRF токен из cookie
			csrfCookie, err := r.Cookie("csrf_token")
			if err != nil {
				logger.Logger.WithFields(map[string]interface{}{
					"error":  err.Error(),
					"method": r.Method,
					"path":   r.URL.Path,
				}).Warn("CSRF token missing in cookies")
				http.Error(w, `{"error":"CSRF token missing in cookies"}`, http.StatusForbidden)
				return
			}

			// Получаем CSRF токен из заголовка
			csrfHeader := r.Header.Get("X-CSRF-Token")
			if csrfHeader == "" {
				logger.Logger.WithFields(map[string]interface{}{
					"method":       r.Method,
					"path":         r.URL.Path,
					"cookie_value": csrfCookie.Value,
				}).Warn("X-CSRF-Token header missing")
				http.Error(w, `{"error":"X-CSRF-Token header missing"}`, http.StatusForbidden)
				return
			}

			// Сравниваем
			if csrfCookie.Value != csrfHeader {
				logger.Logger.WithFields(map[string]interface{}{
					"method":       r.Method,
					"path":         r.URL.Path,
					"cookie_value": csrfCookie.Value,
					"header_value": csrfHeader,
				}).Warn("CSRF token mismatch")
				http.Error(w, `{"error":"CSRF token mismatch"}`, http.StatusForbidden)
				return
			}

			// Если всё хорошо, логируем успех
			logger.Logger.WithFields(map[string]interface{}{
				"method": r.Method,
				"path":   r.URL.Path,
			}).Info("CSRF check passed")
		}

		// Если проверка пройдена или метод безопасный, передаём дальше
		next.ServeHTTP(w, r)
	})
}
