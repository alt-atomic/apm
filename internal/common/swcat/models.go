package swcat

import (
	"encoding/json"
	"encoding/xml"
)

func marshalDict[T any](items []T, toKV func(T) (string, string)) ([]byte, error) {
	dict := make(map[string]string, len(items))
	for _, item := range items {
		k, v := toKV(item)
		if _, exists := dict[k]; !exists {
			dict[k] = v
		}
	}
	return json.Marshal(dict)
}

func unmarshalDict[T any](data []byte, dst *[]T, fromKV func(string, string) T) error {
	var dict map[string]string
	if err := json.Unmarshal(data, &dict); err != nil {
		return err
	}
	*dst = make([]T, 0, len(dict))
	for k, v := range dict {
		*dst = append(*dst, fromKV(k, v))
	}
	return nil
}

type LocalizedText struct {
	Lang  string `xml:"lang,attr,omitempty"`
	Value string `xml:",innerxml"`
}

type Keyword struct {
	Value string `xml:",chardata"`
}

type URL struct {
	Type  string `xml:"type,attr,omitempty"`
	Value string `xml:",chardata"`
}

type ContentAttribute struct {
	ID    string `xml:"id,attr"`
	Value string `xml:",chardata"`
}

type LocalizedMap []LocalizedText

func (m *LocalizedMap) MarshalJSON() ([]byte, error) {
	return marshalDict(*m, func(t LocalizedText) (string, string) {
		if t.Lang == "" {
			return "C", t.Value
		}
		return t.Lang, t.Value
	})
}

func (m *LocalizedMap) UnmarshalJSON(data []byte) error {
	return unmarshalDict(data, (*[]LocalizedText)(m), func(key, value string) LocalizedText {
		lang := key
		if lang == "C" {
			lang = ""
		}
		return LocalizedText{Lang: lang, Value: value}
	})
}

type KeywordList []Keyword

func (m *KeywordList) MarshalJSON() ([]byte, error) {
	keywords := make([]string, len(*m))
	for i, keyword := range *m {
		keywords[i] = keyword.Value
	}
	return json.Marshal(keywords)
}

func (m *KeywordList) UnmarshalJSON(data []byte) error {
	var values []string
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	*m = make(KeywordList, len(values))
	for i, value := range values {
		(*m)[i] = Keyword{Value: value}
	}
	return nil
}

type URLMap []URL

func (m *URLMap) MarshalJSON() ([]byte, error) {
	return marshalDict(*m, func(u URL) (string, string) { return u.Type, u.Value })
}

func (m *URLMap) UnmarshalJSON(data []byte) error {
	return unmarshalDict(data, (*[]URL)(m), func(urlType, url string) URL {
		return URL{Type: urlType, Value: url}
	})
}

type ContentAttributeMap []ContentAttribute

func (m *ContentAttributeMap) MarshalJSON() ([]byte, error) {
	return marshalDict(*m, func(a ContentAttribute) (string, string) { return a.ID, a.Value })
}

func (m *ContentAttributeMap) UnmarshalJSON(data []byte) error {
	return unmarshalDict(data, (*[]ContentAttribute)(m), func(id, value string) ContentAttribute {
		return ContentAttribute{ID: id, Value: value}
	})
}

type ScreenshotImage struct {
	Type   string `xml:"type,attr,omitempty"   json:"type,omitempty"`
	Width  int    `xml:"width,attr,omitempty"  json:"width,omitempty"`
	Height int    `xml:"height,attr,omitempty" json:"height,omitempty"`
	URL    string `xml:",chardata"             json:"url"`
}

type Screenshot struct {
	Type    string            `xml:"type,attr,omitempty" json:"type,omitempty"`
	Caption LocalizedMap      `xml:"caption"             json:"caption,omitempty"`
	Images  []ScreenshotImage `xml:"image"               json:"images,omitempty"`
}

type Release struct {
	Timestamp int64  `xml:"timestamp,attr" json:"timestamp"`
	Version   string `xml:"version,attr"   json:"version"`
}

type Icon struct {
	Type   string `xml:"type,attr"             json:"type"`
	Width  int    `xml:"width,attr,omitempty"  json:"width,omitempty"`
	Height int    `xml:"height,attr,omitempty" json:"height,omitempty"`
	Value  string `xml:",chardata"             json:"value"`
}

type Launchable struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

func (l *Launchable) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{l.Type: l.Value})
}

func (l *Launchable) UnmarshalJSON(data []byte) error {
	var dict map[string]string
	if err := json.Unmarshal(data, &dict); err != nil {
		return err
	}
	for k, v := range dict {
		l.Type = k
		l.Value = v
		break
	}
	return nil
}

type ContentRating struct {
	Type       string              `xml:"type,attr,omitempty"  json:"type,omitempty"`
	Attributes ContentAttributeMap `xml:"content_attribute"    json:"attributes,omitempty"`
}

type Component struct {
	XMLName         xml.Name `xml:"component"                 json:"-"`
	Type            string   `xml:"type,attr"                 json:"type"`
	ID              string   `xml:"id"                        json:"id,omitempty"`
	MetadataLicense string   `xml:"metadata_license"          json:"metadata_license,omitempty"`
	ProjectLicense  string   `xml:"project_license,omitempty" json:"project_license,omitempty"`

	Name        LocalizedMap `xml:"name"        json:"name,omitempty"`
	Summary     LocalizedMap `xml:"summary"     json:"summary,omitempty"`
	Description LocalizedMap `xml:"description" json:"description,omitempty"`

	Keywords    KeywordList  `xml:"keywords>keyword"       json:"keywords,omitempty"`
	Categories  []string     `xml:"categories>category"    json:"categories,omitempty"`
	Urls        URLMap       `xml:"url"                    json:"urls,omitempty"`
	Screenshots []Screenshot `xml:"screenshots>screenshot" json:"screenshots,omitempty"`
	Releases    []Release    `xml:"releases>release"       json:"releases,omitempty"`
	Icons       []Icon       `xml:"icon"                   json:"icons,omitempty"`

	Launchable    *Launchable    `xml:"launchable,omitempty"     json:"launchable,omitempty"`
	ContentRating *ContentRating `xml:"content_rating,omitempty" json:"content_rating,omitempty"`

	PkgName string `xml:"pkgname" json:"pkgname"`
}

type SWCatalog struct {
	XMLName    xml.Name    `xml:"components"`
	Components []Component `xml:"component"`
}
