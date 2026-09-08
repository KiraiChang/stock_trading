package store

import (
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
)

// pragmasOf 取出 DSN 裡的 _pragma 清單（已排序，方便比對）。
func pragmasOf(t *testing.T, dsn string) []string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse %q: %v", dsn, err)
	}
	got := append([]string(nil), u.Query()["_pragma"]...)
	sort.Strings(got)
	return got
}

func hasPragma(t *testing.T, dsn, want string) bool {
	t.Helper()
	for _, p := range pragmasOf(t, dsn) {
		if p == want {
			return true
		}
	}
	return false
}

// countByName 回傳某個 pragma 名稱在 DSN 裡出現幾次。
//
// ⛔ **刻意不呼叫 production 的 isEnforcedSQLitePragma**——那會讓測試拿被測程式
// 自己的判斷去檢查它自己（套套邏輯）：把 production 的名稱切法改壞時，
// 這裡的斷言也會跟著看不見漏網的值，測試照樣是綠的。實測過，M1 mutation
// （只依 `(` 切名稱）在改成獨立實作之前確實抓不到。
//
// 這裡用一個**獨立、刻意寬鬆**的判準：只要 trim + 轉小寫之後以該名稱開頭就算。
func countByName(t *testing.T, dsn, name string) int {
	t.Helper()
	n := 0
	for _, p := range pragmasOf(t, dsn) {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(p)), name) {
			n++
		}
	}
	return n
}

// assertEnforced 檢查兩個強制 pragma 各恰好一份。
func assertEnforced(t *testing.T, dsn string) {
	t.Helper()
	if !hasPragma(t, dsn, "busy_timeout("+sqliteBusyTimeoutMS+")") {
		t.Errorf("缺 busy_timeout，DSN=%q", dsn)
	}
	if !hasPragma(t, dsn, "foreign_keys(1)") {
		t.Errorf("缺 foreign_keys，DSN=%q", dsn)
	}
	// ⛔ 恰好一份：留下第二份的話，最後生效的取決於 driver 的套用順序，
	// 外部就有機會用 foreign_keys(0) 把 FK 關掉。
	if n := countByName(t, dsn, "busy_timeout"); n != 1 {
		t.Errorf("busy_timeout 應恰好一份，得到 %d（DSN=%q）", n, dsn)
	}
	if n := countByName(t, dsn, "foreign_keys"); n != 1 {
		t.Errorf("foreign_keys 應恰好一份，得到 %d（DSN=%q）", n, dsn)
	}
}

// ── #20f5a：DSN builder 的單元測試（只驗產生的字串，不連線）────────────────

func TestBuildSQLiteDSN_EmptyFallsBackToAbsoluteDefault(t *testing.T) {
	absDefault, err := filepath.Abs("./trading.db")
	if err != nil {
		t.Fatal(err)
	}
	got, err := buildSQLiteDSN("")
	if err != nil {
		t.Fatal(err)
	}
	// ⛔ 不對字面 "./trading.db" 斷言——builder 已改為先 filepath.Abs。
	if !strings.HasPrefix(got, "file://"+absDefault+"?") {
		t.Errorf("空 DSN 應以 file://%s 開頭，得到 %q", absDefault, got)
	}
	assertEnforced(t, got)
}

// 相對路徑是 live 現況（config.yaml 的 ../../output/stock_trading/trading.db）。
//
// ⛔ 核心斷言是「不得產生 authority」：沒有先 filepath.Abs 的話，
// url.URL 會輸出 file://../../output/... ，反解時 ".." 變成 host、路徑被改掉。
func TestBuildSQLiteDSN_RelativePathIsMadeAbsolute(t *testing.T) {
	const rel = "../../output/stock_trading/trading.db"
	got, err := buildSQLiteDSN(rel)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse %q: %v", got, err)
	}
	if u.Host != "" {
		t.Errorf("相對路徑不得產生 authority，得到 host=%q（DSN=%q）", u.Host, got)
	}
	abs, _ := filepath.Abs(rel)
	if u.Path != abs {
		t.Errorf("path 應為絕對路徑 %q，得到 %q", abs, u.Path)
	}
	assertEnforced(t, got)
}

func TestBuildSQLiteDSN_AbsolutePath(t *testing.T) {
	got, err := buildSQLiteDSN("/var/lib/trading/x.db")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(got)
	if u.Host != "" || u.Path != "/var/lib/trading/x.db" {
		t.Errorf("host=%q path=%q，期望 host 空、path=/var/lib/trading/x.db", u.Host, u.Path)
	}
	assertEnforced(t, got)
}

func TestBuildSQLiteDSN_ExistingURIKeepsOtherParams(t *testing.T) {
	got, err := buildSQLiteDSN("file:/data/x.db?mode=rwc&_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(got)
	q := u.Query()
	if q.Get("mode") != "rwc" {
		t.Errorf("mode 應保留 rwc，得到 %q（DSN=%q）", q.Get("mode"), got)
	}
	if q.Get("_txlock") != "immediate" {
		t.Errorf("_txlock 應保留 immediate，得到 %q", q.Get("_txlock"))
	}
	if u.Path != "/data/x.db" {
		t.Errorf("已是 URI 時不得改動 path，得到 %q", u.Path)
	}
	assertEnforced(t, got)
}

// memory DSN 三種都不得被 filepath.Abs 變成磁碟路徑。
func TestBuildSQLiteDSN_MemoryFormsAreNotMadeAbsolute(t *testing.T) {
	cwd, _ := os.Getwd()
	cases := []struct {
		name  string
		dsn   string
		check func(t *testing.T, u *url.URL)
	}{
		{
			name: "裸 :memory:",
			dsn:  ":memory:",
			check: func(t *testing.T, u *url.URL) {
				if u.Opaque != ":memory:" {
					t.Errorf("應為 Opaque=\":memory:\"，得到 opaque=%q path=%q", u.Opaque, u.Path)
				}
			},
		},
		{
			name: "file::memory:?cache=shared",
			dsn:  "file::memory:?cache=shared",
			check: func(t *testing.T, u *url.URL) {
				if u.Opaque != ":memory:" {
					t.Errorf("應維持 Opaque=\":memory:\"，得到 opaque=%q path=%q", u.Opaque, u.Path)
				}
				if u.Query().Get("cache") != "shared" {
					t.Errorf("cache=shared 必須保留，得到 %q", u.Query().Get("cache"))
				}
			},
		},
		{
			name: "named memory URI",
			dsn:  "file:t071?mode=memory&cache=shared",
			check: func(t *testing.T, u *url.URL) {
				if u.Opaque != "t071" {
					t.Errorf("named memory 的 opaque 應為 t071，得到 opaque=%q path=%q", u.Opaque, u.Path)
				}
				q := u.Query()
				if q.Get("mode") != "memory" || q.Get("cache") != "shared" {
					t.Errorf("mode/cache 必須保留，得到 mode=%q cache=%q", q.Get("mode"), q.Get("cache"))
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := buildSQLiteDSN(c.dsn)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(got, cwd) {
				t.Fatalf("memory DSN 不得含工作目錄（被 Abs 了）：%q", got)
			}
			u, err := url.Parse(got)
			if err != nil {
				t.Fatalf("parse %q: %v", got, err)
			}
			if u.Path != "" {
				t.Errorf("memory DSN 不應有 path，得到 %q", u.Path)
			}
			c.check(t, u)
			assertEnforced(t, got)
		})
	}
}

// ⛔ 只測括號形式的話，實作若依 `(` 切名稱，`foreign_keys=OFF` 會整個穿過去。
func TestBuildSQLiteDSN_ConflictingPragmaSyntaxVariants(t *testing.T) {
	// ⚠️ 這裡列的是 **decode 之後**的值——也就是 url.Query() 交給比對邏輯的東西。
	// URL encode 那個變體差異在 **raw query 層**而不是 Go 字串層，
	// 所以另外用 raw query 測（見 TestBuildSQLiteDSN_PercentEncodedConflictIsRemoved）。
	// ⛔ 別把 "foreign_keys%3D0" 放進這張表：那個 Go 字串經 url.Values.Encode()
	// 會變成 raw 的 %253D，decode 回來仍是 "foreign_keys%3D0"，
	// 那是 **double-encode** 的案例、預期相反（必須保留）。
	variants := []string{
		"foreign_keys(0)",   // 括號
		"foreign_keys=OFF",  // 等號
		"foreign_keys = 0",  // 空白
		"FOREIGN_KEYS(0)",   // 大小寫
		"Busy_Timeout=0",    // 大小寫 + 等號
		" busy_timeout(0) ", // 前後空白
	}
	for _, v := range variants {
		t.Run(v, func(t *testing.T) {
			q := url.Values{"_pragma": []string{v}}
			got, err := buildSQLiteDSN("file:/data/x.db?" + q.Encode())
			if err != nil {
				t.Fatal(err)
			}
			// ⛔ 兩條斷言都**不呼叫 production 的判斷函式**，否則就是拿被測程式
			// 檢查它自己。①輸入的衝突值不得原樣殘留；②總數必須恰好是 2
			// （輸入那 1 個被移除、強制項補回 2 個）——②是真正抓得到
			// 「名稱切法寫錯讓某個變體穿過去」的那一條。
			for _, p := range pragmasOf(t, got) {
				if p == v {
					t.Errorf("衝突值 %q 未被移除，DSN=%q", p, got)
				}
			}
			if n := len(pragmasOf(t, got)); n != 2 {
				t.Errorf("_pragma 總數應為 2（衝突值被移除、強制項各一），得到 %d：%v", n, pragmasOf(t, got))
			}
			assertEnforced(t, got)
		})
	}
}

func TestBuildSQLiteDSN_KeepsUnrelatedPragmas(t *testing.T) {
	q := url.Values{"_pragma": []string{"cache_size(-2000)", "foreign_keys(0)"}}
	got, err := buildSQLiteDSN("file:/data/x.db?" + q.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if !hasPragma(t, got, "cache_size(-2000)") {
		t.Errorf("非強制的 _pragma 必須保留，得到 %v", pragmasOf(t, got))
	}
	// cache_size 保留 + 強制項兩個 = 3；foreign_keys(0) 必須已被移除。
	if n := len(pragmasOf(t, got)); n != 3 {
		t.Errorf("_pragma 總數應為 3，得到 %d：%v", n, pragmasOf(t, got))
	}
	assertEnforced(t, got)
}

// raw query 帶單層 percent-encode：`_pragma=foreign_keys%3D0` 經 url.Query()
// decode 一次得到 `foreign_keys=0`，**是**衝突值，必須被移除。
//
// ⚠️ 這條與下面的 double-encode 測試是**同一個 Go 字串、不同的 raw query**，
// 兩者的預期相反——差異只存在於 wire format，所以這裡刻意手組 raw query，
// ⛔ 不能用 url.Values.Encode()（它會再編一層，就變成 double-encode 那條）。
func TestBuildSQLiteDSN_PercentEncodedConflictIsRemoved(t *testing.T) {
	got, err := buildSQLiteDSN("file:/data/x.db?_pragma=foreign_keys%3D0")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pragmasOf(t, got) {
		if p == "foreign_keys=0" {
			t.Errorf("單層 encode 的衝突值 decode 後應被移除，DSN=%q", got)
		}
	}
	if n := len(pragmasOf(t, got)); n != 2 {
		t.Errorf("_pragma 總數應為 2，得到 %d：%v", n, pragmasOf(t, got))
	}
	assertEnforced(t, got)
}

// %253D 經 url.Query() decode 一次得到 "foreign_keys%3D0"——那**不是** foreign_keys。
// ⛔ 不得再 decode 一次而誤判成衝突項移除。
func TestBuildSQLiteDSN_DoubleEncodedValueIsKept(t *testing.T) {
	q := url.Values{"_pragma": []string{"foreign_keys%3D0"}}
	got, err := buildSQLiteDSN("file:/data/x.db?" + q.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if !hasPragma(t, got, "foreign_keys%3D0") {
		t.Errorf("double-encoded 值必須原樣保留，得到 %v", pragmasOf(t, got))
	}
}

// ── #20f5b：實際開啟連線的 integration test ────────────────────────────────
//
// A／B／C 都驗 ①busy_timeout=5000 ②foreign_keys=1；
// A／B 另驗 ③database_list 的 main 路徑等於預期的絕對路徑，C 不驗 ③。

func assertPragmaValues(t *testing.T, db *sqlx.DB) {
	t.Helper()
	var busy int
	if err := db.Get(&busy, `PRAGMA busy_timeout`); err != nil {
		t.Fatalf("查 busy_timeout: %v", err)
	}
	if busy != 5000 {
		t.Errorf("busy_timeout 應為 5000，得到 %d", busy)
	}
	var fk int
	if err := db.Get(&fk, `PRAGMA foreign_keys`); err != nil {
		t.Fatalf("查 foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys 應為 1，得到 %d", fk)
	}
}

// ⛔ **這條不可省**：`?` / `#` 的 escape 錯掉時，driver 仍可能**成功開啟另一個
// 被截斷或改名的 DB**，只查兩個 pragma 的測試照樣會綠。
func assertMainFile(t *testing.T, db *sqlx.DB, wantAbs string) {
	t.Helper()
	rows, err := db.Query(`PRAGMA database_list`)
	if err != nil {
		t.Fatalf("database_list: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var seq int
		var name, file string
		if err := rows.Scan(&seq, &name, &file); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if name == "main" {
			if file != wantAbs {
				t.Errorf("開啟的檔案不是預期的那個：\n got=%q\nwant=%q", file, wantAbs)
			}
			return
		}
	}
	t.Fatal("database_list 沒有 main")
}

// 檔名刻意含空白／?／#／中文——那些正是 URI escape 會出錯的字元。
const trickySQLiteFileName = "測 試?a#b.db"

// A｜絕對路徑
func TestNewSQLiteAppliesPragmasAbsolutePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), trickySQLiteFileName)
	db, err := NewSQLite(path)
	if err != nil {
		t.Fatalf("NewSQLite(%q): %v", path, err)
	}
	defer db.Close()
	assertPragmaValues(t, db)
	assertMainFile(t, db, path)
}

// B｜相對路徑——**live 現況就是這種**，只測 A 會漏掉 file://../.. 把 `..` 當 host 的 bug。
//
// ⛔ 這條與它的 parent 都不得呼叫 t.Parallel()：t.Chdir 改的是**行程層級**的
// 工作目錄，放在 parallel test 下會 panic 或讓結果不穩定。
func TestNewSQLiteAppliesPragmasRelativePath(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	workDir := filepath.Join(root, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workDir)

	rel := filepath.Join("..", "data", trickySQLiteFileName)
	db, err := NewSQLite(rel)
	if err != nil {
		t.Fatalf("NewSQLite(%q): %v", rel, err)
	}
	defer db.Close()
	assertPragmaValues(t, db)
	wantAbs, err := filepath.Abs(rel)
	if err != nil {
		t.Fatal(err)
	}
	assertMainFile(t, db, wantAbs)
}

// C｜memory DSN 三種。記憶體 DB 沒有檔案路徑，⛔ 不驗 database_list。
func TestNewSQLiteAppliesPragmasMemory(t *testing.T) {
	t.Run("bare-memory", func(t *testing.T) {
		db, err := NewSQLite(":memory:")
		if err != nil {
			t.Fatalf("NewSQLite(:memory:): %v", err)
		}
		defer db.Close()
		assertPragmaValues(t, db)
	})

	// ⛔ cache=shared 要用**雙 handle** 證明真的生效：只斷言「query 字串裡還有
	// cache=shared」證明不了參數有作用。第一個 handle 建表寫入、第二個讀得到，
	// 才代表它們共用同一塊記憶體 DB。
	shared := []struct{ name, dsn string }{
		{"file-memory-shared", "file::memory:?cache=shared"},
		// ⚠️ named memory 的名稱要**每個測試唯一**，否則不同測試會共用同一塊
		// 記憶體 DB 互相污染。
		{"named-memory-shared", "file:t071-named-shared?mode=memory&cache=shared"},
	}
	for _, c := range shared {
		t.Run(c.name, func(t *testing.T) {
			first, err := NewSQLite(c.dsn)
			if err != nil {
				t.Fatalf("NewSQLite(%q): %v", c.dsn, err)
			}
			defer first.Close()
			assertPragmaValues(t, first)

			if _, err := first.Exec(`CREATE TABLE shared_probe(v INTEGER)`); err != nil {
				t.Fatalf("建表: %v", err)
			}
			if _, err := first.Exec(`INSERT INTO shared_probe VALUES (42)`); err != nil {
				t.Fatalf("寫入: %v", err)
			}

			second, err := NewSQLite(c.dsn)
			if err != nil {
				t.Fatalf("第二個 handle: %v", err)
			}
			defer second.Close()
			var v int
			if err := second.Get(&v, `SELECT v FROM shared_probe`); err != nil {
				t.Fatalf("cache=shared 未生效，第二個 handle 讀不到第一個寫的資料: %v", err)
			}
			if v != 42 {
				t.Errorf("讀到 %d，期望 42", v)
			}
			assertPragmaValues(t, second)
		})
	}
}

// ── #20f3：physical connection 重建後，connection-local pragma 仍生效 ───────
//
// ⛔ 四個步驟缺一不可，特別是「保留同一個 pool」——若關掉整個 pool 再 NewSQLite
// 一次，即使用被禁止的「每個 pool Exec 一次」也會通過，這條就測不到東西。
func TestNewSQLitePragmasSurviveConnectionRecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recycle.db")
	db, err := NewSQLite(path)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	defer db.Close()

	assertPragmaValues(t, db)

	// ②③ 強制丟棄既有的 physical connection，讓 pool 自動建立下一條。
	// SetMaxIdleConns(0) 會讓歸還的 connection 直接關閉而不是留在 idle pool。
	db.SetMaxIdleConns(0)
	if _, err := db.Exec(`SELECT 1`); err != nil {
		t.Fatalf("觸發連線回收: %v", err)
	}
	db.SetMaxIdleConns(2)

	// ④ 在**新的** connection 上再查一次——兩個 pragma 都要還在。
	// 只驗 busy_timeout 會漏掉 FK 靜默失效，那會讓所有 FK 契約整組落空。
	assertPragmaValues(t, db)
}
