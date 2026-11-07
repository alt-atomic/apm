package modules

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// BrandingMetainfo describes the optional fields of an operating-system component.
type BrandingMetainfo struct {
	Name          map[string]string `yaml:"name,omitempty" json:"name,omitempty"`
	Summary       map[string]string `yaml:"summary,omitempty" json:"summary,omitempty"`
	Description   map[string]string `yaml:"description,omitempty" json:"description,omitempty"`
	Variants      []BrandingVariant `yaml:"x-variants,omitempty" json:"x-variants,omitempty"`
	LatestRelease *BrandingRelease  `yaml:"latest-release,omitempty" json:"latest-release,omitempty"`
}

type BrandingVariant struct {
	Image       string            `yaml:"image" json:"image"`
	Name        map[string]string `yaml:"name,omitempty" json:"name,omitempty"`
	Summary     map[string]string `yaml:"summary,omitempty" json:"summary,omitempty"`
	Description map[string]string `yaml:"description,omitempty" json:"description,omitempty"`
}

type BrandingRelease struct {
	Version     string            `yaml:"version,omitempty" json:"version,omitempty"`
	Date        string            `yaml:"date,omitempty" json:"date,omitempty"`
	Description map[string]string `yaml:"description,omitempty" json:"description,omitempty"`
}

func brandingReverseDNS(home, suffix string) string {
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

type brandingXMLText struct {
	Lang string `xml:"xml:lang,attr,omitempty"`
	Text string `xml:",chardata"`
}

type brandingXMLDescription struct {
	Markup string `xml:",innerxml"`
}

type brandingXMLVariant struct {
	Image       string                  `xml:"image"`
	Name        []brandingXMLText       `xml:"name"`
	Summary     []brandingXMLText       `xml:"summary"`
	Description *brandingXMLDescription `xml:"description"`
}

type brandingXMLRelease struct {
	Version     string                  `xml:"version,attr"`
	Date        string                  `xml:"date,attr,omitempty"`
	Description *brandingXMLDescription `xml:"description"`
}

type brandingXMLComponent struct {
	XMLName     xml.Name                `xml:"component"`
	Type        string                  `xml:"type,attr"`
	ID          string                  `xml:"id"`
	Name        []brandingXMLText       `xml:"name"`
	Summary     []brandingXMLText       `xml:"summary"`
	Description *brandingXMLDescription `xml:"description"`
	Variants    *struct {
		Items []brandingXMLVariant `xml:"variant"`
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
	Releases []brandingXMLRelease `xml:"releases>release"`
}

func brandingTexts(values map[string]string, fallback string) []brandingXMLText {
	keys := make([]string, 0, len(values)+1)
	for key := range values {
		keys = append(keys, key)
	}
	if _, ok := values["C"]; !ok && fallback != "" {
		keys = append(keys, "C")
	}
	slices.Sort(keys)
	var result []brandingXMLText
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			value = fallback
		}
		lang := key
		if lang == "C" {
			lang = ""
		}
		result = append(result, brandingXMLText{Lang: lang, Text: value})
	}
	return result
}

func brandingDescriptions(values map[string]string) (*brandingXMLDescription, error) {
	if len(values) == 0 {
		return nil, nil
	}
	var markup bytes.Buffer
	for _, text := range brandingTexts(values, "") {
		// Validate the complete wrapper so a fragment cannot escape its description.
		wrapped := "<description>" + text.Text + "</description>"
		decoder := xml.NewDecoder(strings.NewReader(wrapped))
		var fragment struct{ XMLName xml.Name }
		if err := decoder.Decode(&fragment); err != nil {
			return nil, fmt.Errorf("invalid metainfo description: %w", err)
		}
		if _, err := decoder.Token(); err != io.EOF {
			return nil, fmt.Errorf("invalid metainfo description fragment")
		}
		decoder = xml.NewDecoder(strings.NewReader(wrapped))
		encoder := xml.NewEncoder(&markup)
		depth := 0
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
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
				if depth == 0 {
					continue
				}
			}
			if depth > 0 {
				if err := encoder.EncodeToken(token); err != nil {
					return nil, err
				}
			}
		}
		if err := encoder.Flush(); err != nil {
			return nil, err
		}
	}
	return &brandingXMLDescription{Markup: markup.String()}, nil
}

func renderBrandingMetainfo(id string, vars map[string]string, cfg *BrandingMetainfo) ([]byte, error) {
	if cfg == nil {
		cfg = &BrandingMetainfo{}
	}
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, "/\\\x00") {
		return nil, fmt.Errorf("invalid operating-system component ID %q", id)
	}
	doc := brandingXMLComponent{Type: "operating-system", ID: id, License: "FSFAP", Name: brandingTexts(cfg.Name, vars["NAME"]), Summary: brandingTexts(cfg.Summary, vars["NAME"])}
	var err error
	doc.Description, err = brandingDescriptions(cfg.Description)
	if err != nil {
		return nil, err
	}
	if home := vars["HOME_URL"]; home != "" {
		doc.Homepage = &struct {
			Type string `xml:"type,attr"`
			URL  string `xml:",chardata"`
		}{"homepage", home}
	}
	if name := vars["VENDOR_NAME"]; name != "" {
		vendorID := ""
		if home := vars["VENDOR_URL"]; home != "" {
			vendorID = brandingReverseDNS(home, "")
		}
		doc.Developer = &struct {
			ID   string `xml:"id,attr,omitempty"`
			Name string `xml:"name"`
		}{vendorID, name}
	}
	for _, variant := range cfg.Variants {
		description, err := brandingDescriptions(variant.Description)
		if err != nil {
			return nil, err
		}
		if doc.Variants == nil {
			doc.Variants = &struct {
				Items []brandingXMLVariant `xml:"variant"`
			}{}
		}
		doc.Variants.Items = append(doc.Variants.Items, brandingXMLVariant{Image: variant.Image, Name: brandingTexts(variant.Name, ""), Summary: brandingTexts(variant.Summary, ""), Description: description})
	}
	release := BrandingRelease{}
	if cfg.LatestRelease != nil {
		release = *cfg.LatestRelease
	}
	if release.Version == "" {
		release.Version = vars["VERSION_ID"]
	}
	if release.Version != "" {
		description, err := brandingDescriptions(release.Description)
		if err != nil {
			return nil, err
		}
		doc.Releases = []brandingXMLRelease{{Version: release.Version, Date: release.Date, Description: description}}
	}
	data, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), append(data, '\n')...), nil
}

func saveBrandingMetainfo(root, id string, found bool, vars map[string]string, cfg *BrandingMetainfo) error {
	if found {
		if cfg != nil {
			return fmt.Errorf("operating-system component %q already exists; branding.metainfo must be omitted", id)
		}
		return nil
	}

	data, err := renderBrandingMetainfo(id, vars, cfg)
	if err != nil {
		return err
	}

	dir := filepath.Join(root, "usr/share/metainfo")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, id+".metainfo.xml"), data, 0644)
}
