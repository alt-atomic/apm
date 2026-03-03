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
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ServeHTML запускает HTTP сервер, отдающий HTML на указанном адресе.
func ServeHTML(ctx context.Context, addr string, htmlGenerator func() string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := fmt.Fprint(w, htmlGenerator())
		if err != nil {
			if !strings.Contains(err.Error(), "broken pipe") {
				app.Log.Error("HTTP write error: " + err.Error())
			}
			return
		}
	})

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	fmt.Printf("Documentation server started at http://%s\n", addr)
	fmt.Println("Press Ctrl+C to stop")

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			app.Log.Fatal(err.Error())
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}
