package tool

import "testing"

// TestScriptReplaceLocationAssign 验证 location 赋值行：提取 varname 并截断到最后一个 '}'。
// 对应 strings.CutLast 改写后的行为（与旧 strings.LastIndex 版本等价）。
func TestScriptReplaceLocationAssign(t *testing.T) {
	got := ScriptReplace("return '/t' } _qf14P = location", "varname")
	want := "return '/t' }"
	if got != want {
		t.Errorf("ScriptReplace(location 赋值) = %q, want %q", got, want)
	}
}

// TestScriptReplaceLocationNoBrace 验证 location 赋值但无 '}' 时：整行清空并提取 varname。
func TestScriptReplaceLocationNoBrace(t *testing.T) {
	got := ScriptReplace("_jzvXT = location", "varname")
	if got != "" {
		t.Errorf("ScriptReplace(location 无 }) = %q, want 空串", got)
	}
}

// TestScriptReplaceWindowBound 验证 window 赋值提取 + varWindow 引用行之后的内容被截断。
func TestScriptReplaceWindowBound(t *testing.T) {
	got := ScriptReplace("window._LoKlO='a';return '/t' } _qf14P = window;var x = _qf14P", "varname")
	want := ";return '/t' }"
	if got != want {
		t.Errorf("ScriptReplace(window) = %q, want %q", got, want)
	}
}

// TestScriptReplaceWindowNoBrace 验证 window 赋值但无 '}' 时整行清空。
func TestScriptReplaceWindowNoBrace(t *testing.T) {
	got := ScriptReplace("_jzvXT = window", "varname")
	if got != "" {
		t.Errorf("ScriptReplace(window 无 }) = %q, want 空串", got)
	}
}

// TestScriptReplaceSetVarname 验证 location 属性赋值（无 " = " 分隔）分支：location.href= 被替换为 varname=。
func TestScriptReplaceSetVarname(t *testing.T) {
	got := ScriptReplace("location.href='http://x'", "varname")
	want := "varname='http://x'"
	if got != want {
		t.Errorf("ScriptReplace(set varname) = %q, want %q", got, want)
	}
}

// TestScriptReplacePlain 验证不含 location/window 的普通 JS 原样返回。
func TestScriptReplacePlain(t *testing.T) {
	js := "var a = 1; var b = 2"
	if got := ScriptReplace(js, "varname"); got != js {
		t.Errorf("ScriptReplace(普通 JS) = %q, want %q", got, js)
	}
}

// TestScriptReplaceShortInput 验证长度 < 2 的输入直接原样返回。
func TestScriptReplaceShortInput(t *testing.T) {
	if got := ScriptReplace("a", "varname"); got != "a" {
		t.Errorf("ScriptReplace(短输入) = %q, want %q", got, "a")
	}
}
