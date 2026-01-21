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
	"apm/internal/common/app"
	"apm/internal/common/reply"
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
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
	config    Config
	appConfig *app.Config
	mux       *http.ServeMux
	server    *http.Server
	listener  net.Listener
	registry  *Registry
}

// tokenInfo информация о токене
type tokenInfo struct {
	permission string // read или manage
	token      string // сам токен
}

// NewServer создаёт новый HTTP сервер
func NewServer(config Config, appConfig *app.Config) *Server {
	return &Server{
		config:    config,
		appConfig: appConfig,
		mux:       http.NewServeMux(),
	}
}

// GetMux возвращает маршрутизатор для регистрации обработчиков
func (s *Server) GetMux() *http.ServeMux {
	return s.mux
}

// SetRegistry устанавливает registry для проверки прав
func (s *Server) SetRegistry(registry *Registry) {
	s.registry = registry
}

// parseToken парсит токен в формате "permission:token" или просто "token"
func parseToken(tokenStr string) tokenInfo {
	parts := strings.SplitN(tokenStr, ":", 2)
	if len(parts) == 2 {
		return tokenInfo{
			permission: parts[0],
			token:      parts[1],
		}
	}
	return tokenInfo{
		permission: "manage",
		token:      tokenStr,
	}
}

// findEndpointPermission находит требуемое разрешение для endpoint
func (s *Server) findEndpointPermission(path string, method string) string {
	if s.registry == nil {
		return ""
	}

	for _, ep := range s.registry.GetHTTPEndpoints() {
		if ep.HTTPPath == path && ep.HTTPMethod == method {
			return ep.Permission
		}
	}
	return ""
}

// checkPermission проверяет, достаточно ли прав у токена
func checkPermission(tokenPerm string, requiredPerm string) bool {
	// manage имеет доступ ко всему
	if tokenPerm == "manage" {
		return true
	}
	// read имеет доступ только к read
	if tokenPerm == "read" && requiredPerm == "read" {
		return true
	}
	// read не имеет доступа к manage
	if tokenPerm == "read" && requiredPerm == "manage" {
		return false
	}
	// Если разрешение не указано, пропускаем
	if requiredPerm == "" {
		return true
	}
	return false
}

// authMiddleware проверяет авторизацию
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	publicPaths := []string{
		"/",
		"/api/v1",
		"/api/v1/health",
		"/api/v1/docs",
		"/api/v1/openapi.json",
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем, является ли путь публичным
		for _, path := range publicPaths {
			if r.URL.Path == path {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Если токен не настроен, пропускаем все запросы
		if s.config.APIToken == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Получаем токен из заголовка
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeUnauthorized(w, "Authorization header is required")
			return
		}

		// Поддерживаем формат "Bearer <token>" и просто "<token>"
		tokenStr := authHeader
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		}

		// Парсим токен (может быть в формате "permission:token")
		parsedToken := parseToken(tokenStr)

		// Парсим конфигурационный токен
		configToken := parseToken(s.config.APIToken)

		// Проверяем, что сам токен совпадает
		if parsedToken.token != configToken.token {
			writeUnauthorized(w, "Invalid API token")
			return
		}

		// Находим требуемое разрешение для endpoint
		requiredPerm := s.findEndpointPermission(r.URL.Path, r.Method)

		// Проверяем права доступа
		if !checkPermission(parsedToken.permission, requiredPerm) {
			writeForbidden(w, "Insufficient permissions. Required: "+requiredPerm+", provided: "+parsedToken.permission)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware логирует запросы
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Обёртка для захвата статус-кода
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		elapsed := time.Since(start)
		app.Log.Info(fmt.Sprintf("HTTP %s %s %d %.3fms", r.Method, r.URL.Path, wrapped.statusCode, float64(elapsed.Microseconds())/1000.0))
	})
}

// corsMiddleware добавляет CORS заголовки
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Transaction-ID")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
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

// Start запускает HTTP сервер
func (s *Server) Start(ctx context.Context) error {
	// Применяем middleware
	handler := s.corsMiddleware(s.loggingMiddleware(s.authMiddleware(s.mux)))
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
		if err = s.server.Serve(s.listener); err != nil && !errors.Is(http.ErrServerClosed, err) {
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
	_ = json.NewEncoder(w).Encode(reply.APIResponse{
		Data:  map[string]interface{}{"message": message},
		Error: true,
	})
}

// writeForbidden отправляет ошибку доступа
func writeForbidden(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(reply.APIResponse{
		Data:  map[string]interface{}{"message": message},
		Error: true,
	})
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
	s.mux.HandleFunc("GET /api/v1/events", func(w http.ResponseWriter, r *http.Request) {
		hub.HandleWebSocket(w, r)
	})
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

	// Редирект с корня на API info
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
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
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
