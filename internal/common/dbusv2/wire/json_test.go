package wire

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"altlinux.space/alt-atomic/apm/internal/common/filter"
)

func TestJSONReply(t *testing.T) {
	t.Run("response", func(t *testing.T) {
		type response struct {
			Message string `json:"message"`
			Count   int    `json:"count"`
		}

		got, dbusErr := JSONReply(response{Message: "done", Count: 2}, nil)
		if dbusErr != nil {
			t.Fatal(dbusErr)
		}
		if got != `{"message":"done","count":2}` {
			t.Fatalf("JSONReply() = %s", got)
		}
	})

	t.Run("error", func(t *testing.T) {
		got, dbusErr := JSONReply(nil, errors.New("boom"))
		if got != "" {
			t.Fatalf("JSONReply() response = %q, want empty", got)
		}
		if dbusErr == nil {
			t.Fatal("JSONReply() error is nil")
		}
	})
}

func TestDecodeOptions(t *testing.T) {
	type options struct {
		DownloadOnly bool `json:"downloadOnly"`
	}

	var got options
	if err := DecodeOptions(`{"downloadOnly":true}`, &got); err != nil {
		t.Fatal(err)
	}
	if !got.DownloadOnly {
		t.Fatal("downloadOnly = false")
	}
	if err := DecodeOptions(`{"downlodOnly":true}`, &got); err == nil {
		t.Fatal("unknown option accepted")
	}
	if err := DecodeOptions(`{} {}`, &got); err == nil {
		t.Fatal("multiple JSON values accepted")
	}
}

func TestJSONTask(t *testing.T) {
	task := JSONTask(func(context.Context) (struct {
		Message string `json:"message"`
	}, error) {
		return struct {
			Message string `json:"message"`
		}{Message: "done"}, nil
	})

	got, err := task(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"message":"done"}` {
		t.Fatalf("JSONTask() = %s", got)
	}
}

func TestListRequestValidate(t *testing.T) {
	config := &filter.Config{Fields: map[string]filter.FieldConfig{
		"name": {DefaultOp: filter.OpEq, AllowedOps: []filter.Op{filter.OpEq}},
	}}
	request := ListRequest{
		PageRequest: PageRequest{Limit: 25, Offset: 10},
		ListBody: filter.ListBody{Filters: []filter.Filter{
			{Field: "name", Op: filter.OpEq, Value: "hello"},
		}},
	}

	page, err := request.Validate(config)
	if err != nil {
		t.Fatal(err)
	}
	if page.Limit != 25 || page.Offset != 10 || len(page.Filters) != 1 {
		t.Fatalf("page = %+v", page)
	}
}

func TestPageRequestValidate(t *testing.T) {
	page, err := PageRequest{}.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if page.Limit != PageDefaultLimit || page.Offset != 0 {
		t.Fatalf("page = %+v", page)
	}
	if _, err := (PageRequest{Limit: PageMaxLimit + 1}).Validate(); err == nil {
		t.Fatal("limit above maximum accepted")
	}
}

func TestPageOffsetPlatformRange(t *testing.T) {
	const maxUint32 = ^uint32(0)
	page, err := PageRequest{Offset: maxUint32}.Validate()
	if strconv.IntSize == 32 {
		if err == nil {
			t.Fatalf("offset %d = %d, want overflow error", maxUint32, page.Offset)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if uint64(page.Offset) != uint64(maxUint32) {
		t.Fatalf("offset %d = %d", maxUint32, page.Offset)
	}
}

func TestDecodeJSON(t *testing.T) {
	type request struct {
		Limit uint32 `json:"limit"`
	}

	var got request
	if err := DecodeJSON(`{"limit":25}`, &got); err != nil {
		t.Fatal(err)
	}
	if got.Limit != 25 {
		t.Fatalf("limit = %d", got.Limit)
	}
	if err := DecodeJSON(`{"limit":`, &got); err == nil {
		t.Fatal("invalid JSON request accepted")
	}
}
