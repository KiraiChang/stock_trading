package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestCandleGapWiringUsesNormalisingHelper 讀 main.go 的 AST，守住一個**被連續三輪
// review 指出的接線缺口**（原記於 issue.md I-091，已收斂）。
//
// 缺口是：曾經用**原始設定**建 breaker 與 exchange client，之後才在
// SetCandleGapDetection 裡正規化。於是 `request_interval_ms=0` 時 scheduler 顯示
// 已正規化成 100ms、**client 卻完全不節流**；`breaker_failures=0` 時 client 用的是
// NewSourceBreaker 兜底的 1 而不是規格預設的 5。
//
// **為什麼要用 AST 而不是一般的單元測試**：`NewCandleGapDependencies` 自身的行為測試
// 只能證明那支 helper 是對的，**證明不了 main.go 有用它**。main.go 是接線本身，
// 沒有可注入的縫；改回直接建 client 的話，其他所有測試都照樣通過。
//
// 這支測試刻意只斷言兩件事，避免變成「改一行就紅」的脆弱測試：
//  1. main.go 有呼叫 scheduler.NewCandleGapDependencies；
//  2. main.go **沒有**直接呼叫 market.NewExchangeReference / market.NewSourceBreaker。
//
// 第 2 點是關鍵——只有第 1 點的話，「兩種都寫」的實作仍會通過。
func TestCandleGapWiringUsesNormalisingHelper(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse main.go failed: %v", err)
	}

	called := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		called[pkg.Name+"."+sel.Sel.Name] = true
		return true
	})

	if !called["scheduler.NewCandleGapDependencies"] {
		t.Error("main.go 必須透過 scheduler.NewCandleGapDependencies 建立缺漏偵測的依賴——" +
			"它把「正規化」與「建 client」綁在一起，兩者脫鉤過一次")
	}
	for _, banned := range []string{
		"market.NewExchangeReference",
		"market.NewSourceBreaker",
	} {
		if called[banned] {
			t.Errorf("main.go 不得直接呼叫 %s：那會繞過正規化，讓 client 拿到原始的非法設定"+
				"（request_interval_ms=0 等於完全不節流）", banned)
		}
	}
}

// 上一支測試的前提是「main.go 讀得到而且解析得動」。
// 若日後檔名或套件結構改變，這裡會先失敗，而不是讓上一支靜默地什麼都沒檢查到。
func TestMainWiringTestCanReadSource(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse main.go failed: %v", err)
	}
	var hasScheduler bool
	for _, imp := range file.Imports {
		if strings.Contains(imp.Path.Value, "internal/scheduler") {
			hasScheduler = true
		}
	}
	if !hasScheduler {
		t.Error("main.go 應 import internal/scheduler；找不到代表這支守門測試已經失效")
	}
}
