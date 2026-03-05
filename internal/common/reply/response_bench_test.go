package reply

import (
	"apm/internal/common/app"
	"fmt"
	"testing"
)

func makeTestConfig(formatType string, fields []string) *app.Config {
	return &app.Config{
		ConfigManager: &stubConfigManager{
			config: &app.Configuration{
				Format:     app.FormatText,
				FormatType: formatType,
				Fields:     fields,
				Colors:     testColors(),
			},
		},
	}
}

func testColors() app.Colors {
	return app.Colors{
		Enumerator: "#c4c8c6",
		Accent:     "#a2734c",
		ItemLight:  "#171717",
		ItemDark:   "#c4c8c6",
		Success:    "2",
		Error:      "9",
	}
}

type stubConfigManager struct {
	config *app.Configuration
}

func (s *stubConfigManager) GetConfig() *app.Configuration     { return s.config }
func (s *stubConfigManager) GetColors() app.Colors             { return s.config.Colors }
func (s *stubConfigManager) IsDevMode() bool                   { return false }
func (s *stubConfigManager) SetFormat(string)                  {}
func (s *stubConfigManager) SetFormatType(string)              {}
func (s *stubConfigManager) SetFields([]string)                {}
func (s *stubConfigManager) GetTemporaryImageFile() string     { return "" }
func (s *stubConfigManager) GetPathImageContainerFile() string { return "" }
func (s *stubConfigManager) GetPathImageFile() string          { return "" }
func (s *stubConfigManager) GetResourcesDir() string           { return "" }

// generateDeepData создаёт структуру с depth уровнями вложенности и width ключами на каждом уровне.
func generateDeepData(depth, width int) map[string]interface{} {
	if depth <= 0 {
		m := make(map[string]interface{}, width)
		for i := 0; i < width; i++ {
			m[fmt.Sprintf("field_%d", i)] = fmt.Sprintf("value_%d", i)
		}
		return m
	}
	m := make(map[string]interface{}, width)
	for i := 0; i < width; i++ {
		m[fmt.Sprintf("level%d_key%d", depth, i)] = generateDeepData(depth-1, width)
	}
	return m
}

// generateWideData создаёт плоскую структуру с n полями.
func generateWideData(n int) map[string]interface{} {
	m := make(map[string]interface{}, n)
	for i := 0; i < n; i++ {
		m[fmt.Sprintf("pkg_%04d", i)] = map[string]interface{}{
			"name":      fmt.Sprintf("package-%d", i),
			"version":   fmt.Sprintf("%d.0.0", i),
			"size":      i * 1024,
			"installed": i%2 == 0,
		}
	}
	return m
}

// generateListData создаёт структуру со списком из n элементов.
func generateListData(n int) map[string]interface{} {
	items := make([]interface{}, n)
	for i := 0; i < n; i++ {
		items[i] = map[string]interface{}{
			"name":          fmt.Sprintf("package-%d", i),
			"version":       fmt.Sprintf("%d.%d.%d", i/100, i/10%10, i%10),
			"size":          i * 512,
			"installedSize": i * 1024,
			"description":   fmt.Sprintf("Description for package %d with some extra text", i),
			"installed":     i%3 == 0,
		}
	}
	return map[string]interface{}{
		"message":  "Package list",
		"packages": items,
	}
}

// BenchmarkTreeDeep — глубокая вложенность: 6 уровней по 5 ключей = ~19500 узлов
func BenchmarkTreeDeep(b *testing.B) {
	r := NewResponseRenderer(makeTestConfig(app.FormatTypeTree, nil))
	data := generateDeepData(6, 5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.renderTree(data, false)
	}
}

// BenchmarkPlainDeep — то же для plain
func BenchmarkPlainDeep(b *testing.B) {
	r := NewResponseRenderer(makeTestConfig(app.FormatTypePlain, nil))
	data := generateDeepData(6, 5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.renderPlain(data)
	}
}

// BenchmarkTreeWide — 3000 вложенных объектов на верхнем уровне
func BenchmarkTreeWide(b *testing.B) {
	r := NewResponseRenderer(makeTestConfig(app.FormatTypeTree, nil))
	data := generateWideData(3000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.renderTree(data, false)
	}
}

// BenchmarkPlainWide — то же для plain
func BenchmarkPlainWide(b *testing.B) {
	r := NewResponseRenderer(makeTestConfig(app.FormatTypePlain, nil))
	data := generateWideData(3000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.renderPlain(data)
	}
}

// BenchmarkTreeList — список из 3000 пакетов (типичный сценарий)
func BenchmarkTreeList(b *testing.B) {
	r := NewResponseRenderer(makeTestConfig(app.FormatTypeTree, nil))
	data := generateListData(3000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.renderTree(data, false)
	}
}

// BenchmarkPlainList — то же для plain
func BenchmarkPlainList(b *testing.B) {
	r := NewResponseRenderer(makeTestConfig(app.FormatTypePlain, nil))
	data := generateListData(3000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.renderPlain(data)
	}
}

// BenchmarkFilterFields — фильтрация полей из 3000 элементов
func BenchmarkFilterFields(b *testing.B) {
	r := NewResponseRenderer(makeTestConfig(app.FormatTypePlain, []string{"name", "version"}))
	data := generateListData(3000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.renderText(data, false)
	}
}
