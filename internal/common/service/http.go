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

package service

import (
	"apm/internal/common/app"
	apmcli "apm/internal/common/cli"
	"apm/internal/common/http_server"
	"apm/internal/common/reply"
	"context"
	"fmt"
	"sync"

	"github.com/urfave/cli/v3"
)

type APIInfo struct {
	IsAtomic     bool
	HasDistrobox bool
	HasKernel    bool
}

type HTTPModule struct {
	Endpoints func(ctx context.Context) []http_server.Endpoint
	PostInit  func(context.Context)
}

type HTTPRunConfig struct {
	Mode    apmcli.RootCheckMode
	APIInfo APIInfo
	Modules []HTTPModule
}

func RunHTTP(
	ctx context.Context,
	cmd *cli.Command,
	appConfig *app.Config,
	cfg HTTPRunConfig,
) error {
	appConfig.ConfigManager.SetFormat(app.FormatHTTP)
	appConfig.ConfigManager.EnableVerbose()

	if err := apmcli.CheckRoot(cfg.Mode); err != nil {
		return err
	}

	httpCfg := http_server.DefaultConfig()
	if listen := cmd.String("listen"); listen != "" {
		httpCfg.ListenAddr = listen
	}
	if token := cmd.String("api-token"); token != "" {
		httpCfg.APIToken = token
	}

	server, err := http_server.NewServer(httpCfg, appConfig)
	if err != nil {
		return fmt.Errorf("create http server: %w", err)
	}

	reply.SetWebSocketHub(http_server.GetWebSocketHub())

	server.RegisterHealthCheck()
	server.RegisterWebSocket()
	server.RegisterAPIInfo(cfg.APIInfo.IsAtomic, cfg.APIInfo.HasDistrobox, cfg.APIInfo.HasKernel)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for _, mod := range cfg.Modules {
		server.RegisterEndpoints(mod.Endpoints(runCtx))
		if mod.PostInit != nil {
			wg.Add(1)
			go func(h func(context.Context)) {
				defer wg.Done()
				h(runCtx)
			}(mod.PostInit)
		}
	}

	server.RegisterOpenAPIFromRegistry(http_server.NewOpenAPIGenerator(
		server.GetRegistry(),
		appConfig.ConfigManager.GetConfig().Version,
		httpCfg.ListenAddr,
	))

	err = server.Start(ctx)
	cancel()
	wg.Wait()
	if err != nil {
		return fmt.Errorf("start http server: %w", err)
	}
	return nil
}
