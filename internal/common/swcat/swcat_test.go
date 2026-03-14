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

package swcat

import (
	"encoding/json"
	"encoding/xml"
	"reflect"
	"testing"
)

func TestXMLParseRealComponent(t *testing.T) {
	raw := `<component type="desktop">
		<id>0ad.desktop</id>
		<metadata_license>CC0-1.0</metadata_license>
		<name>0 A.D.</name>
		<summary>A real-time strategy game of ancient warfare</summary>
		<summary xml:lang="de">Ein Echtzeitstrategiespiel</summary>
		<summary xml:lang="ru">Игра в жанре исторической стратегии в реальном времени</summary>
		<url type="homepage">https://play0ad.com/</url>
		<description>
			<p>0 A.D. is a free software, cross-platform RTS game.</p>
		</description>
		<keywords>
			<keyword>RTS</keyword>
			<keyword>History</keyword>
			<keyword>Warfare</keyword>
		</keywords>
		<categories>
			<category>Game</category>
			<category>StrategyGame</category>
		</categories>
		<pkgname>0ad</pkgname>
		<releases>
			<release date="2024-10-15" version="0.27.1" type="stable"/>
		</releases>
	</component>`

	var comp Component
	if err := xml.Unmarshal([]byte(raw), &comp); err != nil {
		t.Fatalf("xml.Unmarshal failed: %v", err)
	}

	if comp.Type != "desktop" {
		t.Errorf("Type = %q, want %q", comp.Type, "desktop")
	}
	if comp.ID != "0ad.desktop" {
		t.Errorf("ID = %q, want %q", comp.ID, "0ad.desktop")
	}
	if comp.PkgName != "0ad" {
		t.Errorf("PkgName = %q, want %q", comp.PkgName, "0ad")
	}
	if len(comp.Summary) != 3 {
		t.Errorf("Summary count = %d, want 3", len(comp.Summary))
	}
	if len(comp.Keywords) != 3 {
		t.Errorf("Keywords count = %d, want 3", len(comp.Keywords))
	}
	if len(comp.Categories) != 2 {
		t.Errorf("Categories count = %d, want 2", len(comp.Categories))
	}
	if len(comp.Urls) != 1 {
		t.Errorf("URLs count = %d, want 1", len(comp.Urls))
	}
	if len(comp.Releases) != 1 {
		t.Errorf("Releases count = %d, want 1", len(comp.Releases))
	}
	if comp.Releases[0].Version != "0.27.1" {
		t.Errorf("Release version = %q, want %q", comp.Releases[0].Version, "0.27.1")
	}
	if comp.Releases[0].Date != "2024-10-15" {
		t.Errorf("Release date = %q, want %q", comp.Releases[0].Date, "2024-10-15")
	}
	if comp.Releases[0].Type != "stable" {
		t.Errorf("Release type = %q, want %q", comp.Releases[0].Type, "stable")
	}
}

func TestXMLParseReleaseWithDetails(t *testing.T) {
	raw := `<component type="desktop">
		<id>test.desktop</id>
		<pkgname>test</pkgname>
		<releases>
			<release date="2024-10-15" version="1.2.0" type="stable" urgency="high">
				<description>
					<p>New features and bug fixes</p>
					<p xml:lang="ru">Новые функции и исправления</p>
				</description>
				<url type="details">https://example.com/releases/1.2.0</url>
			</release>
			<release date="2024-08-01" version="1.1.0"/>
		</releases>
	</component>`

	var comp Component
	if err := xml.Unmarshal([]byte(raw), &comp); err != nil {
		t.Fatalf("xml.Unmarshal failed: %v", err)
	}

	if len(comp.Releases) != 2 {
		t.Fatalf("Releases count = %d, want 2", len(comp.Releases))
	}

	r := comp.Releases[0]
	if r.Version != "1.2.0" {
		t.Errorf("Release.Version = %q, want %q", r.Version, "1.2.0")
	}
	if r.Date != "2024-10-15" {
		t.Errorf("Release.Date = %q, want %q", r.Date, "2024-10-15")
	}
	if r.Type != "stable" {
		t.Errorf("Release.Type = %q, want %q", r.Type, "stable")
	}
	if r.Urgency != "high" {
		t.Errorf("Release.Urgency = %q, want %q", r.Urgency, "high")
	}
	if len(r.Description) == 0 {
		t.Fatal("Release.Description is empty")
	}
	if r.URL == nil {
		t.Fatal("Release.URL is nil")
	}
	if r.URL.Value != "https://example.com/releases/1.2.0" {
		t.Errorf("Release.URL.Value = %q, want %q", r.URL.Value, "https://example.com/releases/1.2.0")
	}
	if r.URL.Type != "details" {
		t.Errorf("Release.URL.Type = %q, want %q", r.URL.Type, "details")
	}

	r2 := comp.Releases[1]
	if r2.Version != "1.1.0" {
		t.Errorf("Release[1].Version = %q, want %q", r2.Version, "1.1.0")
	}
	if r2.Date != "2024-08-01" {
		t.Errorf("Release[1].Date = %q, want %q", r2.Date, "2024-08-01")
	}
}

func TestXMLParseLaunchableAndContentRating(t *testing.T) {
	raw := `<component type="desktop">
		<id>net.86box.86Box</id>
		<pkgname>86box</pkgname>
		<launchable type="desktop-id">net.86box.86Box.desktop</launchable>
		<content_rating type="oars-1.0">
			<content_attribute id="violence-cartoon">mild</content_attribute>
			<content_attribute id="violence-fantasy">mild</content_attribute>
		</content_rating>
	</component>`

	var comp Component
	if err := xml.Unmarshal([]byte(raw), &comp); err != nil {
		t.Fatalf("xml.Unmarshal failed: %v", err)
	}

	if comp.Launchable == nil {
		t.Fatal("Launchable is nil")
	}
	if comp.Launchable.Type != "desktop-id" {
		t.Errorf("Launchable.Type = %q, want %q", comp.Launchable.Type, "desktop-id")
	}
	if comp.Launchable.Value != "net.86box.86Box.desktop" {
		t.Errorf("Launchable.Value = %q, want %q", comp.Launchable.Value, "net.86box.86Box.desktop")
	}

	if comp.ContentRating == nil {
		t.Fatal("ContentRating is nil")
	}
	if comp.ContentRating.Type != "oars-1.0" {
		t.Errorf("ContentRating.Type = %q, want %q", comp.ContentRating.Type, "oars-1.0")
	}
	if len(comp.ContentRating.Attributes) != 2 {
		t.Errorf("ContentRating.Attributes count = %d, want 2", len(comp.ContentRating.Attributes))
	}
}

func TestXMLParseScreenshots(t *testing.T) {
	raw := `<component type="desktop">
		<id>test.desktop</id>
		<pkgname>test</pkgname>
		<screenshots>
			<screenshot type="default" environment="gnome">
				<caption>Main screen</caption>
				<caption xml:lang="ru">Главный экран</caption>
				<image type="source" width="905" height="650">https://example.com/screen.png</image>
			</screenshot>
		</screenshots>
	</component>`

	var comp Component
	if err := xml.Unmarshal([]byte(raw), &comp); err != nil {
		t.Fatalf("xml.Unmarshal failed: %v", err)
	}

	if len(comp.Screenshots) != 1 {
		t.Fatalf("Screenshots count = %d, want 1", len(comp.Screenshots))
	}
	screenshot := comp.Screenshots[0]
	if screenshot.Type != "default" {
		t.Errorf("Screenshot.Type = %q, want %q", screenshot.Type, "default")
	}
	if screenshot.Environment != "gnome" {
		t.Errorf("Screenshot.Environment = %q, want %q", screenshot.Environment, "gnome")
	}
	if len(screenshot.Caption) != 2 {
		t.Errorf("Caption count = %d, want 2", len(screenshot.Caption))
	}
	if len(screenshot.Images) != 1 {
		t.Fatalf("Images count = %d, want 1", len(screenshot.Images))
	}
	if screenshot.Images[0].Width != 905 || screenshot.Images[0].Height != 650 {
		t.Errorf("Image size = %dx%d, want 905x650", screenshot.Images[0].Width, screenshot.Images[0].Height)
	}
}

func TestXMLParseIcons(t *testing.T) {
	raw := `<component type="desktop">
		<id>7colors.desktop</id>
		<pkgname>7colors</pkgname>
		<icon type="cached" width="64" height="64">7colors.png</icon>
		<icon type="cached" width="128" height="128">7colors.png</icon>
	</component>`

	var comp Component
	if err := xml.Unmarshal([]byte(raw), &comp); err != nil {
		t.Fatalf("xml.Unmarshal failed: %v", err)
	}

	if len(comp.Icons) != 2 {
		t.Fatalf("Icons count = %d, want 2", len(comp.Icons))
	}
	if comp.Icons[0].Width != 64 || comp.Icons[1].Width != 128 {
		t.Errorf("Icon widths = [%d, %d], want [64, 128]", comp.Icons[0].Width, comp.Icons[1].Width)
	}
}

func TestLocalizedMapMarshalJSON(t *testing.T) {
	input := LocalizedMap{
		{Lang: "", Value: "0 A.D."},
		{Lang: "ru", Value: "0 A.D. (рус)"},
	}
	data, err := json.Marshal(&input)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal result failed: %v", err)
	}

	if result["C"] != "0 A.D." {
		t.Errorf("key C = %q, want %q", result["C"], "0 A.D.")
	}
	if result["ru"] != "0 A.D. (рус)" {
		t.Errorf("key ru = %q, want %q", result["ru"], "0 A.D. (рус)")
	}
}

func TestLocalizedMapMarshalJSONDuplicateLang(t *testing.T) {
	input := LocalizedMap{
		{Lang: "en", Value: "First"},
		{Lang: "en", Value: "Second"},
	}
	data, err := json.Marshal(&input)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal result failed: %v", err)
	}

	if result["en"] != "First" {
		t.Errorf("duplicate lang should keep first value, got %q", result["en"])
	}
}

func TestLocalizedMapUnmarshalJSON(t *testing.T) {
	data := `{"C": "Firefox", "ru": "Фаерфокс"}`
	var result LocalizedMap
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	found := make(map[string]string)
	for _, item := range result {
		found[item.Lang] = item.Value
	}
	if found[""] != "Firefox" {
		t.Errorf("C lang should map to empty string, got lang entries: %v", found)
	}
	if found["ru"] != "Фаерфокс" {
		t.Errorf("ru = %q, want %q", found["ru"], "Фаерфокс")
	}
}

func TestURLMapMarshalJSON(t *testing.T) {
	input := URLMap{
		{Type: "homepage", Value: "https://play0ad.com/"},
		{Type: "bugtracker", Value: "https://bugs.example.com/"},
	}
	data, err := json.Marshal(&input)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal result failed: %v", err)
	}

	if result["homepage"] != "https://play0ad.com/" {
		t.Errorf("homepage = %q, want %q", result["homepage"], "https://play0ad.com/")
	}
	if result["bugtracker"] != "https://bugs.example.com/" {
		t.Errorf("bugtracker = %q, want %q", result["bugtracker"], "https://bugs.example.com/")
	}
}

func TestURLMapUnmarshalJSON(t *testing.T) {
	data := `{"homepage": "https://example.com", "bugtracker": "https://bugs.example.com"}`
	var result URLMap
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	found := make(map[string]string)
	for _, u := range result {
		found[u.Type] = u.Value
	}
	if found["homepage"] != "https://example.com" {
		t.Errorf("homepage = %q, want %q", found["homepage"], "https://example.com")
	}
}

func TestKeywordListMarshalJSON(t *testing.T) {
	input := KeywordList{
		{Value: "RTS"},
		{Value: "History"},
		{Value: "Warfare"},
	}
	data, err := json.Marshal(&input)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result []string
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal result failed: %v", err)
	}

	expected := []string{"RTS", "History", "Warfare"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("got %v, want %v", result, expected)
	}
}

func TestKeywordListUnmarshalJSON(t *testing.T) {
	data := `["RTS", "History", "Warfare"]`
	var result KeywordList
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(result) != 3 || result[0].Value != "RTS" || result[2].Value != "Warfare" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestContentAttributeMapMarshalJSON(t *testing.T) {
	input := ContentAttributeMap{
		{ID: "violence-cartoon", Value: "mild"},
		{ID: "violence-fantasy", Value: "mild"},
	}
	data, err := json.Marshal(&input)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal result failed: %v", err)
	}

	if result["violence-cartoon"] != "mild" {
		t.Errorf("violence-cartoon = %q, want %q", result["violence-cartoon"], "mild")
	}
	if result["violence-fantasy"] != "mild" {
		t.Errorf("violence-fantasy = %q, want %q", result["violence-fantasy"], "mild")
	}
}

func TestContentAttributeMapUnmarshalJSON(t *testing.T) {
	data := `{"violence-cartoon": "mild", "social-chat": "intense"}`
	var result ContentAttributeMap
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	found := make(map[string]string)
	for _, a := range result {
		found[a.ID] = a.Value
	}
	if found["violence-cartoon"] != "mild" {
		t.Errorf("violence-cartoon = %q, want %q", found["violence-cartoon"], "mild")
	}
	if found["social-chat"] != "intense" {
		t.Errorf("social-chat = %q, want %q", found["social-chat"], "intense")
	}
}

func TestLaunchableMarshalJSON(t *testing.T) {
	input := Launchable{Type: "desktop-id", Value: "net.86box.86Box.desktop"}
	data, err := json.Marshal(&input)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result map[string]string
	if err = json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal result failed: %v", err)
	}

	if result["desktop-id"] != "net.86box.86Box.desktop" {
		t.Errorf("desktop-id = %q, want %q", result["desktop-id"], "net.86box.86Box.desktop")
	}
}

func TestLaunchableUnmarshalJSON(t *testing.T) {
	data := `{"desktop-id": "org.firefox.desktop"}`
	var result Launchable
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result.Type != "desktop-id" || result.Value != "org.firefox.desktop" {
		t.Errorf("got {Type:%q, Value:%q}, want {Type:%q, Value:%q}",
			result.Type, result.Value, "desktop-id", "org.firefox.desktop")
	}
}

func TestComponentJSONRoundtrip(t *testing.T) {
	raw := `<component type="desktop">
		<id>net.86box.86Box</id>
		<metadata_license>CC0-1.0</metadata_license>
		<project_license>GPL-2.0-or-later</project_license>
		<name>86Box</name>
		<name xml:lang="ru">86Box (рус)</name>
		<summary>An emulator for classic IBM PC clones</summary>
		<url type="homepage">https://86box.net</url>
		<url type="bugtracker">https://bugs.86box.net</url>
		<launchable type="desktop-id">net.86box.86Box.desktop</launchable>
		<content_rating type="oars-1.0">
			<content_attribute id="violence-cartoon">mild</content_attribute>
		</content_rating>
		<keywords>
			<keyword>emulator</keyword>
			<keyword>PC</keyword>
		</keywords>
		<categories>
			<category>Emulator</category>
		</categories>
		<screenshots>
			<screenshot type="default">
				<caption>Main window</caption>
				<caption xml:lang="ru">Главное окно</caption>
				<image type="source" width="800" height="600">https://example.com/screen.png</image>
			</screenshot>
		</screenshots>
		<pkgname>86box</pkgname>
		<releases>
			<release date="2025-01-15" version="5.3" type="stable">
				<description><p>Bug fixes and improvements</p></description>
				<url type="details">https://example.com/release</url>
			</release>
		</releases>
		<icon type="cached" width="64" height="64">net.86box.86Box.png</icon>
		<icon type="cached" width="128" height="128">net.86box.86Box.png</icon>
	</component>`

	var original Component
	if err := xml.Unmarshal([]byte(raw), &original); err != nil {
		t.Fatalf("xml.Unmarshal failed: %v", err)
	}

	jsonData, err := json.Marshal(&original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var restored Component
	if err := json.Unmarshal(jsonData, &restored); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if restored.Type != original.Type {
		t.Errorf("Type = %q, want %q", restored.Type, original.Type)
	}
	if restored.ID != original.ID {
		t.Errorf("ID = %q, want %q", restored.ID, original.ID)
	}
	if restored.PkgName != original.PkgName {
		t.Errorf("PkgName = %q, want %q", restored.PkgName, original.PkgName)
	}
	if restored.ProjectLicense != original.ProjectLicense {
		t.Errorf("ProjectLicense = %q, want %q", restored.ProjectLicense, original.ProjectLicense)
	}
	if len(restored.Name) != len(original.Name) {
		t.Errorf("Name count = %d, want %d", len(restored.Name), len(original.Name))
	}
	if len(restored.Urls) != len(original.Urls) {
		t.Errorf("URLs count = %d, want %d", len(restored.Urls), len(original.Urls))
	}
	if len(restored.Keywords) != len(original.Keywords) {
		t.Errorf("Keywords count = %d, want %d", len(restored.Keywords), len(original.Keywords))
	}
	if len(restored.Icons) != len(original.Icons) {
		t.Errorf("Icons count = %d, want %d", len(restored.Icons), len(original.Icons))
	}
	if restored.Launchable == nil {
		t.Fatal("Launchable is nil after roundtrip")
	}
	if restored.Launchable.Type != original.Launchable.Type || restored.Launchable.Value != original.Launchable.Value {
		t.Errorf("Launchable = {%q: %q}, want {%q: %q}",
			restored.Launchable.Type, restored.Launchable.Value,
			original.Launchable.Type, original.Launchable.Value)
	}
	if restored.ContentRating == nil {
		t.Fatal("ContentRating is nil after roundtrip")
	}
	if len(restored.ContentRating.Attributes) != len(original.ContentRating.Attributes) {
		t.Errorf("ContentRating.Attributes count = %d, want %d",
			len(restored.ContentRating.Attributes), len(original.ContentRating.Attributes))
	}
	if len(restored.Screenshots) != 1 {
		t.Fatalf("Screenshots count = %d, want 1", len(restored.Screenshots))
	}
	if len(restored.Screenshots[0].Caption) != len(original.Screenshots[0].Caption) {
		t.Errorf("Screenshot Caption count = %d, want %d",
			len(restored.Screenshots[0].Caption), len(original.Screenshots[0].Caption))
	}
}

func TestLegacyFieldsFallback(t *testing.T) {
	raw := `<component type="desktop">
		<id>legacy.desktop</id>
		<pkgname>legacy-app</pkgname>
		<_name>Legacy Name</_name>
		<_summary>Legacy Summary</_summary>
		<licence>GPL-2.0</licence>
		<metadata_licence>CC0-1.0</metadata_licence>
		<updatecontact>dev@example.com</updatecontact>
		<launch>legacy.desktop</launch>
	</component>`

	var comp Component
	if err := xml.Unmarshal([]byte(raw), &comp); err != nil {
		t.Fatalf("xml.Unmarshal failed: %v", err)
	}
	sanitizeComponent(&comp)

	if len(comp.Name) == 0 || comp.Name[0].Value != "Legacy Name" {
		t.Errorf("Name not populated from <_name>, got %v", comp.Name)
	}
	if len(comp.Summary) == 0 || comp.Summary[0].Value != "Legacy Summary" {
		t.Errorf("Summary not populated from <_summary>, got %v", comp.Summary)
	}
	if comp.MetadataLicense != "CC0-1.0" {
		t.Errorf("MetadataLicense = %q, want %q", comp.MetadataLicense, "CC0-1.0")
	}
	if comp.ProjectLicense != "GPL-2.0" {
		t.Errorf("ProjectLicense = %q, want %q", comp.ProjectLicense, "GPL-2.0")
	}
	if comp.UpdateContact != "dev@example.com" {
		t.Errorf("UpdateContact = %q, want %q", comp.UpdateContact, "dev@example.com")
	}
	if comp.Launchable == nil || comp.Launchable.Value != "legacy.desktop" {
		t.Errorf("Launchable not populated from <launch>, got %v", comp.Launchable)
	}
}

func TestLegacyFieldsNoOverride(t *testing.T) {
	raw := `<component type="desktop">
		<id>modern.desktop</id>
		<pkgname>modern-app</pkgname>
		<name>Modern Name</name>
		<summary>Modern Summary</summary>
		<project_license>MIT</project_license>
		<metadata_license>CC0-1.0</metadata_license>
		<update_contact>modern@example.com</update_contact>
		<launchable type="desktop-id">modern.desktop</launchable>
		<_name>Legacy Name</_name>
		<licence>GPL-2.0</licence>
		<updatecontact>legacy@example.com</updatecontact>
	</component>`

	var comp Component
	if err := xml.Unmarshal([]byte(raw), &comp); err != nil {
		t.Fatalf("xml.Unmarshal failed: %v", err)
	}
	sanitizeComponent(&comp)

	if comp.Name[0].Value != "Modern Name" {
		t.Errorf("Name should not be overridden, got %q", comp.Name[0].Value)
	}
	if comp.ProjectLicense != "MIT" {
		t.Errorf("ProjectLicense should not be overridden, got %q", comp.ProjectLicense)
	}
	if comp.UpdateContact != "modern@example.com" {
		t.Errorf("UpdateContact should not be overridden, got %q", comp.UpdateContact)
	}
	if comp.Launchable.Value != "modern.desktop" {
		t.Errorf("Launchable should not be overridden, got %q", comp.Launchable.Value)
	}
}

func TestSanitizeComponent(t *testing.T) {
	comp := Component{
		Name: LocalizedMap{
			{Lang: "", Value: "App"},
			{Lang: "", Value: "App"},
			{Lang: "ru", Value: "Приложение"},
		},
		Description: LocalizedMap{
			{Lang: "", Value: "<p>A <b>bold</b> description with &amp; entities.</p>"},
		},
	}

	sanitizeComponent(&comp)

	if len(comp.Name) != 2 {
		t.Errorf("Name count after dedup = %d, want 2", len(comp.Name))
	}
	if comp.Description[0].Value != "<p>A <b>bold</b> description with &amp; entities.</p>" {
		t.Errorf("Description should preserve HTML, got %q", comp.Description[0].Value)
	}
}

func TestDedupTexts(t *testing.T) {
	tests := []struct {
		name     string
		input    LocalizedMap
		expected LocalizedMap
	}{
		{
			name: "No duplicates",
			input: LocalizedMap{
				{Lang: "en", Value: "Hello"},
				{Lang: "ru", Value: "Привет"},
				{Lang: "fr", Value: "Bonjour"},
			},
			expected: LocalizedMap{
				{Lang: "en", Value: "Hello"},
				{Lang: "ru", Value: "Привет"},
				{Lang: "fr", Value: "Bonjour"},
			},
		},
		{
			name: "Exact duplicates",
			input: LocalizedMap{
				{Lang: "en", Value: "Hello"},
				{Lang: "en", Value: "Hello"},
				{Lang: "ru", Value: "Привет"},
			},
			expected: LocalizedMap{
				{Lang: "en", Value: "Hello"},
				{Lang: "ru", Value: "Привет"},
			},
		},
		{
			name: "Same value different language",
			input: LocalizedMap{
				{Lang: "en", Value: "OK"},
				{Lang: "fr", Value: "OK"},
				{Lang: "de", Value: "OK"},
			},
			expected: LocalizedMap{
				{Lang: "en", Value: "OK"},
				{Lang: "fr", Value: "OK"},
				{Lang: "de", Value: "OK"},
			},
		},
		{
			name: "Same language different value",
			input: LocalizedMap{
				{Lang: "en", Value: "Hello"},
				{Lang: "en", Value: "Hi"},
				{Lang: "en", Value: "Hey"},
			},
			expected: LocalizedMap{
				{Lang: "en", Value: "Hello"},
				{Lang: "en", Value: "Hi"},
				{Lang: "en", Value: "Hey"},
			},
		},
		{
			name: "Empty language",
			input: LocalizedMap{
				{Lang: "", Value: "Default"},
				{Lang: "en", Value: "English"},
				{Lang: "", Value: "Default"},
			},
			expected: LocalizedMap{
				{Lang: "", Value: "Default"},
				{Lang: "en", Value: "English"},
			},
		},
		{
			name: "Empty value",
			input: LocalizedMap{
				{Lang: "en", Value: ""},
				{Lang: "ru", Value: "Text"},
				{Lang: "en", Value: ""},
			},
			expected: LocalizedMap{
				{Lang: "en", Value: ""},
				{Lang: "ru", Value: "Text"},
			},
		},
		{
			name:     "Empty slice",
			input:    LocalizedMap{},
			expected: LocalizedMap{},
		},
		{
			name: "Single element",
			input: LocalizedMap{
				{Lang: "en", Value: "Single"},
			},
			expected: LocalizedMap{
				{Lang: "en", Value: "Single"},
			},
		},
		{
			name: "Complex duplicates pattern",
			input: LocalizedMap{
				{Lang: "en", Value: "App"},
				{Lang: "ru", Value: "Приложение"},
				{Lang: "en", Value: "App"},
				{Lang: "fr", Value: "Application"},
				{Lang: "ru", Value: "Приложение"},
				{Lang: "en", Value: "Application"},
				{Lang: "fr", Value: "Application"},
			},
			expected: LocalizedMap{
				{Lang: "en", Value: "App"},
				{Lang: "ru", Value: "Приложение"},
				{Lang: "fr", Value: "Application"},
				{Lang: "en", Value: "Application"},
			},
		},
		{
			name: "Regional language codes",
			input: LocalizedMap{
				{Lang: "en_US", Value: "Color"},
				{Lang: "en_GB", Value: "Colour"},
				{Lang: "en_US", Value: "Color"},
				{Lang: "pt_BR", Value: "Português brasileiro"},
			},
			expected: LocalizedMap{
				{Lang: "en_US", Value: "Color"},
				{Lang: "en_GB", Value: "Colour"},
				{Lang: "pt_BR", Value: "Português brasileiro"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dedupTexts(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("dedupTexts() returned %d items, want %d", len(result), len(tt.expected))
				return
			}

			seen := make(map[string]struct{})
			for _, item := range result {
				key := item.Lang + "\x00" + item.Value
				if _, exists := seen[key]; exists {
					t.Errorf("dedupTexts() returned duplicate: Lang=%q, Value=%q", item.Lang, item.Value)
					return
				}
				seen[key] = struct{}{}
			}

			expectedMap := make(map[string]struct{})
			for _, expected := range tt.expected {
				key := expected.Lang + "\x00" + expected.Value
				expectedMap[key] = struct{}{}
			}

			for _, item := range result {
				key := item.Lang + "\x00" + item.Value
				if _, exists := expectedMap[key]; !exists {
					t.Errorf("dedupTexts() returned unexpected item: Lang=%q, Value=%q", item.Lang, item.Value)
				}
			}
		})
	}
}

func TestDedupTextsOrder(t *testing.T) {
	input := LocalizedMap{
		{Lang: "en", Value: "First"},
		{Lang: "ru", Value: "Второй"},
		{Lang: "en", Value: "First"},
		{Lang: "fr", Value: "Troisième"},
		{Lang: "ru", Value: "Второй"},
	}

	result := dedupTexts(input)

	expected := LocalizedMap{
		{Lang: "en", Value: "First"},
		{Lang: "ru", Value: "Второй"},
		{Lang: "fr", Value: "Troisième"},
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("dedupTexts() order test failed.\nGot:      %+v\nExpected: %+v", result, expected)
	}
}

func TestDedupTextsPerformance(t *testing.T) {
	large := make(LocalizedMap, 1000)
	for i := 0; i < 1000; i++ {
		large[i] = LocalizedText{
			Lang:  "en",
			Value: "Text",
		}
	}

	result := dedupTexts(large)

	if len(result) != 1 {
		t.Errorf("dedupTexts() expected 1 item, got %d", len(result))
	}

	if result[0].Lang != "en" || result[0].Value != "Text" {
		t.Errorf("dedupTexts() wrong content: %+v", result[0])
	}
}

func TestComponentSliceJSONRoundtrip(t *testing.T) {
	components := []Component{
		{
			Type:    "desktop-application",
			ID:      "org.gnome.Evince",
			PkgName: "evince",
			Name:    LocalizedMap{{Lang: "", Value: "Document Viewer"}},
			Summary: LocalizedMap{{Lang: "", Value: "View documents"}},
			Urls:    URLMap{{Type: "homepage", Value: "https://wiki.gnome.org/Apps/Evince"}},
			Launchable: &Launchable{
				Type:  "desktop-id",
				Value: "org.gnome.Evince.desktop",
			},
		},
		{
			Type:    "addon",
			ID:      "evince-comicsdocument",
			PkgName: "evince",
			Name:    LocalizedMap{{Lang: "", Value: "Comic Books"}, {Lang: "ru", Value: "Книги комиксов"}},
		},
		{
			Type:    "addon",
			ID:      "evince-pdfdocument",
			PkgName: "evince",
			Name:    LocalizedMap{{Lang: "", Value: "PDF Documents"}},
		},
	}

	data, err := json.Marshal(components)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var restored []Component
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if len(restored) != 3 {
		t.Fatalf("expected 3 components, got %d", len(restored))
	}

	if restored[0].Type != "desktop-application" {
		t.Errorf("component[0].Type = %q, want %q", restored[0].Type, "desktop-application")
	}
	if restored[0].ID != "org.gnome.Evince" {
		t.Errorf("component[0].ID = %q, want %q", restored[0].ID, "org.gnome.Evince")
	}
	if restored[0].Launchable == nil || restored[0].Launchable.Value != "org.gnome.Evince.desktop" {
		t.Error("component[0].Launchable lost after roundtrip")
	}

	if restored[1].Type != "addon" {
		t.Errorf("component[1].Type = %q, want %q", restored[1].Type, "addon")
	}
	if restored[1].ID != "evince-comicsdocument" {
		t.Errorf("component[1].ID = %q, want %q", restored[1].ID, "evince-comicsdocument")
	}
	if len(restored[1].Name) != 2 {
		t.Errorf("component[1].Name count = %d, want 2", len(restored[1].Name))
	}

	if restored[2].ID != "evince-pdfdocument" {
		t.Errorf("component[2].ID = %q, want %q", restored[2].ID, "evince-pdfdocument")
	}
}

func TestComponentSliceJSONEmpty(t *testing.T) {
	var empty []Component
	data, err := json.Marshal(empty)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if string(data) != "null" {
		t.Errorf("expected null, got %s", data)
	}

	emptySlice := []Component{}
	data, err = json.Marshal(emptySlice)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if string(data) != "[]" {
		t.Errorf("expected [], got %s", data)
	}

	var restored []Component
	if err := json.Unmarshal([]byte("[]"), &restored); err != nil {
		t.Fatalf("json.Unmarshal [] failed: %v", err)
	}
	if len(restored) != 0 {
		t.Errorf("expected 0 components, got %d", len(restored))
	}

	if err := json.Unmarshal([]byte("null"), &restored); err != nil {
		t.Fatalf("json.Unmarshal null failed: %v", err)
	}
	if restored != nil {
		t.Errorf("expected nil, got %v", restored)
	}
}

func TestComponentSliceMultipleDesktopApps(t *testing.T) {
	components := []Component{
		{
			Type:       "desktop",
			ID:         "dribble-Frodo.desktop",
			PkgName:    "Frodo",
			Name:       LocalizedMap{{Lang: "", Value: "Frodo"}},
			Categories: []string{"Game", "Emulator"},
		},
		{
			Type:       "desktop",
			ID:         "dribble-FrodoPC.desktop",
			PkgName:    "Frodo",
			Name:       LocalizedMap{{Lang: "", Value: "FrodoPC"}},
			Categories: []string{"Game", "Emulator"},
		},
		{
			Type:       "desktop",
			ID:         "dribble-FrodoSC.desktop",
			PkgName:    "Frodo",
			Name:       LocalizedMap{{Lang: "", Value: "FrodoSC"}},
			Categories: []string{"Game", "Emulator"},
		},
	}

	data, err := json.Marshal(components)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var restored []Component
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if len(restored) != 3 {
		t.Fatalf("expected 3 components, got %d", len(restored))
	}

	ids := make(map[string]bool)
	for _, c := range restored {
		ids[c.ID] = true
		if c.Type != "desktop" {
			t.Errorf("component %q type = %q, want %q", c.ID, c.Type, "desktop")
		}
		if len(c.Categories) != 2 {
			t.Errorf("component %q categories count = %d, want 2", c.ID, len(c.Categories))
		}
	}

	for _, id := range []string{"dribble-Frodo.desktop", "dribble-FrodoPC.desktop", "dribble-FrodoSC.desktop"} {
		if !ids[id] {
			t.Errorf("missing component with id %q", id)
		}
	}
}
