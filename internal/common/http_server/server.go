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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// Config конфигурация HTTP сервера
type Config struct {
	ListenAddr   string
	UnixSocket   string
	APIToken     string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// DefaultConfig возвращает конфигурацию по умолчанию
func DefaultConfig() Config {
	return Config{
		ListenAddr:   "127.0.0.1:8080",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}
}

// Server HTTP сервер APM
type Server struct {
	config    Config
	appConfig *app.Config
	mux       *http.ServeMux
	server    *http.Server
	listener  net.Listener
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

// authMiddleware проверяет авторизацию
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		token := authHeader
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}

		if token != s.config.APIToken {
			writeUnauthorized(w, "Invalid API token")
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

	// Выбираем способ прослушивания: Unix socket или TCP
	if s.config.UnixSocket != "" {
		if err = os.Remove(s.config.UnixSocket); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove old socket: %w", err)
		}

		s.listener, err = net.Listen("unix", s.config.UnixSocket)
		if err != nil {
			return fmt.Errorf("failed to listen on unix socket %s: %w", s.config.UnixSocket, err)
		}

		// Устанавливаем права доступа на сокет
		if err = os.Chmod(s.config.UnixSocket, 0660); err != nil {
			return fmt.Errorf("failed to chmod socket: %w", err)
		}

		app.Log.Info("HTTP server listening on unix://" + s.config.UnixSocket)
	} else {
		s.listener, err = net.Listen("tcp", s.config.ListenAddr)
		if err != nil {
			return fmt.Errorf("failed to listen on %s: %w", s.config.ListenAddr, err)
		}

		app.Log.Info("HTTP server listening on http://" + s.config.ListenAddr)
	}

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

	if s.config.UnixSocket != "" {
		_ = os.Remove(s.config.UnixSocket)
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
