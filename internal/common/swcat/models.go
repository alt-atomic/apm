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
	Type        string            `xml:"type,attr,omitempty"        json:"type,omitempty"`
	Environment string            `xml:"environment,attr,omitempty" json:"environment,omitempty"`
	Caption     LocalizedMap      `xml:"caption"                    json:"caption,omitempty"`
	Images      []ScreenshotImage `xml:"image"                      json:"images,omitempty"`
}

type ReleaseURL struct {
	Type  string `xml:"type,attr,omitempty" json:"type,omitempty"`
	Value string `xml:",chardata"           json:"url"`
}

type ReleaseIssue struct {
	Type  string `xml:"type,attr,omitempty" json:"type,omitempty"`
	URL   string `xml:"url,attr,omitempty"  json:"url,omitempty"`
	Value string `xml:",chardata"           json:"value"`
}

type ArtifactChecksum struct {
	Type  string `xml:"type,attr,omitempty" json:"type,omitempty"`
	Value string `xml:",chardata"           json:"value"`
}

type ArtifactSize struct {
	Type  string `xml:"type,attr,omitempty" json:"type,omitempty"`
	Value int64  `xml:",chardata"           json:"value"`
}

type ReleaseArtifact struct {
	Type      string             `xml:"type,attr,omitempty"     json:"type,omitempty"`
	Platform  string             `xml:"platform,attr,omitempty" json:"platform,omitempty"`
	Locations []string           `xml:"location"                json:"locations,omitempty"`
	Checksums []ArtifactChecksum `xml:"checksum"                json:"checksums,omitempty"`
	Sizes     []ArtifactSize     `xml:"size"                    json:"sizes,omitempty"`
	Filename  string             `xml:"filename"                json:"filename,omitempty"`
}

type Release struct {
	Version     string            `xml:"version,attr,omitempty"   json:"version,omitempty"`
	Date        string            `xml:"date,attr,omitempty"      json:"date,omitempty"`
	DateEOL     string            `xml:"date_eol,attr,omitempty"  json:"date_eol,omitempty"`
	Timestamp   int64             `xml:"timestamp,attr,omitempty" json:"timestamp,omitempty"`
	Type        string            `xml:"type,attr,omitempty"      json:"type,omitempty"`
	Urgency     string            `xml:"urgency,attr,omitempty"   json:"urgency,omitempty"`
	Description LocalizedMap      `xml:"description"              json:"description,omitempty"`
	URL         *ReleaseURL       `xml:"url,omitempty"            json:"url,omitempty"`
	Issues      []ReleaseIssue    `xml:"issues>issue"             json:"issues,omitempty"`
	Artifacts   []ReleaseArtifact `xml:"artifacts>artifact"       json:"artifacts,omitempty"`
}

type Icon struct {
	Type   string `xml:"type,attr"             json:"type"`
	Width  int    `xml:"width,attr,omitempty"  json:"width,omitempty"`
	Height int    `xml:"height,attr,omitempty" json:"height,omitempty"`
	Value  string `xml:",chardata"             json:"value"`
}

type Developer struct {
	ID   string       `xml:"id,attr,omitempty" json:"id,omitempty"`
	Name LocalizedMap `xml:"name"              json:"name,omitempty"`
	URL  string       `xml:"url"               json:"url,omitempty"`
}

type DBusService struct {
	Type  string `xml:"type,attr" json:"type"`
	Value string `xml:",chardata" json:"value"`
}

type Provides struct {
	Binaries   []string      `xml:"binary"    json:"binaries,omitempty"`
	Fonts      []string      `xml:"font"      json:"fonts,omitempty"`
	Modalias   []string      `xml:"modalias"  json:"modalias,omitempty"`
	Mediatypes []string      `xml:"mediatype" json:"mediatypes,omitempty"`
	IDs        []string      `xml:"id"        json:"ids,omitempty"`
	DBus       []DBusService `xml:"dbus"      json:"dbus,omitempty"`
	Libraries  []string      `xml:"library"   json:"libraries,omitempty"`
}

type BrandingColor struct {
	Type             string `xml:"type,attr"                        json:"type"`
	SchemePreference string `xml:"scheme_preference,attr,omitempty" json:"scheme_preference,omitempty"`
	Value            string `xml:",chardata"                        json:"value"`
}

type Translation struct {
	Type  string `xml:"type,attr" json:"type"`
	Value string `xml:",chardata" json:"value"`
}

type Language struct {
	Percentage int    `xml:"percentage,attr,omitempty" json:"percentage,omitempty"`
	Lang       string `xml:",chardata"                 json:"lang"`
}

type CustomValue struct {
	Key   string `xml:"key,attr"`
	Value string `xml:",chardata"`
}

type CustomMap []CustomValue

func (m *CustomMap) MarshalJSON() ([]byte, error) {
	return marshalDict(*m, func(cv CustomValue) (string, string) { return cv.Key, cv.Value })
}

func (m *CustomMap) UnmarshalJSON(data []byte) error {
	return unmarshalDict(data, (*[]CustomValue)(m), func(key, value string) CustomValue {
		return CustomValue{Key: key, Value: value}
	})
}

type DisplayLength struct {
	Compare string `xml:"compare,attr,omitempty" json:"compare,omitempty"`
	Side    string `xml:"side,attr,omitempty"    json:"side,omitempty"`
	Value   string `xml:",chardata"              json:"value"`
}

type Relation struct {
	Controls      []string        `xml:"control"        json:"controls,omitempty"`
	DisplayLength []DisplayLength `xml:"display_length" json:"display_length,omitempty"`
	Internet      string          `xml:"internet"       json:"internet,omitempty"`
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

	DeveloperName LocalizedMap `xml:"developer_name" json:"developer_name,omitempty"`
	Developer     *Developer   `xml:"developer"      json:"developer,omitempty"`
	ProjectGroup  string       `xml:"project_group"  json:"project_group,omitempty"`

	Keywords    KeywordList  `xml:"keywords>keyword"       json:"keywords,omitempty"`
	Categories  []string     `xml:"categories>category"    json:"categories,omitempty"`
	Urls        URLMap       `xml:"url"                    json:"urls,omitempty"`
	Screenshots []Screenshot `xml:"screenshots>screenshot" json:"screenshots,omitempty"`
	Releases    []Release    `xml:"releases>release"       json:"releases,omitempty"`
	Icons       []Icon       `xml:"icon"                   json:"icons,omitempty"`

	Launchable    *Launchable    `xml:"launchable,omitempty"     json:"launchable,omitempty"`
	ContentRating *ContentRating `xml:"content_rating,omitempty" json:"content_rating,omitempty"`
	Provides      *Provides      `xml:"provides,omitempty"       json:"provides,omitempty"`

	Extends              []string        `xml:"extends"              json:"extends,omitempty"`
	Branding             []BrandingColor `xml:"branding>color"       json:"branding,omitempty"`
	Translations         []Translation   `xml:"translation"          json:"translations,omitempty"`
	Languages            []Language      `xml:"languages>lang"       json:"languages,omitempty"`
	Custom               CustomMap       `xml:"custom>value"         json:"custom,omitempty"`
	Kudos                []string        `xml:"kudos>kudo"           json:"kudos,omitempty"`
	Mimetypes            []string        `xml:"mimetypes>mimetype"   json:"mimetypes,omitempty"`
	Requires             *Relation       `xml:"requires,omitempty"   json:"requires,omitempty"`
	Recommends           *Relation       `xml:"recommends,omitempty" json:"recommends,omitempty"`
	Supports             *Relation       `xml:"supports,omitempty"   json:"supports,omitempty"`
	Suggests             []string        `xml:"suggests>id"          json:"suggests,omitempty"`
	Replaces             []string        `xml:"replaces>id"          json:"replaces,omitempty"`
	CompulsoryForDesktop []string        `xml:"compulsory_for_desktop" json:"compulsory_for_desktop,omitempty"`
	UpdateContact        string          `xml:"update_contact"       json:"update_contact,omitempty"`

	PkgName string `xml:"pkgname" json:"pkgname"`

	// fallback old fields
	LegacyUpdateContact  string `xml:"updatecontact"    json:"-"`
	LegacyXUpdateContact string `xml:"x-updatecontact"  json:"-"`
	LegacyLicence        string `xml:"licence"          json:"-"`
	LegacyLicense        string `xml:"license"          json:"-"`
	LegacyMetaLicence    string `xml:"metadata_licence" json:"-"`
	LegacyName           string `xml:"_name"            json:"-"`
	LegacySummary        string `xml:"_summary"         json:"-"`
	LegacyLaunch         *Launchable `xml:"launch"       json:"-"`
}

type SWCatalog struct {
	XMLName    xml.Name    `xml:"components"`
	Components []Component `xml:"component"`
}
