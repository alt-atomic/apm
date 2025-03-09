package service

import (
	"apm/cmd/common/reply"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// pruneOldImages удаляет старые образы Podman.
func pruneOldImages(ctx context.Context) error {
	reply.CreateEventNotification(ctx, reply.StateBefore)
	defer reply.CreateEventNotification(ctx, reply.StateAfter)
	cmd := exec.Command("podman", "image", "prune", "-f")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ошибка удаления старых изображений: %v, output: %s", err, string(output))
	}

	// Получаем список образов.
	cmd = exec.Command("podman", "images", "--noheading")
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
			cmd = exec.Command("podman", "rmi", "-f", imageID)
			if out, err := cmd.CombinedOutput(); err != nil {
				fmt.Printf("ошибка удаления образа %s: %v, output: %s\n", imageID, err, string(out))
			}
		}
	}

	return nil
}
