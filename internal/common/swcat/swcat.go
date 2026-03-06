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

type Service struct{ path string }

func NewSwCatService(path string) *Service { return &Service{path: path} }

func (s *Service) Load(ctx context.Context) (map[string][]Component, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemUpdateAppStream))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemUpdateAppStream))
	files, err := os.ReadDir(s.path)
	if err != nil {
		return nil, fmt.Errorf(app.T_("Cannot read dir %s: %w"), s.path, err)
	}

	pkgMap := make(map[string][]Component)

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
			sanitizeComponent(&c)
			pkgMap[c.PkgName] = append(pkgMap[c.PkgName], c)
		}
	}

	return pkgMap, nil
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

func dedupTexts(src LocalizedMap) LocalizedMap {
	seen := make(map[string]struct{}, len(src))
	out := make(LocalizedMap, 0, len(src))
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
