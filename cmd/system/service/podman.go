package service

import (
	"apm/cmd/common/reply"
	"apm/lib"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/creack/pty"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

func PullAndProgress(ctx context.Context, cmdLine string) error {
	allBlobs := make(map[string]bool)

	parts := strings.Fields(cmdLine)
	cmd := exec.Command(parts[0], parts[1:]...)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	defer func() { _ = ptmx.Close() }()

	// Установим размер терминала
	err = pty.Setsize(ptmx, &pty.Winsize{
		Rows: 40,
		Cols: 120,
	})
	if err != nil {
		return err
	}

	// Переменные окружения
	env := os.Environ()
	env = append(env, "TERM=xterm-256color")
	cmd.Env = env

	go func() {
		scanner := bufio.NewScanner(ptmx)
		for scanner.Scan() {
			line := scanner.Text()
			parseProgressLine(ctx, line, allBlobs)
		}
		if scanErr := scanner.Err(); scanErr != nil && scanErr != io.EOF {
			//lib.Log.Error("Ошибка сканирования вывода: %v\n", scanErr)
		}
	}()

	if err = cmd.Wait(); err != nil {
		return err
	}

	for blobKey := range allBlobs {
		reply.CreateEventNotification(ctx, reply.StateAfter,
			reply.WithEventName("service.pullImage-"+blobKey),
			reply.WithProgress(true),
			reply.WithProgressPercent(100),
		)
	}

	return nil
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
func parseProgressLine(ctx context.Context, line string, allBlobs map[string]bool) {
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
		return 0, fmt.Errorf("не могу разобрать размер: %s", sizeStr)
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
		return 0, fmt.Errorf("неизвестный суффикс: %s", unit)
	}

	return value, nil
}

// pruneOldImages удаляет старые образы Podman.
func pruneOldImages(ctx context.Context) error {
	reply.CreateEventNotification(ctx, reply.StateBefore)
	defer reply.CreateEventNotification(ctx, reply.StateAfter)

	command := fmt.Sprintf("%s podman image prune -f", lib.Env.CommandPrefix)
	cmd := exec.Command("sh", "-c", command)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ошибка удаления старых изображений: %v, output: %s", err, string(output))
	}

	command = fmt.Sprintf("%s podman images --noheading", lib.Env.CommandPrefix)
	cmd = exec.Command("sh", "-c", command)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("ошибка получения образа podman: %v", err)
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
				return fmt.Errorf("ошибка удаления образа %s: %v, output: %s\n", imageID, err, string(out))
			}
		}
	}

	return nil
}
