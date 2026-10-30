// Package textutil 提供终端文本处理工具：
// CJK 感知的显示宽度、表格对齐辅助，以及描述字段的 HTML ↔ 纯文本转换。
package textutil

import (
	"html"
	"regexp"
	"strings"
)

// wideRanges 覆盖东亚宽字符的主要 Unicode 区段（谚文/CJK/全角等）。
var wideRanges = [][2]rune{
	{0x1100, 0x115F}, {0x2E80, 0x303E}, {0x3041, 0x33FF}, {0x3400, 0x4DBF},
	{0x4E00, 0x9FFF}, {0xA000, 0xA4CF}, {0xAC00, 0xD7A3}, {0xF900, 0xFAFF},
	{0xFE30, 0xFE4F}, {0xFF00, 0xFF60}, {0xFFE0, 0xFFE6},
	{0x1F300, 0x1F64F}, {0x1F900, 0x1F9FF}, // emoji 主区段
	{0x20000, 0x2FFFD}, {0x30000, 0x3FFFD},
}

// RuneWidth 返回单个字符的终端显示宽度：宽字符 2，NUL/组合字符 0，其余 1（含 TAB）。
func RuneWidth(r rune) int {
	switch {
	case r == 0:
		return 0
	case r >= 0x0300 && r <= 0x036F:
		return 0 // 组合变音符号
	}
	for _, rg := range wideRanges {
		if r >= rg[0] && r <= rg[1] {
			return 2
		}
	}
	return 1
}

// StringWidth 返回字符串的终端显示宽度。
func StringWidth(s string) int {
	w := 0
	for _, r := range s {
		w += RuneWidth(r)
	}
	return w
}

// Pad 将 s 左对齐补空格到显示宽度 w。
func Pad(s string, w int) string {
	d := w - StringWidth(s)
	if d <= 0 {
		return s
	}
	return s + strings.Repeat(" ", d)
}

// Clip 将 s 按显示宽度截断，超出部分以省略号（…）结尾。
func Clip(s string, w int) string {
	if StringWidth(s) <= w {
		return s
	}
	n, out := 0, strings.Builder{}
	for _, r := range s {
		rw := RuneWidth(r)
		if n+rw > w-1 { // 预留省略号位置
			break
		}
		out.WriteRune(r)
		n += rw
	}
	return out.String() + "…"
}

var (
	tagBR      = regexp.MustCompile(`(?i)<br\s*/?>`)
	blockClose = regexp.MustCompile(`(?i)</(p|div|li|h[1-6]|tr)>`)
	anyTag     = regexp.MustCompile(`<[^>]*>`)
	multiNL    = regexp.MustCompile(`\n{3,}`)
)

// HTMLToText 将 Plane 的 description_html 转为可读纯文本（去标签、还原实体、压缩空行）。
func HTMLToText(h string) string {
	if h == "" {
		return ""
	}
	t := tagBR.ReplaceAllString(h, "\n")
	t = blockClose.ReplaceAllString(t, "\n")
	t = anyTag.ReplaceAllString(t, "")
	t = html.UnescapeString(t)
	lines := strings.Split(t, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimRight(ln, " \t")
	}
	return strings.TrimSpace(multiNL.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}

// TextToHTML 将纯文本转为 Plane API 接受的 description_html（转义 + <p>/<br/>）。
func TextToHTML(s string) string {
	var b strings.Builder
	b.WriteString("<p>")
	for i, ln := range strings.Split(s, "\n") {
		if i > 0 {
			b.WriteString("<br/>")
		}
		b.WriteString(html.EscapeString(ln))
	}
	b.WriteString("</p>")
	return b.String()
}

// ShortDate 取 RFC3339/日期字符串的前 10 位（YYYY-MM-DD），空值返回 "-"。
func ShortDate(s string) string {
	if s == "" {
		return "-"
	}
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}
