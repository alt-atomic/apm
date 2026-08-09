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

package system

import (
	"context"
	"slices"
	"strings"
	"syscall"
	"testing"

	"altlinux.space/alt-atomic/apm/internal/domain/system"
	"altlinux.space/alt-atomic/apm/tests/integration/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type ProviderSelectionTestSuite struct {
	suite.Suite
	actions *system.Actions
	ctx     context.Context
}

func (s *ProviderSelectionTestSuite) SetupSuite() {
	if syscall.Geteuid() != 0 {
		s.T().Skip("This test suite requires root privileges. Run with sudo.")
	}

	appConfig, reporter, ctx := common.GetTestAppConfig(s.T())
	s.actions = system.NewActions(appConfig, reporter)
	s.ctx = ctx
}

// simulateNewInstalls возвращает набор новых пакетов из симуляции установки.
func (s *ProviderSelectionTestSuite) simulateNewInstalls(pkgs []string) (map[string]struct{}, string) {
	resp, err := s.actions.CheckInstall(s.ctx, pkgs)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "not found") ||
			strings.Contains(msg, "no installation candidate") ||
			strings.Contains(msg, "Package database is empty") {
			return nil, msg
		}
		s.T().Fatalf("unexpected CheckInstall error for %v: %v", pkgs, msg)
	}

	set := make(map[string]struct{}, len(resp.Info.NewInstalledPackages))
	for _, p := range resp.Info.NewInstalledPackages {
		set[p] = struct{}{}
	}
	return set, ""
}

// TestExplicitProviderWinsRegardlessOfOrder: gdm тянет виртуальный
// x-terminal-emulator, при явном ptyxis в списке xterm не должен попадать в план установки
// вне зависимости от позиции при установке.
func (s *ProviderSelectionTestSuite) TestExplicitProviderWinsRegardlessOfOrder() {
	consumerFirst, skip := s.simulateNewInstalls([]string{"gdm", "ptyxis"})
	if skip != "" {
		s.T().Skipf("packages unavailable in test env: %s", skip)
	}
	providerFirst, skip := s.simulateNewInstalls([]string{"ptyxis", "gdm"})
	if skip != "" {
		s.T().Skipf("packages unavailable in test env: %s", skip)
	}

	_, xtermConsumerFirst := consumerFirst["xterm"]
	_, xtermProviderFirst := providerFirst["xterm"]
	assert.False(s.T(), xtermConsumerFirst,
		"xterm must not be pulled when ptyxis is requested (consumer-first order)")
	assert.False(s.T(), xtermProviderFirst,
		"xterm must not be pulled when ptyxis is requested (provider-first order)")

	// План не зависит от порядка аргументов.
	consumerList := setToSortedSlice(consumerFirst)
	providerList := setToSortedSlice(providerFirst)
	assert.True(s.T(), slices.Equal(consumerList, providerList),
		"install plan must be identical regardless of package order")
}

func setToSortedSlice(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func TestProviderSelectionSuite(t *testing.T) {
	suite.Run(t, new(ProviderSelectionTestSuite))
}
