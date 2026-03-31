package altfiles

import (
	"bytes"
	"strings"
)

// PatchNsswitch добавляет altfiles в строки passwd и group nsswitch.conf
func PatchNsswitch(data []byte) []byte {
	var buf bytes.Buffer
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "passwd:") && !strings.Contains(trimmed, "altfiles") {
			line = patchPasswdLine(line)
		} else if strings.HasPrefix(trimmed, "group:") && !strings.Contains(trimmed, "altfiles") {
			line = patchGroupLine(line)
		}

		buf.WriteString(line)
		buf.WriteByte('\n')
	}

	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}
	return result
}

// patchPasswdLine добавляет altfiles после files в строке passwd
func patchPasswdLine(line string) string {
	return insertAfterFiles(line, "altfiles")
}

// patchGroupLine добавляет altfiles [SUCCESS=merge] после files [SUCCESS=merge] в строке group
func patchGroupLine(line string) string {
	return insertAfterFilesWithMerge(line)
}

// insertAfterFiles вставляет token после "files" в строке NSS
func insertAfterFiles(line string, token string) string {
	idx := strings.Index(line, "files")
	if idx == -1 {
		return line
	}

	insertPos := idx + len("files")
	rest := line[insertPos:]

	return line[:insertPos] + " " + token + rest
}

// insertAfterFilesWithMerge вставляет "altfiles [SUCCESS=merge]" после "files [SUCCESS=merge]"
func insertAfterFilesWithMerge(line string) string {
	mergePattern := "files [SUCCESS=merge]"
	idx := strings.Index(line, mergePattern)
	if idx != -1 {
		insertPos := idx + len(mergePattern)
		rest := line[insertPos:]
		return line[:insertPos] + " altfiles [SUCCESS=merge]" + rest
	}

	return insertAfterFiles(line, "altfiles [SUCCESS=merge]")
}
