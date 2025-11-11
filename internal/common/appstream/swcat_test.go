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

package appstream

import (
	"reflect"
	"strings"
	"testing"
)

func TestCleanHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple HTML tags",
			input:    "<p>Hello <b>world</b></p>",
			expected: "Hello world",
		},
		{
			name:     "Complex nested tags",
			input:    "<div><h1>Title</h1><p>Paragraph with <em>emphasis</em> and <strong>strong</strong> text.</p></div>",
			expected: "TitleParagraph with emphasis and strong text.",
		},
		{
			name:     "Self-closing tags",
			input:    "Line 1<br/>Line 2<hr/>Line 3",
			expected: "Line 1Line 2Line 3",
		},
		{
			name:     "HTML entities",
			input:    "Caf&eacute; &amp; R&eacute;sum&eacute;",
			expected: "Café & Résumé",
		},
		{
			name:     "Mixed HTML entities and tags",
			input:    "<p>Price: &pound;100 &lt; &pound;200</p>",
			expected: "Price: £100",
		},
		{
			name:     "No HTML content",
			input:    "Plain text without any HTML",
			expected: "Plain text without any HTML",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Only whitespace",
			input:    "   \n\t   ",
			expected: "",
		},
		{
			name:     "Tags with attributes",
			input:    `<a href="https://example.com" class="link">Click here</a>`,
			expected: "Click here",
		},
		{
			name:     "Malformed HTML",
			input:    "<p>Unclosed paragraph<div>Another tag</p>",
			expected: "Unclosed paragraphAnother tag",
		},
		{
			name:     "Multiple consecutive spaces",
			input:    "<p>Text    with     multiple     spaces</p>",
			expected: "Text with multiple spaces",
		},
		{
			name:     "Mixed whitespace characters",
			input:    "<div>Text\n\twith\r\nmixed\t\twhitespace</div>",
			expected: "Text with mixed whitespace",
		},
		{
			name:     "HTML comments",
			input:    "<!-- This is a comment --><p>Visible text</p><!-- Another comment -->",
			expected: "Visible text",
		},
		{
			name:     "Script and style tags",
			input:    "<script>var x = 1;</script><p>Content</p><style>body { color: red; }</style>",
			expected: "var x = 1;Contentbody { color: red; }",
		},
		{
			name:     "Multiline HTML",
			input:    "<div>\n  <h1>Title</h1>\n  <p>Paragraph</p>\n</div>",
			expected: "Title Paragraph",
		},
		{
			name:     "HTML with special characters",
			input:    "<p>Symbols: &copy; &trade; &reg;</p>",
			expected: "Symbols: © ™ ®",
		},
		{
			name:     "Nested quotes in attributes",
			input:    `<div title="He said 'Hello'">Content</div>`,
			expected: "Content",
		},
		{
			name:     "Tags with line breaks",
			input:    "<p>First\nline</p><p>Second\nline</p>",
			expected: "First lineSecond line",
		},
		{
			name:     "Only HTML tags",
			input:    "<div><span></span></div>",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanHTML(tt.input)
			if result != tt.expected {
				t.Errorf("cleanHTML(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDedupTexts(t *testing.T) {
	tests := []struct {
		name     string
		input    []LocalizedText
		expected []LocalizedText
	}{
		{
			name: "No duplicates",
			input: []LocalizedText{
				{Lang: "en", Value: "Hello"},
				{Lang: "ru", Value: "Привет"},
				{Lang: "fr", Value: "Bonjour"},
			},
			expected: []LocalizedText{
				{Lang: "en", Value: "Hello"},
				{Lang: "ru", Value: "Привет"},
				{Lang: "fr", Value: "Bonjour"},
			},
		},
		{
			name: "Exact duplicates",
			input: []LocalizedText{
				{Lang: "en", Value: "Hello"},
				{Lang: "en", Value: "Hello"},
				{Lang: "ru", Value: "Привет"},
			},
			expected: []LocalizedText{
				{Lang: "en", Value: "Hello"},
				{Lang: "ru", Value: "Привет"},
			},
		},
		{
			name: "Same value different language",
			input: []LocalizedText{
				{Lang: "en", Value: "OK"},
				{Lang: "fr", Value: "OK"},
				{Lang: "de", Value: "OK"},
			},
			expected: []LocalizedText{
				{Lang: "en", Value: "OK"},
				{Lang: "fr", Value: "OK"},
				{Lang: "de", Value: "OK"},
			},
		},
		{
			name: "Same language different value",
			input: []LocalizedText{
				{Lang: "en", Value: "Hello"},
				{Lang: "en", Value: "Hi"},
				{Lang: "en", Value: "Hey"},
			},
			expected: []LocalizedText{
				{Lang: "en", Value: "Hello"},
				{Lang: "en", Value: "Hi"},
				{Lang: "en", Value: "Hey"},
			},
		},
		{
			name: "Empty language",
			input: []LocalizedText{
				{Lang: "", Value: "Default"},
				{Lang: "en", Value: "English"},
				{Lang: "", Value: "Default"},
			},
			expected: []LocalizedText{
				{Lang: "", Value: "Default"},
				{Lang: "en", Value: "English"},
			},
		},
		{
			name: "Empty value",
			input: []LocalizedText{
				{Lang: "en", Value: ""},
				{Lang: "ru", Value: "Text"},
				{Lang: "en", Value: ""},
			},
			expected: []LocalizedText{
				{Lang: "en", Value: ""},
				{Lang: "ru", Value: "Text"},
			},
		},
		{
			name:     "Empty slice",
			input:    []LocalizedText{},
			expected: []LocalizedText{},
		},
		{
			name: "Single element",
			input: []LocalizedText{
				{Lang: "en", Value: "Single"},
			},
			expected: []LocalizedText{
				{Lang: "en", Value: "Single"},
			},
		},
		{
			name: "Complex duplicates pattern",
			input: []LocalizedText{
				{Lang: "en", Value: "App"},
				{Lang: "ru", Value: "Приложение"},
				{Lang: "en", Value: "App"},
				{Lang: "fr", Value: "Application"},
				{Lang: "ru", Value: "Приложение"},
				{Lang: "en", Value: "Application"},
				{Lang: "fr", Value: "Application"},
			},
			expected: []LocalizedText{
				{Lang: "en", Value: "App"},
				{Lang: "ru", Value: "Приложение"},
				{Lang: "fr", Value: "Application"},
				{Lang: "en", Value: "Application"},
			},
		},
		{
			name: "Whitespace variations",
			input: []LocalizedText{
				{Lang: "en", Value: "Text"},
				{Lang: "en", Value: " Text"},
				{Lang: "en", Value: "Text "},
				{Lang: "en", Value: " Text "},
			},
			expected: []LocalizedText{
				{Lang: "en", Value: "Text"},
				{Lang: "en", Value: " Text"},
				{Lang: "en", Value: "Text "},
				{Lang: "en", Value: " Text "},
			},
		},
		{
			name: "Special characters in values",
			input: []LocalizedText{
				{Lang: "en", Value: "Hello\x00World"},
				{Lang: "en", Value: "Hello\x00World"},
				{Lang: "ru", Value: "Тест\x00значение"},
			},
			expected: []LocalizedText{
				{Lang: "en", Value: "Hello\x00World"},
				{Lang: "ru", Value: "Тест\x00значение"},
			},
		},
		{
			name: "Regional language codes",
			input: []LocalizedText{
				{Lang: "en_US", Value: "Color"},
				{Lang: "en_GB", Value: "Colour"},
				{Lang: "en_US", Value: "Color"},
				{Lang: "pt_BR", Value: "Português brasileiro"},
			},
			expected: []LocalizedText{
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

// Дополнительный тест для проверки порядка элементов в dedupTexts
func TestDedupTextsOrder(t *testing.T) {
	input := []LocalizedText{
		{Lang: "en", Value: "First"},
		{Lang: "ru", Value: "Второй"},
		{Lang: "en", Value: "First"},
		{Lang: "fr", Value: "Troisième"},
		{Lang: "ru", Value: "Второй"},
	}

	result := dedupTexts(input)

	expected := []LocalizedText{
		{Lang: "en", Value: "First"},
		{Lang: "ru", Value: "Второй"},
		{Lang: "fr", Value: "Troisième"},
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("dedupTexts() order test failed.\nGot:      %+v\nExpected: %+v", result, expected)
	}
}

// Тест производительности для больших объемов данных
func TestDedupTextsPerformance(t *testing.T) {
	// Создаем большой слайс с дубликатами
	large := make([]LocalizedText, 1000)
	for i := 0; i < 1000; i++ {
		large[i] = LocalizedText{
			Lang:  "en",
			Value: "Text",
		}
	}

	result := dedupTexts(large)

	if len(result) != 1 {
		t.Errorf("dedupTexts() performance test failed, expected 1 item, got %d", len(result))
	}

	if result[0].Lang != "en" || result[0].Value != "Text" {
		t.Errorf("dedupTexts() performance test failed, wrong content: %+v", result[0])
	}
}

// Тест граничных случаев
func TestCleanHTMLEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Very long string",
			input:    "<p>" + strings.Repeat("a", 10000) + "</p>",
			expected: strings.Repeat("a", 10000),
		},
		{
			name:     "Deeply nested tags",
			input:    "<a><b><c><d><e><f>Content</f></e></d></c></b></a>",
			expected: "Content",
		},
		{
			name:     "Unclosed tag at end",
			input:    "Content<p",
			expected: "Content<p",
		},
		{
			name:     "Tag in middle of word",
			input:    "Hel<b>lo Wor</b>ld",
			expected: "Hello World",
		},
		{
			name:     "Escaped HTML in text",
			input:    "Code: &lt;script&gt;alert('hi')&lt;/script&gt;",
			expected: "Code: alert('hi')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanHTML(tt.input)
			if result != tt.expected {
				t.Errorf("cleanHTML(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
