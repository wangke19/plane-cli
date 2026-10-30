package app

import "testing"

func TestMergeIDs(t *testing.T) {
	cases := []struct {
		name    string
		current []string
		add     []string
		remove  []string
		clear   bool
		want    []string
	}{
		{"追加去重", []string{"a"}, []string{"b", "a"}, nil, false, []string{"a", "b"}},
		{"移除", []string{"a", "b"}, nil, []string{"a"}, false, []string{"b"}},
		{"清空后追加", []string{"a"}, []string{"c"}, nil, true, []string{"c"}},
		{"只清空", []string{"a"}, nil, nil, true, []string{}},
		{"add 不受 clear 之外的 current 重复影响", []string{"a"}, []string{"a", "b"}, nil, false, []string{"a", "b"}},
	}
	for _, c := range cases {
		got := mergeIDs(c.current, c.add, c.remove, c.clear)
		if len(got) != len(c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: got %v, want %v", c.name, got, c.want)
				break
			}
		}
	}
}

func TestFlagSetInterleaved(t *testing.T) {
	f := newFlagSet()
	f.str("p", "project")
	f.str("d", "description")
	f.boolean("clear")
	f.list("remove", "members")
	args, err := f.parse([]string{"8", "kewang", "--remove", "bot", "-p", "demo", "extra", "--clear", "--description=hello"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(args) != 3 || args[0] != "8" || args[1] != "kewang" || args[2] != "extra" {
		t.Errorf("位置参数: %v", args)
	}
	if f.val("p", "") != "demo" {
		t.Errorf("短别名 -p 未生效: %q", f.val("p", ""))
	}
	if f.val("description", "") != "hello" {
		t.Errorf("= 赋值未生效: %q", f.val("description", ""))
	}
	if !f.isSet("clear") {
		t.Error("--clear 未置位")
	}
	got := f.values("remove")
	if len(got) != 1 || got[0] != "bot" {
		t.Errorf("list flag: %v", got)
	}
}

func TestFlagSetUnknown(t *testing.T) {
	f := newFlagSet()
	f.str("p")
	if _, err := f.parse([]string{"--nope", "x"}); err == nil {
		t.Error("未知 flag 应报错")
	}
}

func TestFlagSetDoubleDash(t *testing.T) {
	f := newFlagSet()
	f.str("p")
	args, err := f.parse([]string{"--", "--not-a-flag", "-x"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(args) != 2 || args[0] != "--not-a-flag" {
		t.Errorf("-- 后应全部视为位置参数: %v", args)
	}
}
