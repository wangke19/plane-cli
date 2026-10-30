package textutil

import "testing"

func TestStringWidth(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"hello", 5},
		{"你好", 4},       // CJK 每字宽 2
		{"混合mix文本", 10}, // 2*1 + 3 + 2*2 = 9? 逐项核算：混2合2m1i1x1文2本2=11
		{"a\tb", 3},     // tab 宽 1（未特殊处理）
	}
	cases[3].want = 11
	for _, c := range cases {
		if got := StringWidth(c.in); got != c.want {
			t.Errorf("StringWidth(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestPadClip(t *testing.T) {
	if got := Pad("ab", 4); StringWidth(got) != 4 {
		t.Errorf("Pad 宽度不对: %q", got)
	}
	if got := Pad("你好", 8); StringWidth(got) != 8 {
		t.Errorf("Pad CJK 宽度不对: %q", got)
	}
	if got := Clip("hello", 4); got != "hel…" {
		t.Errorf("Clip = %q, want hel…", got)
	}
	if got := Clip("你好世界", 7); got != "你好世…" {
		t.Errorf("Clip CJK = %q, want 你好世…", got)
	}
	if got := Clip("ab", 10); got != "ab" {
		t.Errorf("Clip 不应截断: %q", got)
	}
}

func TestHTMLRoundTrip(t *testing.T) {
	in := "第一行\n第二行 <b>加粗</b> & \"引号\""
	h := TextToHTML(in)
	want := "<p>第一行<br/>第二行 &lt;b&gt;加粗&lt;/b&gt; &amp; &#34;引号&#34;</p>"
	if h != want {
		t.Errorf("TextToHTML = %q, want %q", h, want)
	}
	got := HTMLToText(h)
	if got != in {
		t.Errorf("HTMLToText = %q, want %q", got, in)
	}
}

func TestHTMLToTextBlocks(t *testing.T) {
	in := "<p>段落一</p><p>段落二</p><div>x</div>"
	got := HTMLToText(in)
	if !containsAll(got, "段落一", "段落二", "x") {
		t.Errorf("HTMLToText 丢内容: %q", got)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestShortDate(t *testing.T) {
	if ShortDate("") != "-" {
		t.Error("空日期应返回 -")
	}
	if ShortDate("2026-12-28T00:00:00Z") != "2026-12-28" {
		t.Error("应截取日期部分")
	}
}
