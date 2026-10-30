// Package api 封装 Plane 外部 REST API（/api/v1/，PAT 认证）：
// 带限流退避的 HTTP 客户端、游标分页、类型化解码与资源解析。
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// APIError 表示一次失败的 API 调用。
type APIError struct {
	Status int
	Method string
	Path   string
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("HTTP %d %s %s: %s", e.Status, e.Method, e.Path, e.Body)
}

// Client 是对单个工作区的 API 客户端。
type Client struct {
	BaseURL    string // 形如 https://plane.example.com，末尾无斜杠
	Token      string
	Workspace  string
	HTTP       *http.Client
	MaxRetries int                             // 429 限流的最大重试次数
	Backoff    func(attempt int) time.Duration // 第 attempt 次（0 起）等待时长
}

// New 基于连接三元组创建客户端。
func New(baseURL, token, workspace string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Token:      token,
		Workspace:  workspace,
		HTTP:       &http.Client{Timeout: 30 * time.Second},
		MaxRetries: 3,
		Backoff:    defaultBackoff,
	}
}

func defaultBackoff(attempt int) time.Duration {
	return time.Duration(2<<attempt) * time.Second // 2s, 4s, 8s
}

// url 把相对路径转为完整 URL：以 / 开头原样拼接，否则补 /api/v1/ 前缀。
func (c *Client) url(path string) string {
	if strings.HasPrefix(path, "/") {
		return c.BaseURL + path
	}
	return c.BaseURL + "/api/v1/" + path
}

// Response 是一次成功调用的原始返回。
type Response struct {
	Status int
	Raw    json.RawMessage
}

// Decode 将返回体解码到 v（204 空体时不动 v）。
func (r *Response) Decode(v any) error {
	if len(r.Raw) == 0 {
		return nil
	}
	return json.Unmarshal(r.Raw, v)
}

// Do 执行一次 API 调用。body 非 nil 时以 JSON 发送。
// okCodes 为可接受的状态码集合，缺省 200/201/204。
func (c *Client) Do(method, path string, body any, okCodes ...int) (*Response, error) {
	if len(okCodes) == 0 {
		okCodes = []int{http.StatusOK, http.StatusCreated, http.StatusNoContent}
	}
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return nil, fmt.Errorf("序列化请求体: %w", err)
		}
	}
	var lastErr error
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		req, err := http.NewRequest(method, c.url(path), bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-API-Key", c.Token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, fmt.Errorf("连接失败 %s: %w", c.BaseURL, err)
		}
		raw, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("读取响应: %w", readErr)
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < c.MaxRetries {
			wait := c.Backoff(attempt)
			if ra, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil {
				if d := time.Duration(ra) * time.Second; d > wait {
					wait = d
				}
			}
			time.Sleep(wait)
			continue
		}
		ok := false
		for _, code := range okCodes {
			if resp.StatusCode == code {
				ok = true
				break
			}
		}
		if !ok {
			lastErr = &APIError{Status: resp.StatusCode, Method: method, Path: path, Body: truncate(string(raw), 300)}
			break
		}
		return &Response{Status: resp.StatusCode, Raw: json.RawMessage(raw)}, nil
	}
	return nil, lastErr
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// page 是分页接口的信封；个别端点直接返回数组，用 asList 兼容。
type page[T any] struct {
	Results    []T    `json:"results"`
	NextCursor string `json:"next_cursor"`
}

// Paginate 跟随 next_cursor 翻页，最多取 limit 条；同时兼容直接返回数组的端点。
func Paginate[T any](c *Client, path string, limit int) ([]T, error) {
	var out []T
	cursor := ""
	for len(out) < limit {
		p := path
		if cursor != "" {
			sep := "?"
			if strings.Contains(path, "?") {
				sep = "&"
			}
			p = path + sep + "cursor=" + cursor
		}
		resp, err := c.Do(http.MethodGet, p, nil)
		if err != nil {
			return nil, err
		}
		var items []T
		if len(resp.Raw) > 0 && resp.Raw[0] == '[' {
			// 裸数组端点（如 members）
			if err := json.Unmarshal(resp.Raw, &items); err != nil {
				return nil, err
			}
		} else {
			var env page[T]
			if err := json.Unmarshal(resp.Raw, &env); err != nil {
				return nil, err
			}
			items = env.Results
			cursor = env.NextCursor
		}
		out = append(out, items...)
		if cursor == "" || len(items) == 0 {
			break
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
