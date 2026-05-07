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

package build

import (
	"apm/internal/common/app"
	"apm/internal/common/command"
	"apm/internal/common/reply"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// PodmanService инкапсулирует операции с podman/bootc.
type PodmanService struct {
	runner   command.Runner
	reporter *reply.Reporter
}

// NewPodmanService создаёт сервис для работы с podman/bootc.
func NewPodmanService(runner command.Runner, reporter *reply.Reporter) *PodmanService {
	return &PodmanService{runner: runner, reporter: reporter}
}

// blobProgress хранит состояние загрузки одного blob'а
type blobProgress struct {
	downloaded float64
	total      float64
}

// progressTracker отслеживает общий прогресс загрузки всех blob'ов
type progressTracker struct {
	blobs       map[string]*blobProgress
	mu          sync.Mutex
	lastPercent int
}

func newProgressTracker() *progressTracker {
	return &progressTracker{
		blobs:       make(map[string]*blobProgress),
		lastPercent: -1,
	}
}

func (pt *progressTracker) update(blobKey string, downloaded, total float64) (totalPercent int, changed bool) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if _, exists := pt.blobs[blobKey]; !exists {
		pt.blobs[blobKey] = &blobProgress{}
	}
	pt.blobs[blobKey].downloaded = downloaded
	pt.blobs[blobKey].total = total

	var sumDownloaded, sumTotal float64
	for _, bp := range pt.blobs {
		sumDownloaded += bp.downloaded
		sumTotal += bp.total
	}

	if sumTotal == 0 {
		return 0, false
	}

	percent := int((sumDownloaded / sumTotal) * 100)
	if percent > 100 {
		percent = 100
	}

	changed = percent != pt.lastPercent
	pt.lastPercent = percent

	return percent, changed
}

// Pull запускает podman build/pull с pty-прогрессом.
func (p *PodmanService) Pull(ctx context.Context, args []string) (string, error) {
	tracker := newProgressTracker()

	output, _, err := p.runner.Run(ctx, args,
		command.WithPTY(40, 120),
		command.WithEnv("TERM=xterm-256color", "TMPDIR=/var/tmp", "LC_ALL=C"),
		command.WithStreamHandler(func(r io.Reader) {
			scanner := bufio.NewScanner(r)
			scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
			for scanner.Scan() {
				p.parseProgressLine(ctx, scanner.Text(), tracker)
			}
			if scanErr := scanner.Err(); scanErr != nil && scanErr != io.EOF {
				app.Log.Debugf("Pull scanner error: %v", scanErr)
			}
		}),
	)
	if err != nil {
		return output, fmt.Errorf(app.T_("Command failed with error: %v"), err)
	}

	p.reporter.CreateEventNotification(ctx, reply.StateAfter,
		reply.WithEventName(reply.EventSystemPullImage),
		reply.WithProgress(true),
		reply.WithProgressPercent(100),
	)

	return output, nil
}

// parseProgressLine разбирает строки вывода podman и обновляет общий прогресс.
func (p *PodmanService) parseProgressLine(ctx context.Context, rawLine string, tracker *progressTracker) {
	line := strings.TrimSpace(removeANSI(rawLine))

	if !strings.HasPrefix(line, "Copying blob ") {
		return
	}

	fields := strings.Fields(line)
	if len(fields) < 10 {
		return
	}

	// Пример: Copying blob ead6e2ffd75d [------] 192.0KiB / 525.6MiB | 28.3 KiB/s
	blobKey := fields[2]
	downloadedStr := fields[4]
	totalStr := fields[6]
	speed := fields[8] + " " + fields[9]

	downloadedBytes, err1 := parseSize(downloadedStr)
	totalBytes, err2 := parseSize(totalStr)
	if err1 != nil || err2 != nil || totalBytes == 0 {
		return
	}

	percent, changed := tracker.update(blobKey, downloadedBytes, totalBytes)
	if changed {
		p.reporter.CreateEventNotification(ctx, reply.StateBefore,
			reply.WithEventName(reply.EventSystemPullImage),
			reply.WithEventView(speed),
			reply.WithProgress(true),
			reply.WithProgressPercent(float64(percent)),
		)
	}
}

// parseSize разбирает строку типа "192.0KiB", "1.8GiB" и т.п.
func parseSize(sizeStr string) (float64, error) {
	re := regexp.MustCompile(`^([0-9.]+)([KMG]?i?B)$`)
	matches := re.FindStringSubmatch(sizeStr)
	if len(matches) != 3 {
		return 0, fmt.Errorf("cannot parse size: %s", sizeStr)
	}

	valueStr := matches[1]
	unit := matches[2]

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, err
	}

	switch unit {
	case "B":
	case "KiB":
		value *= 1024
	case "MiB":
		value *= 1024 * 1024
	case "GiB":
		value *= 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown suffix: %s", unit)
	}

	return value, nil
}

// PruneOldImages удаляет dangling-образы podman.
func (p *PodmanService) PruneOldImages(ctx context.Context) error {
	p.reporter.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemPruneOldImages))
	defer p.reporter.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemPruneOldImages))

	if stdout, stderr, err := p.runner.Run(ctx, []string{"podman", "image", "prune", "-f"}, command.WithQuiet()); err != nil {
		return fmt.Errorf(app.T_("Error deleting old images: %v, output: %s"), err, stdout+stderr)
	}

	stdout, _, err := p.runner.Run(ctx, []string{"podman", "images", "--noheading"}, command.WithQuiet())
	if err != nil {
		return fmt.Errorf(app.T_("Error retrieving podman image: %v"), err)
	}

	scanner := bufio.NewScanner(strings.NewReader(stdout))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		if fields[0] != "<none>" {
			continue
		}
		imageID := fields[2]
		if rmStdout, rmStderr, rmErr := p.runner.Run(ctx, []string{"podman", "rmi", "-f", imageID}, command.WithQuiet()); rmErr != nil {
			return fmt.Errorf(app.T_("Error deleting image %s: %v, output: %s\n"), imageID, rmErr, rmStdout+rmStderr)
		}
	}

	return nil
}

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func removeANSI(s string) string {
	return ansiRegexp.ReplaceAllString(s, "")
}

// BootcUpgrade запускает bootc upgrade с отображением прогресса.
func (p *PodmanService) BootcUpgrade(ctx context.Context, args []string) (string, error) {
	output, _, err := p.runner.Run(ctx, args,
		command.WithPTY(40, 120),
		command.WithEnv("TERM=xterm-256color", "LC_ALL=C"),
		command.WithStreamHandler(func(r io.Reader) {
			reader := bufio.NewReader(r)
			var lineBuffer bytes.Buffer
			for {
				b, readErr := reader.ReadByte()
				if readErr != nil {
					break
				}
				if b == '\n' || b == '\r' {
					if lineBuffer.Len() > 0 {
						p.parseBootcProgressLine(ctx, lineBuffer.String())
						lineBuffer.Reset()
					}
				} else {
					lineBuffer.WriteByte(b)
				}
			}
			if lineBuffer.Len() > 0 {
				p.parseBootcProgressLine(ctx, lineBuffer.String())
			}
		}),
	)
	if err != nil {
		return output, fmt.Errorf(app.T_("Command failed with error: %v"), err)
	}

	p.reporter.CreateEventNotification(ctx, reply.StateAfter,
		reply.WithEventName(reply.EventBootcLayers),
		reply.WithProgress(true),
		reply.WithProgressPercent(100),
	)
	p.reporter.CreateEventNotification(ctx, reply.StateAfter,
		reply.WithEventName(reply.EventBootcDownload),
		reply.WithProgress(true),
		reply.WithProgressPercent(100),
	)

	return output, nil
}

// parseBootcProgressLine парсит вывод bootc upgrade для отображения прогресса.
func (p *PodmanService) parseBootcProgressLine(ctx context.Context, rawLine string) {
	line := strings.TrimSpace(removeANSI(rawLine))

	if strings.Contains(line, "Fetching layers") {
		re := regexp.MustCompile(`(\d+)/(\d+)`)
		matches := re.FindStringSubmatch(line)
		if len(matches) == 3 {
			current, err1 := strconv.ParseFloat(matches[1], 64)
			total, err2 := strconv.ParseFloat(matches[2], 64)
			if err1 == nil && err2 == nil && total > 0 {
				percent := (current / total) * 100
				viewText := fmt.Sprintf(app.T_("Fetching layers %d/%d"), int(current), int(total))
				p.reporter.CreateEventNotification(ctx, reply.StateBefore,
					reply.WithEventName(reply.EventBootcLayers),
					reply.WithEventView(viewText),
					reply.WithProgress(true),
					reply.WithProgressPercent(percent),
				)
			}
		}
		return
	}

	if strings.Contains(line, "Fetching") && !strings.Contains(line, "Fetching layers") {
		re := regexp.MustCompile(`([\d.]+)\s+([KMG]iB)\s*/\s*([\d.]+)\s+([KMG]iB)\s+\(\s*([\d.]+)\s+([KMG]iB/s)\s*\)`)
		matches := re.FindStringSubmatch(line)
		if len(matches) == 7 {
			downloadedStr := matches[1] + matches[2]
			totalStr := matches[3] + matches[4]
			speed := matches[5] + " " + matches[6]

			downloadedBytes, errDownload := parseSize(downloadedStr)
			totalBytes, errBytes := parseSize(totalStr)
			if errDownload == nil && errBytes == nil && totalBytes > 0 {
				percent := (downloadedBytes / totalBytes) * 100
				p.reporter.CreateEventNotification(ctx, reply.StateBefore,
					reply.WithEventName(reply.EventBootcDownload),
					reply.WithEventView(speed),
					reply.WithProgress(true),
					reply.WithProgressPercent(percent),
				)
			}
		}
	}
}
