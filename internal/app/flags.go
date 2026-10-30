package app

import (
	"strconv"
	"strings"
)

// flagSet 是一个零依赖的命令行参数解析器：
// 支持 flag 与位置参数任意交错（gh 风格），--flag=value 与 --flag value 两种写法，
// 短/长别名，"--" 终止符，未知 flag 报错。
//
//	flagSet 注意：本解析器不做校验以外的事，具体取值合法性由命令自行检查。
type flagSet struct {
	entries map[string]*flagEntry
	order   []string
	args    []string // 解析后的位置参数
	helpReq bool     // 遇到 -h/--help
}

type flagEntry struct {
	kind  string // "string" | "bool" | "list"
	str   string
	boolV bool
	list  []string
}

func newFlagSet() *flagSet {
	return &flagSet{entries: map[string]*flagEntry{}}
}

func (f *flagSet) decl(kind string, names ...string) *flagEntry {
	e := &flagEntry{kind: kind}
	for _, n := range names {
		f.entries[n] = e
		f.order = append(f.order, n)
	}
	return e
}

// str 声明字符串 flag（首个名字为主名，其余为别名）。
func (f *flagSet) str(names ...string) *flagEntry { return f.decl("string", names...) }

// list 声明可重复出现的字符串列表 flag（nargs=*）。
func (f *flagSet) list(names ...string) *flagEntry { return f.decl("list", names...) }

// boolean 声明布尔开关 flag。
func (f *flagSet) boolean(names ...string) *flagEntry { return f.decl("bool", names...) }

// parse 解析参数，返回位置参数列表。
func (f *flagSet) parse(argv []string) ([]string, error) {
	positionalOnly := false
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if positionalOnly || tok == "-" || !strings.HasPrefix(tok, "-") || tok == "" {
			f.args = append(f.args, tok)
			continue
		}
		if tok == "--" {
			positionalOnly = true
			continue
		}
		name := strings.TrimLeft(tok, "-")
		value := ""
		hasValue := false
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			value, hasValue = name[eq+1:], true
			name = name[:eq]
		}
		e, ok := f.entries[name]
		if !ok {
			return nil, usagef("未知参数 '--%s'", name)
		}
		switch e.kind {
		case "bool":
			if hasValue {
				b, err := strconv.ParseBool(value)
				if err != nil {
					return nil, usagef("--%s 需要 true/false，得到 '%s'", name, value)
				}
				e.boolV = b
			} else {
				e.boolV = true
			}
			if name == "help" {
				f.helpReq = true
			}
		case "string":
			if !hasValue {
				if i+1 >= len(argv) {
					return nil, usagef("--%s 缺少取值", name)
				}
				i++
				value = argv[i]
			}
			e.str = value
		case "list":
			if !hasValue {
				if i+1 >= len(argv) {
					return nil, usagef("--%s 缺少取值", name)
				}
				i++
				value = argv[i]
			}
			e.list = append(e.list, value)
		}
	}
	return f.args, nil
}

// val 返回字符串 flag 取值；未设置时返回默认值。
func (f *flagSet) val(name, def string) string {
	if e, ok := f.entries[name]; ok && e.str != "" {
		return e.str
	}
	return def
}

// isSet 返回布尔 flag 是否被置位。
func (f *flagSet) isSet(name string) bool {
	if e, ok := f.entries[name]; ok {
		return e.boolV
	}
	return false
}

// values 返回列表 flag 的取值切片。
func (f *flagSet) values(name string) []string {
	if e, ok := f.entries[name]; ok {
		return e.list
	}
	return nil
}

// require 至少一个位置参数，返回之（用于 <ref> 类参数）。
func (f *flagSet) require(positionals []string, what string) (string, error) {
	if len(positionals) < 1 {
		return "", usagef("缺少 %s", what)
	}
	return positionals[0], nil
}
