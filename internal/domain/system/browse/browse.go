package browse

import (
	"apm/internal/common/app"
	"apm/internal/common/reply"
	"context"
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func Run(ctx context.Context, appConfig *app.Config, dbService aptDatabaseService) error {
	if !reply.IsInteractive(appConfig) {
		return errors.New(app.T_("browse command requires an interactive terminal"))
	}

	reply.StopSpinner(appConfig)

	pkgs, err := dbService.QueryHostImagePackages(ctx, nil, "name", "ASC", 0, 0)
	if err != nil {
		return fmt.Errorf(app.T_("Failed to load packages: %w"), err)
	}

	total, _ := dbService.CountHostImagePackages(ctx, nil)

	m := newModel(ctx, appConfig, dbService, pkgs, total)

	p := tea.NewProgram(&m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err = p.Run(); err != nil {
		return fmt.Errorf(app.T_("TUI error: %w"), err)
	}

	return nil
}
