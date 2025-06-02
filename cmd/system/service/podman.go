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
	"apm/cmd/common/reply"
	"apm/lib"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/creack/pty"
)

func PullAndProgress(ctx context.Context, cmdLine string) (string, error) {
	allBlobs := make(map[string]bool)

	parts := strings.Fields(cmdLine)
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	env := os.Environ()
	env = append(env, "TERM=xterm-256color")
	env = append(env, "TMPDIR=/var/tmp")
	cmd.Env = env

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return "", err
	}
	defer func() { _ = ptmx.Close() }()

	// Устанавливаем размер терминала
	err = pty.Setsize(ptmx, &pty.Winsize{
		Rows: 40,
		Cols: 120,
	})
	if err != nil {
		return "", err
	}

	var outputBuffer bytes.Buffer

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Используем TeeReader для одновременного сканирования и записи в буфер
		scanner := bufio.NewScanner(io.TeeReader(ptmx, &outputBuffer))
		for scanner.Scan() {
			line := scanner.Text()
			parseProgressLine(ctx, line, allBlobs)
		}
		if scanErr := scanner.Err(); scanErr != nil && scanErr != io.EOF {
			// Можно добавить логирование ошибки
		}
	}()

	// Ждем завершения команды
	err = cmd.Wait()
	wg.Wait()

	if err != nil {
		// Возвращаем вывод вместе с ошибкой для более подробной диагностики
		return outputBuffer.String(), fmt.Errorf(lib.T_("Command failed with error: %v, output: %s"), err, outputBuffer.String())
	}

	for blobKey := range allBlobs {
		reply.CreateEventNotification(ctx, reply.StateAfter,
			reply.WithEventName("service.pullImage-"+blobKey),
			reply.WithProgress(true),
			reply.WithProgressPercent(100),
		)
	}

	return outputBuffer.String(), nil
}

// printProgress вызывается, когда мы успешно распознали
func printProgress(ctx context.Context, keyBlob string, progressPercent float64, speed string, allBlobs map[string]bool) {
	allBlobs[keyBlob] = true

	reply.CreateEventNotification(ctx, reply.StateBefore,
		reply.WithEventName("service.pullImage-"+keyBlob),
		reply.WithEventView(speed),
		reply.WithProgress(true),
		reply.WithProgressPercent(progressPercent),
	)
}

// parseProgressLine разбирает строки
func parseProgressLine(ctx context.Context, rawLine string, allBlobs map[string]bool) {
	line := strings.TrimSpace(removeANSI(rawLine))

	// Проверим, действительно ли строка начинается с "Copying blob "
	if !strings.HasPrefix(line, "Copying blob ") {
		return
	}

	fields := strings.Fields(line)
	if len(fields) < 10 {
		return
	}

	// Пример полей:
	//  0: Copying
	//  1: blob
	//  2: ead6e2ffd75d
	//  3: [--------------------------------------]
	//  4: 192.0KiB
	//  5: /
	//  6: 525.6MiB
	//  7: |
	//  8: 28.3
	//  9: KiB/s
	blobKey := fields[2]
	downloadedStr := fields[4]
	totalStr := fields[6]

	speed := fields[8] + " " + fields[9]

	downloadedBytes, err1 := parseSize(downloadedStr)
	totalBytes, err2 := parseSize(totalStr)
	if err1 != nil || err2 != nil || totalBytes == 0 {
		return
	}

	// Вычисляем % (float64)
	percent := (downloadedBytes / totalBytes) * 100
	printProgress(ctx, blobKey, percent, speed, allBlobs)
}

// parseSize разбирает строку типа "192.0KiB", "1.8GiB" и т.п.
func parseSize(sizeStr string) (float64, error) {
	re := regexp.MustCompile(`^([0-9.]+)([KMG]?i?B)$`)
	matches := re.FindStringSubmatch(sizeStr)
	if len(matches) != 3 {
		return 0, fmt.Errorf(lib.T_("Cannot parse size: %s"), sizeStr)
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
		return 0, fmt.Errorf(lib.T_("Unknown suffix: %s"), unit)
	}

	return value, nil
}

// pruneOldImages удаляет старые образы Podman.
func pruneOldImages(ctx context.Context) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.pruneOldImages"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.pruneOldImages"))

	command := fmt.Sprintf("%s podman image prune -f", lib.Env.CommandPrefix)
	cmd := exec.Command("sh", "-c", command)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf(lib.T_("Error deleting old images: %v, output: %s"), err, string(output))
	}

	command = fmt.Sprintf("%s podman images --noheading", lib.Env.CommandPrefix)
	cmd = exec.Command("sh", "-c", command)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf(lib.T_("Error retrieving podman image: %v"), err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[0] == "<none>" {
			imageID := fields[2]
			command = fmt.Sprintf("%s podman rmi -f %s", lib.Env.CommandPrefix, imageID)
			cmd = exec.Command("sh", "-c", command)
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf(lib.T_("Error deleting image %s: %v, output: %s\n"), imageID, err, string(out))
			}
		}
	}

	return nil
}

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func removeANSI(s string) string {
	return ansiRegexp.ReplaceAllString(s, "")
}
