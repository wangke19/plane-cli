package config

import (
	"strings"
	"testing"
)

func setenvs(t *testing.T, kv map[string]string) {
	t.Helper()
	for _, k := range []string{"PLANE_TOKEN", "PLANE_BASE_URL", "PLANE_WORKSPACE", "PLANE_PROJECT"} {
		t.Setenv(k, "")
	}
	for k, v := range kv {
		t.Setenv(k, v)
	}
}

func TestFromEnvOK(t *testing.T) {
	setenvs(t, map[string]string{
		"PLANE_TOKEN":     " test-token ",
		"PLANE_BASE_URL":  "https://plane.example.com/",
		"PLANE_WORKSPACE": "acme",
		"PLANE_PROJECT":   "demo",
	})
	c, err := FromEnv()
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if c.Token != "test-token" {
		t.Errorf("token 应去空白: %q", c.Token)
	}
	if c.BaseURL != "https://plane.example.com" {
		t.Errorf("base url 应去尾斜杠: %q", c.BaseURL)
	}
	if c.DefaultProject != "demo" {
		t.Errorf("默认项目: %q", c.DefaultProject)
	}
}

func TestFromEnvMissing(t *testing.T) {
	setenvs(t, map[string]string{})
	_, err := FromEnv()
	if err == nil {
		t.Fatal("缺配置时应报错")
	}
	for _, want := range []string{"PLANE_TOKEN", "PLANE_BASE_URL", "PLANE_WORKSPACE"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("错误应包含 %s: %v", want, err)
		}
	}
}
