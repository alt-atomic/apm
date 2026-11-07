package metainfo

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type OperatingSystem struct {
	ID          string
	Name        map[string]string
	Summary     map[string]string
	Description map[string]string
	Variants    []Variant
	Homepage    string
	Developer   *Developer
	Release     *Release
}

type Developer struct {
	ID   string
	Name string
}

type Variant struct {
	Image       string            `yaml:"image" json:"image"`
	Name        map[string]string `yaml:"name,omitempty" json:"name,omitempty"`
	Summary     map[string]string `yaml:"summary,omitempty" json:"summary,omitempty"`
	Description map[string]string `yaml:"description,omitempty" json:"description,omitempty"`
}

type Release struct {
	Version     string            `yaml:"version,omitempty" json:"version,omitempty"`
	Date        string            `yaml:"date,omitempty" json:"date,omitempty"`
	Description map[string]string `yaml:"description,omitempty" json:"description,omitempty"`
}

func ReverseDNS(home, suffix string) string {
	if _, after, ok := strings.Cut(home, "://"); ok {
		home = after
	}
	home, _, _ = strings.Cut(home, "/")
	parts := strings.Split(home, ".")
	result := suffix
	for _, part := range parts {
		if part != "www" {
			result = strings.ReplaceAll(part, "-", "_") + "." + result
		}
	}
	if suffix == "" {
		result = strings.TrimSuffix(result, ".")
	}
	return result
}

type xmlText struct {
	Lang string `xml:"xml:lang,attr,omitempty"`
	Text string `xml:",chardata"`
}

type xmlDescription struct {
	Markup string `xml:",innerxml"`
}

type xmlVariant struct {
	Image       string          `xml:"image"`
	Name        []xmlText       `xml:"name"`
	Summary     []xmlText       `xml:"summary"`
	Description *xmlDescription `xml:"description"`
}

type xmlRelease struct {
	Version     string          `xml:"version,attr"`
	Date        string          `xml:"date,attr,omitempty"`
	Description *xmlDescription `xml:"description"`
}

type xmlComponent struct {
	XMLName     xml.Name        `xml:"component"`
	Type        string          `xml:"type,attr"`
	ID          string          `xml:"id"`
	Name        []xmlText       `xml:"name"`
	Summary     []xmlText       `xml:"summary"`
	Description *xmlDescription `xml:"description"`
	Variants    *struct {
		Items []xmlVariant `xml:"variant"`
	} `xml:"x-variants,omitempty"`
	Homepage *struct {
		Type string `xml:"type,attr"`
		URL  string `xml:",chardata"`
	} `xml:"url,omitempty"`
	License   string `xml:"metadata_license"`
	Developer *struct {
		ID   string `xml:"id,attr,omitempty"`
		Name string `xml:"name"`
	} `xml:"developer,omitempty"`
	Releases []xmlRelease `xml:"releases>release"`
}

func localizedTexts(values map[string]string) []xmlText {
	var result []xmlText
	for _, key := range slices.Sorted(maps.Keys(values)) {
		lang := key
		if lang == "C" {
			lang = ""
		}
		result = append(result, xmlText{Lang: lang, Text: values[key]})
	}
	return result
}

func descriptions(values map[string]string) (*xmlDescription, error) {
	if len(values) == 0 {
		return nil, nil
	}
	var markup bytes.Buffer
	encoder := xml.NewEncoder(&markup)
	for _, text := range localizedTexts(values) {
		// Validate the complete wrapper so a fragment cannot escape its description.
		wrapped := "<description>" + text.Text + "</description>"
		decoder := xml.NewDecoder(strings.NewReader(wrapped))
		depth := 0
		for {
			token, err := decoder.Token()
			if err != nil {
				return nil, fmt.Errorf("invalid metainfo description: %w", err)
			}
			switch element := token.(type) {
			case xml.StartElement:
				depth++
				if depth == 1 {
					continue
				}
				// MetaInfo translations belong on paragraphs and list items, rather
				// than on the description wrapper used by catalog XML.
				if text.Lang != "" && (element.Name.Local == "p" || element.Name.Local == "li") {
					element.Attr = append(element.Attr, xml.Attr{Name: xml.Name{Local: "xml:lang"}, Value: text.Lang})
					token = element
				}
			case xml.EndElement:
				depth--
			}
			if depth == 0 {
				if _, err := decoder.Token(); err != io.EOF {
					return nil, fmt.Errorf("invalid metainfo description fragment")
				}
				break
			}
			if err := encoder.EncodeToken(token); err != nil {
				return nil, err
			}
		}
	}
	if err := encoder.Flush(); err != nil {
		return nil, err
	}
	return &xmlDescription{Markup: markup.String()}, nil
}

func RenderOperatingSystem(component OperatingSystem) ([]byte, error) {
	id := component.ID
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, "/\\\x00") {
		return nil, fmt.Errorf("invalid operating-system component ID %q", id)
	}
	doc := xmlComponent{Type: "operating-system", ID: id, License: "FSFAP", Name: localizedTexts(component.Name), Summary: localizedTexts(component.Summary)}
	var err error
	doc.Description, err = descriptions(component.Description)
	if err != nil {
		return nil, err
	}
	if home := component.Homepage; home != "" {
		doc.Homepage = &struct {
			Type string `xml:"type,attr"`
			URL  string `xml:",chardata"`
		}{"homepage", home}
	}
	if developer := component.Developer; developer != nil {
		doc.Developer = &struct {
			ID   string `xml:"id,attr,omitempty"`
			Name string `xml:"name"`
		}{developer.ID, developer.Name}
	}
	for _, variant := range component.Variants {
		description, err := descriptions(variant.Description)
		if err != nil {
			return nil, err
		}
		if doc.Variants == nil {
			doc.Variants = &struct {
				Items []xmlVariant `xml:"variant"`
			}{}
		}
		doc.Variants.Items = append(doc.Variants.Items, xmlVariant{Image: variant.Image, Name: localizedTexts(variant.Name), Summary: localizedTexts(variant.Summary), Description: description})
	}
	release := Release{}
	if component.Release != nil {
		release = *component.Release
	}
	if release.Version != "" {
		description, err := descriptions(release.Description)
		if err != nil {
			return nil, err
		}
		doc.Releases = []xmlRelease{{Version: release.Version, Date: release.Date, Description: description}}
	}
	data, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), append(data, '\n')...), nil
}

func WriteOperatingSystem(root string, component OperatingSystem) error {
	data, err := RenderOperatingSystem(component)
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "usr/share/metainfo")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, component.ID+".metainfo.xml"), data, 0644)
}
