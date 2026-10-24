package system

import (
	"context"
	"errors"
	"testing"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
	"altlinux.space/alt-atomic/apm/internal/common/imagesvc"
	"altlinux.space/alt-atomic/apm/internal/common/testutil"
	pkgbuild "altlinux.space/alt-atomic/apm/pkg/build"
)

func TestImageHistory(t *testing.T) {
	history := []imagesvc.ImageHistory{
		{ImageName: "alt:p11"},
		{ImageName: "alt:p11"},
	}

	t.Run("returns history with total count", func(t *testing.T) {
		hostDB := &mockHostDB{historyResult: history, countResult: 10}
		actions := newTestActions(nil, &mockAptDB{}, hostDB)

		resp, err := actions.ImageHistory(context.Background(), "", 10, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.History) != 2 {
			t.Errorf("expected 2 entries, got %d", len(resp.History))
		}
		if resp.TotalCount != 10 {
			t.Errorf("expected totalCount=10, got %d", resp.TotalCount)
		}
	})

	t.Run("history query error propagates", func(t *testing.T) {
		hostDB := &mockHostDB{historyErr: errors.New("db error")}
		actions := newTestActions(nil, &mockAptDB{}, hostDB)

		_, err := actions.ImageHistory(context.Background(), "", 10, 0)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeDatabase)
	})

	t.Run("count query error propagates", func(t *testing.T) {
		hostDB := &mockHostDB{historyResult: history, countErr: errors.New("count error")}
		actions := newTestActions(nil, &mockAptDB{}, hostDB)

		_, err := actions.ImageHistory(context.Background(), "", 10, 0)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeDatabase)
	})
}

func TestImageGetConfig(t *testing.T) {
	t.Run("returns loaded config", func(t *testing.T) {
		cfg := &imagesvc.Config{Image: "alt:p11"}
		actions := newTestActions(nil, &mockAptDB{}, nil)
		actions.serviceHostConfig = &mockHostConfig{config: cfg}

		resp, err := actions.ImageGetConfig(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Config.Image != "alt:p11" {
			t.Errorf("expected image alt:p11, got %s", resp.Config.Image)
		}
	})

	t.Run("load error propagates as image error", func(t *testing.T) {
		actions := newTestActions(nil, &mockAptDB{}, nil)
		actions.serviceHostConfig = &mockHostConfig{loadErr: errors.New("file not found")}

		_, err := actions.ImageGetConfig(context.Background())
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeImage)
	})
}

func TestImageSaveConfig(t *testing.T) {
	t.Run("replaces config and saves", func(t *testing.T) {
		hcfg := &mockHostConfig{config: &imagesvc.Config{Image: "old"}}
		actions := newTestActions(nil, &mockAptDB{}, nil)
		actions.serviceHostConfig = hcfg

		newCfg := imagesvc.Config{Image: "alt:p12"}
		resp, err := actions.ImageSaveConfig(context.Background(), newCfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Config.Image != "alt:p12" {
			t.Errorf("expected saved image alt:p12, got %s", resp.Config.Image)
		}
		if hcfg.config.Image != "alt:p12" {
			t.Error("SetConfig should update the stored config")
		}
	})

	t.Run("save error propagates as image error", func(t *testing.T) {
		actions := newTestActions(nil, &mockAptDB{}, nil)
		actions.serviceHostConfig = &mockHostConfig{
			config:  &imagesvc.Config{},
			saveErr: errors.New("disk full"),
		}

		_, err := actions.ImageSaveConfig(context.Background(), imagesvc.Config{Image: "new"})
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeImage)
	})
}

func TestImageSwitch(t *testing.T) {
	t.Run("empty image returns validation error", func(t *testing.T) {
		actions := newTestActions(nil, &mockAptDB{}, nil)

		_, err := actions.ImageSwitch(context.Background(), "  ", false, true)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeValidation)
	})

	t.Run("same image still active returns validation error", func(t *testing.T) {
		himg := &mockHostImage{isActive: true}
		actions := newTestActions(nil, &mockAptDB{}, nil)
		actions.serviceHostImage = himg
		actions.serviceHostConfig = &mockHostConfig{config: &imagesvc.Config{Image: "alt:p11"}}

		_, err := actions.ImageSwitch(context.Background(), "alt:p11", false, true)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeValidation)
		if himg.switchCalls != 0 || himg.buildCalls != 0 {
			t.Error("switch should not run when image is already active")
		}
	})

	t.Run("same image after failed switch retries", func(t *testing.T) {
		himg := &mockHostImage{isActive: false}
		hcfg := &mockHostConfig{config: &imagesvc.Config{Image: "alt:p11"}}
		actions := newTestActions(nil, &mockAptDB{}, nil)
		actions.serviceHostImage = himg
		actions.serviceHostConfig = hcfg

		_, err := actions.ImageSwitch(context.Background(), "alt:p11", false, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if himg.switchCalls != 1 {
			t.Error("expected switch to run again")
		}
		if hcfg.saveCalls != 1 {
			t.Error("expected config to be saved after switch")
		}
	})

	t.Run("switch failure does not save config", func(t *testing.T) {
		himg := &mockHostImage{switchErr: errors.New("blob unknown")}
		hcfg := &mockHostConfig{config: &imagesvc.Config{Image: "alt:p11"}}
		actions := newTestActions(nil, &mockAptDB{}, nil)
		actions.serviceHostImage = himg
		actions.serviceHostConfig = hcfg

		_, err := actions.ImageSwitch(context.Background(), "alt:p12", false, true)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeImage)
		if hcfg.saveCalls != 0 {
			t.Error("config should not be saved after failed switch")
		}
	})

	t.Run("modules trigger rebuild and save after success", func(t *testing.T) {
		himg := &mockHostImage{}
		hcfg := &mockHostConfig{config: &imagesvc.Config{
			Image:   "alt:p11",
			Modules: []pkgbuild.Module{{Name: "pkgs"}},
		}}
		actions := newTestActions(nil, &mockAptDB{}, nil)
		actions.serviceHostImage = himg
		actions.serviceHostConfig = hcfg

		_, err := actions.ImageSwitch(context.Background(), "alt:p12", false, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if himg.buildCalls != 1 || himg.switchCalls != 0 {
			t.Error("expected local rebuild instead of direct switch")
		}
		if hcfg.config.Image != "alt:p12" {
			t.Errorf("expected config image alt:p12, got %s", hcfg.config.Image)
		}
		if hcfg.saveCalls != 1 {
			t.Error("expected config to be saved after switch")
		}
	})
}
