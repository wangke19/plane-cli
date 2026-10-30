package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestClient 基于 httptest 服务器构建客户端（零等待重试）。
func newTestClient(t *testing.T, handler http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := New(srv.URL, "test-token", "acme")
	c.Backoff = func(int) time.Duration { return 0 }
	return c, srv
}

func TestDoSendsAuthHeader(t *testing.T) {
	var gotKey, gotPath string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	resp, err := c.Do(http.MethodGet, "users/me/", nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotKey != "test-token" {
		t.Errorf("认证头: %q", gotKey)
	}
	if gotPath != "/api/v1/users/me/" {
		t.Errorf("相对路径应补 /api/v1/ 前缀: %q", gotPath)
	}
	var out struct {
		OK bool `json:"ok"`
	}
	if err := resp.Decode(&out); err != nil || !out.OK {
		t.Errorf("Decode: %v %+v", err, out)
	}
}

func TestDoRetryOn429(t *testing.T) {
	calls := 0
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate"}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	if _, err := c.Do(http.MethodGet, "x/", nil); err != nil {
		t.Fatalf("429 后应重试成功: %v", err)
	}
	if calls != 3 {
		t.Errorf("应调用 3 次，实际 %d", calls)
	}
}

func TestDoAPIError(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Page not found."}`))
	}))
	_, err := c.Do(http.MethodGet, "x/", nil)
	var ae *APIError
	if err == nil || err.Error() == "" {
		t.Fatal("应返回错误")
	}
	if _, ok := err.(*APIError); !ok {
		t.Fatalf("应为 *APIError，得到 %T", err)
	}
	_ = ae
}

func TestPaginateFollowsCursor(t *testing.T) {
	page := 0
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		w.Header().Set("Content-Type", "application/json")
		if page == 1 {
			_, _ = w.Write([]byte(`{"results":[{"id":"1"},{"id":"2"}],"next_cursor":"100:2:0"}`))
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"3"}],"next_cursor":""}`))
	}))
	items, err := Paginate[struct {
		ID string `json:"id"`
	}](c, "workspaces/acme/projects/", 100)
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if len(items) != 3 || items[2].ID != "3" {
		t.Errorf("分页结果: %+v", items)
	}
}

func TestPaginateBareArray(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"a"},{"id":"b"}]`))
	}))
	items, err := Paginate[struct {
		ID string `json:"id"`
	}](c, "workspaces/acme/members/", 100)
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("裸数组端点应兼容: %+v", items)
	}
}
