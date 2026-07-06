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

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// mockTransport подменяет HTTP-ответы по пути запроса
type mockTransport struct {
	handler func(req *http.Request) *http.Response
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := m.handler(req)
	resp.Request = req
	return resp, nil
}

func httpResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestGetTaskPackages(t *testing.T) {
	ctx := context.Background()

	t.Run("filters and dedups packages", func(t *testing.T) {
		s, _ := newTestService(t)
		s.httpClient.Transport = &mockTransport{handler: func(req *http.Request) *http.Response {
			if strings.HasSuffix(req.URL.Path, "/plan/add-bin") {
				body := strings.Join([]string{
					"foo 1.0 x86_64",
					"foo 1.0 noarch",
					"foo-debuginfo 1.0 x86_64",
					"foo-devel 1.0 x86_64",
					"bar-checkinstall 1.0 x86_64",
					"kernel-headers-std 1.0 x86_64",
					"bar 2.0 x86_64",
					"",
				}, "\n")
				return httpResponse(http.StatusOK, body)
			}
			return httpResponse(http.StatusNotFound, "")
		}}

		packages, err := s.GetTaskPackages(ctx, "12345")
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"foo", "bar"}
		if len(packages) != len(want) {
			t.Fatalf("expected %v, got %v", want, packages)
		}
		for i := range want {
			if packages[i] != want[i] {
				t.Errorf("expected %v, got %v", want, packages)
			}
		}
	})

	t.Run("task not found", func(t *testing.T) {
		s, _ := newTestService(t)
		s.httpClient.Transport = &mockTransport{handler: func(_ *http.Request) *http.Response {
			return httpResponse(http.StatusNotFound, "")
		}}

		_, err := s.GetTaskPackages(ctx, "12345")
		if err == nil {
			t.Fatal("expected error for missing task")
		}
	})
}

func TestBuildTaskURLs(t *testing.T) {
	ctx := context.Background()

	t.Run("active task with arepo", func(t *testing.T) {
		s, _ := newTestService(t)
		s.httpClient.Transport = &mockTransport{handler: func(req *http.Request) *http.Response {
			if strings.HasSuffix(req.URL.Path, "/plan/add-bin") {
				return httpResponse(http.StatusOK, "")
			}
			if strings.HasSuffix(req.URL.Path, "/plan/arepo-add-x86_64-i586") {
				return httpResponse(http.StatusOK, "libfoo 1.0\n")
			}
			return httpResponse(http.StatusNotFound, "")
		}}

		urls, err := s.buildTaskURLs(ctx, "12345")
		if err != nil {
			t.Fatal(err)
		}
		if len(urls) != 2 {
			t.Fatalf("expected 2 urls, got %d: %v", len(urls), urls)
		}
		if !strings.Contains(urls[0], "12345") || !strings.HasSuffix(urls[0], "x86_64 task") {
			t.Errorf("unexpected main url: %s", urls[0])
		}
		if !strings.Contains(urls[1], "x86_64-i586") {
			t.Errorf("unexpected arepo url: %s", urls[1])
		}
	})

	t.Run("missing task", func(t *testing.T) {
		s, _ := newTestService(t)
		s.httpClient.Transport = &mockTransport{handler: func(_ *http.Request) *http.Response {
			return httpResponse(http.StatusNotFound, "")
		}}

		_, err := s.buildTaskURLs(ctx, "12345")
		if err == nil {
			t.Fatal("expected error for missing task")
		}
	})
}

// archiveHandler эмулирует редирект git.altlinux.org для архивных задач
func archiveHandler(archiveBase string) func(req *http.Request) *http.Response {
	return func(req *http.Request) *http.Response {
		p := req.URL.Path
		switch {
		case strings.HasPrefix(p, "/tasks/12345/"):
			resp := httpResponse(http.StatusMovedPermanently, "")
			resp.Header.Set("Location", "http://git.altlinux.org"+archiveBase+strings.TrimPrefix(p, "/tasks/12345"))
			return resp
		case p == archiveBase+"/plan/add-bin":
			return httpResponse(http.StatusOK, "foo 1.0 x86_64\nbar 2.0 x86_64\n")
		default:
			return httpResponse(http.StatusNotFound, "")
		}
	}
}

func TestGetTaskPackages_ArchivedTaskRedirect(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	s.httpClient.Transport = &mockTransport{handler: archiveHandler("/tasks/archive/done/_360/12345")}

	packages, err := s.GetTaskPackages(ctx, "12345")
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 2 || packages[0] != "foo" || packages[1] != "bar" {
		t.Errorf("expected [foo bar], got %v", packages)
	}
}

func TestBuildTaskURLs_ArchivedTaskUsesBuildRepo(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	s.httpClient.Transport = &mockTransport{handler: archiveHandler("/tasks/archive/done/_360/12345")}

	urls, err := s.buildTaskURLs(ctx, "12345")
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) == 0 {
		t.Fatal("expected urls for archived task")
	}
	if !strings.Contains(urls[0], "/tasks/archive/done/_360/12345/build/repo/") {
		t.Errorf("expected archive build/repo url, got: %s", urls[0])
	}
	if strings.Contains(urls[0], "git.altlinux.org/repo") {
		t.Errorf("archived task must not use active repo url: %s", urls[0])
	}
}
