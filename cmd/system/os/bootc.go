package os

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Image struct {
	Name           string `json:"name"`
	IsLocalStorage bool   `json:"issLocalStorage"`
}

// runUsrOverlay проверяет и активирует наложение файловой системы.
func runUsrOverlay() error {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return fmt.Errorf("failed to open /proc/mounts: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	runOverlay := true
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		device, mountpoint := fields[0], fields[1]
		if device == "overlay" && mountpoint == "/usr" {
			runOverlay = false
			break
		}
	}
	if scanner.Err() != nil {
		return scanner.Err()
	}

	if runOverlay {
		cmd := exec.Command("bootc", "usr-overlay")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to activate usr-overlay: %v, output: %s", err, string(output))
		}
	}

	return nil
}

func GetActiveImage() (string, error) {
	cmd := exec.Command("bash", "-c", "bootc status | yq '.status.booted.image.image.image'")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("не удалось определить образ командой bootc: %v", err)
	}

	stagedImage := strings.TrimSpace(string(output))
	if stagedImage == "" {
		return "", fmt.Errorf("образ оказался пустой строкой")
	}

	if strings.HasPrefix(stagedImage, "containers-storage:") {
		file, err := os.Open("/var/Containerfile")
		if err != nil {
			return "", fmt.Errorf("не удалось открыть файл Containerfile: %v", err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "FROM ") {
				imageName := strings.TrimSpace(line[len("FROM "):])
				if imageName != "" {
					return imageName, nil
				}
			}
		}
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("ошибка чтения Containerfile: %v", err)
		}

		return "", fmt.Errorf("не удалось определить образ дистрибутива")
	}

	return stagedImage, nil
}

// Switch переключает систему на новый образ.
func Switch() error {
	cmd := exec.Command("podman", "images", "-q", "os")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get podman image: %v", err)
	}

	podmanImageID := strings.TrimSpace(string(output))
	if podmanImageID == "" {
		return fmt.Errorf("no valid image found with tag 'os'. Build the image first")
	}

	cmd = exec.Command("bootc", "switch", "--transport", "containers-storage", podmanImageID)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to switch to the new image: %v, output: %s", err, string(output))
	}

	return nil
}

// checkAndUpdateBaseImage проверяет обновление базового образа.
func checkAndUpdateBaseImage() error {
	containerFile := "/var/Containerfile"
	// Получаем транспорт
	cmd := exec.Command("bash", "-c", "bootc status | yq '.status.booted.image.image.transport'")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get transport: %v", err)
	}
	transport := strings.TrimSpace(string(output))
	if transport != "containers-storage" {
		fmt.Println("Transport is not 'containers-storage'. Running bootc upgrade...")
		cmd = exec.Command("bootc", "upgrade")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("bootc upgrade failed: %v, output: %s", err, string(output))
		}
		return nil
	}

	if _, err := os.Stat(containerFile); err != nil {
		return fmt.Errorf("error: File %s does not exist", containerFile)
	}

	return rebuildAndSwitch()
}

// rebuildAndSwitch перестраивает и переключает систему на новый образ.
func rebuildAndSwitch() error {
	cmd := exec.Command("podman", "build", "--pull=always", "--squash", "-t", "os", "/var")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to rebuild the image: %v, output: %s", err, string(output))
	}

	if err := Switch(); err != nil {
		return err
	}

	return pruneOldImages()
}
