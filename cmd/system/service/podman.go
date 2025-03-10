package service

import (
	"apm/cmd/common/reply"
	"apm/lib"
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

	command := fmt.Sprintf("%s podman image prune -f", lib.Env.CommandPrefix)
	cmd := exec.Command("sh", "-c", command)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ошибка удаления старых изображений: %v, output: %s", err, string(output))
	}

	command = fmt.Sprintf("%s podman image prune --noheading", lib.Env.CommandPrefix)
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
				fmt.Printf("ошибка удаления образа %s: %v, output: %s\n", imageID, err, string(out))
			}
		}
	}

	return nil
}
