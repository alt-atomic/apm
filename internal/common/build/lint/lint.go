package lint

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const confName = "apm-lint.conf"

// Service предоставляет API для линтинга образов
type Service struct {
	rootfs string
}

// Result содержит результат линтинга
type Result struct {
	TmpFiles *TmpFilesResult
	SysUsers *SysUsersResult
	RunTmp   *RunTmpResult
	Written  []string
	Message  string
}

// TmpFilesResult результат анализа tmpfiles.d
type TmpFilesResult struct {
	Missing     []string
	Unsupported []string
}

// SysUsersResult результат анализа sysusers.d
type SysUsersResult struct {
	Missing []string
}

// RunTmpResult результат анализа /run и /tmp
type RunTmpResult struct {
	Entries []string
}

// New создаёт новый Service для линтинга rootfs
func New(rootfs string) *Service {
	return &Service{rootfs: rootfs}
}

// AnalyzeTmpFiles анализирует tmpfiles.d покрытие. При fix=true удаляет старый и записывает новый конфиг.
func (s *Service) AnalyzeTmpFiles(ctx context.Context, fix bool) (*TmpFilesResult, string, error) {
	var a tmpFilesAnalysis

	if fix {
		if _, err := a.RemoveConf(s.rootfs); err != nil {
			return nil, "", err
		}
	}

	if err := a.Analyze(ctx, s.rootfs); err != nil {
		return nil, "", err
	}

	if len(a.Missing) == 0 && len(a.Unsupported) == 0 {
		return nil, "", nil
	}

	result := &TmpFilesResult{Unsupported: a.Unsupported}
	for _, e := range a.Missing {
		result.Missing = append(result.Missing, e.Line)
	}

	var written string
	if fix {
		path, err := a.WriteConf(s.rootfs)
		if err != nil {
			return nil, "", fmt.Errorf("writing tmpfiles.d: %w", err)
		}
		written = path
	}

	return result, written, nil
}

// AnalyzeSysUsers анализирует sysusers.d покрытие. При fix=true удаляет старый и записывает новый конфиг.
func (s *Service) AnalyzeSysUsers(ctx context.Context, fix bool) (*SysUsersResult, string, error) {
	var a sysusersAnalysis

	if fix {
		if _, err := a.RemoveConf(s.rootfs); err != nil {
			return nil, "", err
		}
	}

	if err := a.Analyze(ctx, s.rootfs); err != nil {
		return nil, "", err
	}

	lines := a.MissingLines()
	if len(lines) == 0 {
		return nil, "", nil
	}

	var written string
	if fix {
		path, err := a.WriteConf(s.rootfs)
		if err != nil {
			return nil, "", fmt.Errorf("writing sysusers.d: %w", err)
		}
		written = path
	}

	return &SysUsersResult{Missing: lines}, written, nil
}

// AnalyzeRunTmp анализирует /run и /tmp на наличие неожиданных файлов.
func (s *Service) AnalyzeRunTmp(ctx context.Context) (*RunTmpResult, error) {
	var a runTmpAnalysis

	if err := a.Analyze(ctx, s.rootfs); err != nil {
		return nil, err
	}

	if len(a.Unexpected) == 0 {
		return nil, nil
	}

	return &RunTmpResult{Entries: a.Unexpected}, nil
}

// Analyze выполняет полный анализ образа. При fix=true удаляет старые конфиги и записывает новые.
func (s *Service) Analyze(ctx context.Context, fix bool) (*Result, error) {
	var (
		resTmp *TmpFilesResult
		resSys *SysUsersResult
		resRun *RunTmpResult
		wTmp   string
		wSys   string
		errTmp error
		errSys error
		errRun error
		wg     sync.WaitGroup
	)

	wg.Add(3)
	go func() { defer wg.Done(); resTmp, wTmp, errTmp = s.AnalyzeTmpFiles(ctx, fix) }()
	go func() { defer wg.Done(); resSys, wSys, errSys = s.AnalyzeSysUsers(ctx, fix) }()
	go func() { defer wg.Done(); resRun, errRun = s.AnalyzeRunTmp(ctx) }()
	wg.Wait()

	if err := errors.Join(errTmp, errSys, errRun); err != nil {
		return nil, err
	}

	result := &Result{
		TmpFiles: resTmp,
		SysUsers: resSys,
		RunTmp:   resRun,
	}

	for _, w := range []string{wTmp, wSys} {
		if w != "" {
			result.Written = append(result.Written, w)
		}
	}

	if fix {
		if len(result.Written) > 0 {
			result.Message = fmt.Sprintf("Fixed: written %s", strings.Join(result.Written, ", "))
		} else {
			result.Message = "Nothing to fix"
		}
	} else {
		if result.TmpFiles != nil || result.SysUsers != nil || result.RunTmp != nil {
			result.Message = "Lint issues found"
		} else {
			result.Message = "No issues found"
		}
	}

	return result, nil
}

// removeConf удаляет conf-файл
func removeConf(rootfs, subDir string) (string, error) {
	path := filepath.Join(rootfs, "usr", "lib", subDir, confName)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return path, err
	}
	return path, nil
}

// writeConf записывает содержимое conf-файла
func writeConf(rootfs, subDir, content string) (string, error) {
	dir := filepath.Join(rootfs, "usr", "lib", subDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating %s: %w", dir, err)
	}
	path := filepath.Join(dir, confName)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("writing %s: %w", path, err)
	}
	return path, nil
}
