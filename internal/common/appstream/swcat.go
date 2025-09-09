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
	"apm/internal/common/app"
	"apm/internal/common/reply"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type LocalizedText struct {
	Lang  string `xml:"lang,attr,omitempty" json:"lang,omitempty"`
	Value string `xml:",innerxml"           json:"value"`
}

type URL struct {
	Type  string `xml:"type,attr,omitempty" json:"type,omitempty"`
	Value string `xml:",chardata"           json:"value"`
}

type Keyword struct {
	Value string `xml:",chardata" json:"value"`
}

type ScreenshotImage struct {
	Type   string `xml:"type,attr,omitempty"   json:"type,omitempty"`
	Width  int    `xml:"width,attr,omitempty"  json:"width,omitempty"`
	Height int    `xml:"height,attr,omitempty" json:"height,omitempty"`
	URL    string `xml:",chardata"             json:"url"`
}

type Screenshot struct {
	Type    string            `xml:"type,attr,omitempty" json:"type,omitempty"`
	Caption []LocalizedText   `xml:"caption"             json:"caption,omitempty"`
	Images  []ScreenshotImage `xml:"image"               json:"images,omitempty"`
}

type Release struct {
	Timestamp int64  `xml:"timestamp,attr" json:"timestamp"`
	Version   string `xml:"version,attr"   json:"version"`
}

type Launchable struct {
	Type  string `xml:"type,attr" json:"type,omitempty"`
	Value string `xml:",chardata" json:"value"`
}

type ContentRating struct {
	Type    string `xml:"type,attr,omitempty" json:"type,omitempty"`
	Content string `xml:",innerxml"           json:"content"`
}

type Icon struct {
	Type   string `xml:"type,attr"             json:"type"`
	Width  int    `xml:"width,attr,omitempty"  json:"width,omitempty"`
	Height int    `xml:"height,attr,omitempty" json:"height,omitempty"`
	Value  string `xml:",chardata"             json:"value"`
}

type Component struct {
	XMLName         xml.Name `xml:"component"                 json:"-"`
	Type            string   `xml:"type,attr"                 json:"type"`
	ID              string   `xml:"id"                        json:"id,omitempty"`
	MetadataLicense string   `xml:"metadata_license"          json:"metadata_license,omitempty"`
	ProjectLicense  string   `xml:"project_license,omitempty" json:"project_license,omitempty"`

	Name        []LocalizedText `xml:"name"        json:"name,omitempty"`
	Summary     []LocalizedText `xml:"summary"     json:"summary,omitempty"`
	Description []LocalizedText `xml:"description" json:"description,omitempty"`

	Keywords    []Keyword    `xml:"keywords>keyword"       json:"keywords,omitempty"`
	Categories  []string     `xml:"categories>category"    json:"categories,omitempty"`
	Urls        []URL        `xml:"url"                    json:"urls,omitempty"`
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

type SwCatService struct{ path string }

func NewSwCatService(path string) *SwCatService { return &SwCatService{path: path} }

func (s *SwCatService) Load(ctx context.Context) ([]Component, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.UpdateAppStream"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.UpdateAppStream"))
	files, err := os.ReadDir(s.path)
	if err != nil {
		return nil, fmt.Errorf(app.T_("Cannot read dir %s: %w"), s.path, err)
	}

	pkgMap := make(map[string]Component)

	for _, f := range files {
		if f.IsDir() || !(strings.HasSuffix(f.Name(), ".xml") || strings.HasSuffix(f.Name(), ".xml.gz")) {
			continue
		}

		full := filepath.Join(s.path, f.Name())
		data, err := os.ReadFile(full)
		if err != nil {
			return nil, fmt.Errorf(app.T_("Read file %s failed: %w"), full, err)
		}
		if strings.HasSuffix(f.Name(), ".gz") {
			if data, err = decompressGzip(data); err != nil {
				return nil, fmt.Errorf(app.T_("Unpack %s failed: %w"), full, err)
			}
		}

		var cat SWCatalog
		if err = xml.Unmarshal(data, &cat); err != nil {
			return nil, fmt.Errorf(app.T_("Parse %s failed: %w"), full, err)
		}

		for _, c := range cat.Components {
			if c.PkgName == "" {
				continue
			}
			if exist, ok := pkgMap[c.PkgName]; ok {
				merged := mergeComponents(exist, c)
				sanitizeComponent(&merged)
				pkgMap[c.PkgName] = merged
			} else {
				sanitizeComponent(&c)
				pkgMap[c.PkgName] = c
			}
		}
	}

	out := make([]Component, 0, len(pkgMap))
	for _, c := range pkgMap {
		out = append(out, c)
	}
	return out, nil
}

func mergeComponents(a, b Component) Component {
	if a.Type == "" {
		a.Type = b.Type
	}
	if a.ID == "" {
		a.ID = b.ID
	}
	if a.MetadataLicense == "" {
		a.MetadataLicense = b.MetadataLicense
	}
	if a.ProjectLicense == "" {
		a.ProjectLicense = b.ProjectLicense
	}

	a.Name = append(a.Name, b.Name...)
	a.Summary = append(a.Summary, b.Summary...)
	a.Description = append(a.Description, b.Description...)

	a.Urls = append(a.Urls, b.Urls...)
	a.Screenshots = append(a.Screenshots, b.Screenshots...)
	a.Releases = append(a.Releases, b.Releases...)
	a.Icons = append(a.Icons, b.Icons...)

	kw := make(map[string]struct{})
	for _, k := range append(a.Keywords, b.Keywords...) {
		kw[k.Value] = struct{}{}
	}
	a.Keywords = a.Keywords[:0]
	for v := range kw {
		a.Keywords = append(a.Keywords, Keyword{Value: v})
	}

	cat := make(map[string]struct{})
	for _, c := range append(a.Categories, b.Categories...) {
		cat[c] = struct{}{}
	}
	a.Categories = a.Categories[:0]
	for v := range cat {
		a.Categories = append(a.Categories, v)
	}

	if a.Launchable == nil {
		a.Launchable = b.Launchable
	}
	if a.ContentRating == nil {
		a.ContentRating = b.ContentRating
	}
	return a
}

func sanitizeComponent(c *Component) {
	c.Name = dedupTexts(c.Name)
	c.Summary = dedupTexts(c.Summary)
	c.Description = dedupTexts(c.Description)

	for i := range c.Description {
		c.Description[i].Value = cleanHTML(c.Description[i].Value)
	}
}

var tagRe = regexp.MustCompile(`(?s)<[^>]*>`)

func cleanHTML(raw string) string {
	s := html.UnescapeString(raw)
	s = tagRe.ReplaceAllString(s, "")
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

func dedupTexts(src []LocalizedText) []LocalizedText {
	seen := make(map[string]struct{}, len(src))
	out := make([]LocalizedText, 0, len(src))
	for _, t := range src {
		key := t.Lang + "\x00" + t.Value
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, t)
	}
	return out
}

func decompressGzip(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func(r *gzip.Reader) {
		err = r.Close()
		if err != nil {
			app.Log.Error("decompressGzip", err)
		}
	}(r)
	return io.ReadAll(r)
}
