package metainfo

import "testing"

func TestDescriptionsRejectMalformedFragments(t *testing.T) {
	for _, fragment := range []string{
		"<p>unclosed",
		"<p>mismatched</li>",
		"<p>bad & entity</p>",
		"</description><description>",
		"</description><id>injected</id><description>",
		"</description><!-- trailing -->",
		"</description> ",
	} {
		t.Run(fragment, func(t *testing.T) {
			got, err := descriptions(map[string]string{"C": "<p>Valid</p>", "ru": fragment})
			if err == nil || got != nil {
				t.Fatalf("expected rejection without partial output, got %v, %v", got, err)
			}
		})
	}
}

func TestDescriptionsMultipleTranslations(t *testing.T) {
	got, err := descriptions(map[string]string{
		"ru": "<p>Описание</p><ul><li>Пункт</li></ul>",
		"C":  "<p>A &amp; B</p><ul><li>Item</li></ul>",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `<p>A &amp; B</p><ul><li>Item</li></ul><p xml:lang="ru">Описание</p><ul><li xml:lang="ru">Пункт</li></ul>`
	if got == nil || got.Markup != want {
		t.Fatalf("unexpected descriptions: %+v", got)
	}
}

func TestDescriptionsEmpty(t *testing.T) {
	got, err := descriptions(nil)
	if err != nil || got != nil {
		t.Fatalf("expected absent description, got %v, %v", got, err)
	}
	got, err = descriptions(map[string]string{"C": ""})
	if err != nil || got == nil || got.Markup != "" {
		t.Fatalf("expected empty description, got %v, %v", got, err)
	}
}
