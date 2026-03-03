// Atomic Package Manager
// Copyright (C) 2025 Дмитрий Удалов dmitry@udalov.online
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package http_server

import (
	"apm/internal/common/apmerr"
	"apm/internal/common/app"
	"apm/internal/common/reply"
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config конфигурация HTTP сервера
type Config struct {
	ListenAddr   string
	APIToken     string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// DefaultConfig возвращает конфигурацию по умолчанию
func DefaultConfig() Config {
	return Config{
		ListenAddr:   "127.0.0.1:8080",
		ReadTimeout:  3 * time.Minute,
		WriteTimeout: 30 * time.Minute,
	}
}

// Server HTTP сервер APM
type Server struct {
	config      Config
	appConfig   *app.Config
	mux         *http.ServeMux
	server      *http.Server
	listener    net.Listener
	registry    *Registry
	parsedToken tokenInfo
}

// tokenInfo информация о токене
type tokenInfo struct {
	permission string
	token      string
}

// NewServer создаёт новый HTTP сервер
func NewServer(config Config, appConfig *app.Config) (*Server, error) {
	s := &Server{
		config:    config,
		appConfig: appConfig,
		mux:       http.NewServeMux(),
	}
	if config.APIToken != "" {
		parsed, err := parseToken(config.APIToken)
		if err != nil {
			return nil, err
		}
		s.parsedToken = parsed
	}
	return s, nil
}

// parseToken парсит токен в формате "permission:token".
// Возвращает ошибку если формат неверный или permission неизвестный.
func parseToken(tokenStr string) (tokenInfo, error) {
	parts := strings.SplitN(tokenStr, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return tokenInfo{}, fmt.Errorf(app.T_("Invalid token format: expected '<permission>:<token>', got '%s'\n  permission must be '%s' (read-only) or '%s' (full access)\n  example: --api-token %s:my-secret-token"), tokenStr, PermRead, PermManage, PermRead)
	}

	perm := parts[0]
	if perm != PermRead && perm != PermManage {
		return tokenInfo{}, fmt.Errorf(app.T_("Unknown permission '%s': must be '%s' (read-only) or '%s' (full access)\n  example: --api-token %s:%s"), perm, PermRead, PermManage, PermRead, "my-secret-token")
	}

	if len(parts[1]) < minTokenLength {
		return tokenInfo{}, fmt.Errorf(app.T_("Token is too short: minimum %d characters required"), minTokenLength)
	}

	return tokenInfo{
		permission: perm,
		token:      parts[1],
	}, nil
}

// checkPermission проверяет, достаточно ли прав у токена
func checkPermission(tokenPerm string, requiredPerm string) bool {
	if requiredPerm == "" || tokenPerm == PermManage {
		return true
	}
	return tokenPerm == requiredPerm
}

// withAuth оборачивает handler в per-handler аутентификацию и проверку прав
func (s *Server) withAuth(perm string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.parsedToken.token == "" {
			handler(w, r)
			return
		}

		tokenStr := ""
		if authHeader := r.Header.Get("Authorization"); authHeader != "" {
			tokenStr = authHeader
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
			}
		} else if t := r.URL.Query().Get("token"); t != "" {
			tokenStr = t
		}

		if tokenStr == "" {
			writeUnauthorized(w, app.T_("Authorization header or token query parameter is required"))
			return
		}

		if subtle.ConstantTimeCompare([]byte(tokenStr), []byte(s.parsedToken.token)) != 1 {
			writeUnauthorized(w, app.T_("Invalid API token"))
			return
		}

		if !checkPermission(s.parsedToken.permission, perm) {
			writeForbidden(w, fmt.Sprintf(app.T_("Insufficient permissions. Required: %s, provided: %s"), perm, s.parsedToken.permission))
			return
		}

		handler(w, r)
	}
}

// RegisterEndpoints регистрирует endpoints: оборачивает handler в withAuth, добавляет в mux и registry
func (s *Server) RegisterEndpoints(endpoints []Endpoint) {
	if s.registry == nil {
		s.registry = NewRegistry()
	}

	for _, ep := range endpoints {
		handler := ep.Handler
		if ep.Permission != "" {
			handler = s.withAuth(ep.Permission, handler)
		}
		s.mux.HandleFunc(ep.HTTPMethod+" "+ep.HTTPPath, handler)
	}

	s.registry.RegisterEndpoints(endpoints)
}

// GetRegistry возвращает registry для OpenAPI генератора
func (s *Server) GetRegistry() *Registry {
	if s.registry == nil {
		s.registry = NewRegistry()
	}
	return s.registry
}

// loggingMiddleware логирует запросы
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		elapsed := time.Since(start)
		app.Log.Info(fmt.Sprintf("HTTP %s %s %d %.3fms", r.Method, r.URL.Path, wrapped.statusCode, float64(elapsed.Microseconds())/1000.0))
	})
}

// isAllowedOrigin проверяет, что Origin является локальным или отсутствует.
func isAllowedOrigin(origin string) bool {
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := u.Hostname()
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

// corsMiddleware добавляет CORS заголовки
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Transaction-ID")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}
		} else if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// responseWriter обёртка для захвата статус-кода
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Hijack реализует интерфейс http.Hijacker для поддержки WebSocket
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("responseWriter does not implement http.Hijacker")
	}
	return h.Hijack()
}

const minTokenLength = 6
const maxRequestBodySize = 20 << 20

// bodySizeLimitMiddleware ограничивает размер тела запроса
func (s *Server) bodySizeLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
		next.ServeHTTP(w, r)
	})
}

// Start запускает HTTP сервер
func (s *Server) Start(ctx context.Context) error {
	handler := s.corsMiddleware(s.loggingMiddleware(s.bodySizeLimitMiddleware(s.mux)))
	s.server = &http.Server{
		Handler:      handler,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	var err error

	s.listener, err = net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.config.ListenAddr, err)
	}

	app.Log.Info("HTTP server listening on http://" + s.config.ListenAddr)

	go func() {
		if err = s.server.Serve(s.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			app.Log.Errorf("HTTP server error: %v", err)
		}
	}()

	<-ctx.Done()

	return s.Shutdown()
}

// Shutdown останавливает HTTP сервер
func (s *Server) Shutdown() error {
	if s.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	app.Log.Info("Shutting down HTTP server...")

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	return nil
}

// writeUnauthorized отправляет ошибку авторизации
func writeUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(reply.ErrorResponseFromError(apmerr.New(apmerr.ErrorTypePermission, errors.New(message))))
}

// writeForbidden отправляет ошибку доступа
func writeForbidden(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(reply.ErrorResponseFromError(apmerr.New(apmerr.ErrorTypePermission, errors.New(message))))
}

// RegisterHealthCheck регистрирует эндпоинт проверки здоровья
func (s *Server) RegisterHealthCheck() {
	s.mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"version": s.appConfig.ConfigManager.GetConfig().Version,
		})
	})
}

// RegisterWebSocket регистрирует WebSocket эндпоинт для событий
func (s *Server) RegisterWebSocket() {
	hub := GetWebSocketHub()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleWebSocket(w, r)
	})
	if s.parsedToken.token != "" {
		s.mux.HandleFunc("GET /api/v1/events", s.withAuth(PermRead, handler))
	} else {
		s.mux.HandleFunc("GET /api/v1/events", handler)
	}
	app.Log.Info("WebSocket events endpoint: ws://" + s.config.ListenAddr + "/api/v1/events")
}

// RegisterAPIInfo регистрирует эндпоинт информации об API
func (s *Server) RegisterAPIInfo(isAtomic bool, hasDistrobox bool, hasKernel bool) {
	s.mux.HandleFunc("GET /api/v1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		modules := []string{"system", "repo"}
		if hasDistrobox {
			modules = append(modules, "distrobox")
		}
		if hasKernel {
			modules = append(modules, "kernel")
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"name":       "APM HTTP API",
			"version":    s.appConfig.ConfigManager.GetConfig().Version,
			"apiVersion": "v1",
			"isAtomic":   isAtomic,
			"modules":    modules,
			"docs":       "/api/v1/docs",
			"openapi":    "/api/v1/openapi.json",
		})
	})

	s.mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/api/v1/docs", http.StatusFound)
	})
}

// OpenAPIFromRegistry интерфейс для генератора OpenAPI из registry
type OpenAPIFromRegistry interface {
	GenerateOpenAPI() map[string]interface{}
}

// RegisterOpenAPIFromRegistry регистрирует OpenAPI из registry
func (s *Server) RegisterOpenAPIFromRegistry(gen OpenAPIFromRegistry) {
	spec := gen.GenerateOpenAPI()

	// Добавляем метаданные версии
	if info, ok := spec["info"].(map[string]interface{}); ok {
		info["version"] = s.appConfig.ConfigManager.GetConfig().Version
	}

	specJSON, _ := json.Marshal(spec)

	s.mux.HandleFunc("GET /api/v1/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		html := `<!DOCTYPE html>
<html>
<head>
    <title>APM API Documentation</title>
    <meta charset="utf-8"/>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5.32.0/swagger-ui.css">
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.32.0/swagger-ui-bundle.js"></script>
    <script>
        window.onload = function() {
            SwaggerUIBundle({
                spec: ` + string(specJSON) + `,
                dom_id: '#swagger-ui',
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIBundle.SwaggerUIStandalonePreset
                ],
                layout: "BaseLayout"
            });
        };
    </script>
</body>
</html>`
		_, _ = w.Write([]byte(html))
	})

	s.mux.HandleFunc("GET /api/v1/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(spec)
	})
}
