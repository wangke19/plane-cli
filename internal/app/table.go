package app

import (
	"fmt"
	"io"

	"plane-cli/internal/textutil"
)

// printTable 输出按显示宽度对齐的两空格分隔表格（CJK 友好）。
func printTable(w io.Writer, headers []string, rows [][]string) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = textutil.StringWidth(h)
	}
	for _, r := range rows {
		for i, cell := range r {
			if i < len(widths) && textutil.StringWidth(cell) > widths[i] {
				widths[i] = textutil.StringWidth(cell)
			}
		}
	}
	line := func(cells []string) {
		parts := make([]string, len(cells))
		for i, c := range cells {
			parts[i] = textutil.Pad(c, widths[i])
		}
		fmt.Fprintln(w, joinCells(parts))
	}
	line(headers)
	for _, r := range rows {
		line(r)
	}
}

// joinCells 以两空格连接已对齐的单元格。
func joinCells(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "  "
		}
		out += p
	}
	return out
}
