#!/usr/bin/env python3
"""docs/*.md 裡的原始碼引用是否仍指向正確位置。

⚠️ **要解的問題**：這是**每次改動原始碼就會再發生一次**的漂移。2026-09-17 一次掃描
就在 144 個引用裡找到 **16 處**已經指錯，其中 `evaluation.py` 的 `_atr_pct()` 是
`issue.md` I-107「兩個 ATR 公式」對照表的**核心證據**，文件寫的行號停在 287、實際已漂到 327。
連續三輪 review 都在抓同一類問題。

⛔ **這支工具⛔ 不得 fail-open**（2026-09-17 第一版就犯過，review 抓到）：
初版只檢查「識別符緊鄰行號」的 17 個引用，卻對其餘 125 個一字不提就印
「全部對得上」；識別符**整份檔案都不存在**時反而靜默通過（只有「檔案其他處有」才報）。
守門工具謊報通過比沒有工具更糟——它讓人以為檢查過了。

現在每個引用都會落進下列其中一格，⚠️ **而且數量會逐格印出來**：

| 結果 | 意義 |
|---|---|
| `checked` | 檔案解析成功 ＋ 行號在範圍內 ＋（若是緊鄰格式）識別符位置也對 |
| `external` | 檔名列在 `EXTERNAL_ALLOWLIST` 裡——引用外部套件的行號是刻意的，跳過 |
| `line-only` | 檔案與行號驗過了，但**沒有緊鄰的識別符**可供比對位置。⚠️ 只擋得住「檔案消失／行號超出／區間不合法」，而且**必須在 `doc-refs-legacy.txt` 的額度內**（含數量） |
| **失敗** | 行號超出檔案長度／區間終點不合法／識別符不在該位置／識別符**整份檔案都沒有**／**同名檔案有多個**／**repo 內找不到且不在 allowlist**／**新增的 line-only 超出基準線額度**／**基準線有殘留份額** |

**四種「識別符緊鄰行號」的合法寫法**（⛔ 散在一行各處的識別符不算——
那會把 `BEGIN`、`raise`、`MustExec` 這種字誤判成漂移）：

    `file.py:123` `ident`          `ident`（`file.py:123`）
    `file.py:123` 的 `ident`        `ident()`（`file.py:123`）

⚠️ **兩種引用意圖分開判**：識別符在該檔案有 `def`／`func`／`class` 定義時，
引用只要落在**該函式範圍內**就算對（很多引用指的是函式內部某段，⛔ 不是定義行）；
沒有定義時則要求它出現在行號的 ±WINDOW 內。

⚠️ **引用「歷史 commit 的行號」時⛔ 不要寫成 `檔名:行號` 的形式**：本工具一律拿
**當前工作樹**的檔案驗，⛔ 驗不了 `ecbc141^` 那種舊版本的行號，只會報成漂移。
改寫成「`<commit>` 的 `檔名` 第 N-M 行」即可。

⚠️ **要在文件裡「引述一個寫錯的引用」時，⛔ 不要寫成上面的合法形式**——
工具分不出「這是引用」與「這是在講某個引用曾經寫錯」。把行號與識別符拆開敘述即可。

用法：
    python3 scripts/check-doc-refs.py            # 檢查，有問題回非零
    python3 scripts/check-doc-refs.py --list     # 連同通過的引用一起列出
    python3 scripts/check-doc-refs.py --root DIR # 改對 DIR 底下的 docs/ 與原始碼跑

⚠️ `--root` 是**正式參數**（`scripts/test-doc-refs.sh` 用它對 fixture 跑迴歸測試），
⛔ 不是只給測試用的後門——它不繞過任何檢查，只是換一個 repo 根目錄。
"""
from __future__ import annotations

import pathlib
import re
import sys
from collections import Counter

WINDOW = 15
ROOT = pathlib.Path(__file__).resolve().parent.parent
SRC_EXT = r"(?:py|go|ts|sh|sql|svelte)"

# 識別符與行號「綁在一起」的四種寫法。⛔ 不要放寬成「同一行出現過就算」。
NEAR_PATTERNS = [
    re.compile(rf'`(?P<file>[A-Za-z0-9_/.-]+\.{SRC_EXT}):(?P<line>\d+)(?:-(?P<end>\d+))?`\s*(?:的\s*)?`(?P<ident>[A-Za-z_][A-Za-z0-9_]*)(?:\(\))?`'),
    re.compile(rf'`(?P<ident>[A-Za-z_][A-Za-z0-9_]*)(?:\(\))?`\s*[（(]\s*`(?P<file>[A-Za-z0-9_/.-]+\.{SRC_EXT}):(?P<line>\d+)(?:-(?P<end>\d+))?`'),
]
# 所有帶行號的引用（用來確保「沒有任何一個被靜默略過」）。
ANY_REF = re.compile(rf'`(?P<file>[A-Za-z0-9_/.-]+\.{SRC_EXT}):(?P<line>\d+)(?:-(?P<end>\d+))?`')
DEF_RE = "(?:def|func|class)"

# ⛔ **repo 內找不到的檔案預設是「錯」，⛔ 不是「外部套件」**（2026-09-17 review）：
# 初版把找不到的一律當外部依賴略過，於是**檔名拼錯或檔案被刪掉都會靜默通過**。
# 要略過就得列在這裡，而且要寫清楚為什麼。
# ⚠️ **line-only 的既有引用要凍結成基準線**（2026-09-17 review）：
# 沒有它的話，`line-only` 只是個會愈長愈大的計數器——新引用照樣能用不受支援的格式進來，
# CI 只會把 122 變成 123。⛔ **基準線只准變短，⛔ 不准變長。**
BASELINE_FILE = "scripts/doc-refs-legacy.txt"

EXTERNAL_ALLOWLIST = {
    # go-sql-driver/mysql 的原始碼——`v.In(cfg.Loc)` 的寫入時區轉換（issue.md I-0xx）。
    "connection.go": "go-sql-driver/mysql",
    # jackc/pgtype 的原始碼——DATE 的解析行為。
    "pgtype/date.go": "jackc/pgtype",
    # ⚠️ 這⛔ 不是引用，是 development-workflow.md 裡**示範合法格式**用的假檔名。
    "file.py": "文件裡的格式示範，非真實引用",
}

_cache: dict[str, tuple[str, list[str] | list[str] | None]] = {}


def resolve(name: str):
    """回傳 (status, payload)。

    ⛔ **三種結果必須分開**（2026-09-17 review）：初版把「同名檔案有多個」和
    「檔案不在 repo 內」都回 None，於是 `scheduler.go` 這種常見檔名的 5 個引用
    被當成外部依賴靜默跳過。

      ("ok", 原始碼行)      唯一的本地檔案
      ("ambiguous", 候選)   多個同名檔案——⛔ 失敗，要求文件改用 repo-relative path
      ("external", None)    repo 內找不到——引用外部套件，跳過
    """
    if name in _cache:
        return _cache[name]
    # ⛔ **先看原始字串，⛔ 不要等組合成路徑才驗**（2026-09-21 review）：
    # `ROOT / "/abs/path"` 在 pathlib 裡**會直接變成 `/abs/path`**（絕對路徑吃掉左邊），
    # 而 `scripts/../scripts/x.py` 解析後仍在 repo 內，兩者都會通過 containment 檢查。
    # ⚠️ 文件引用必須是**可攜的 repo-relative 路徑**——絕對路徑與 `..` 一律不收。
    if pathlib.PurePath(name).is_absolute() or ".." in pathlib.PurePath(name).parts:
        _cache[name] = ("missing", None)
        return _cache[name]
    p = ROOT / name
    if _is_acceptable_local_file(p):
        out = ("ok", p.read_text(encoding="utf-8", errors="replace").splitlines())
    else:
        # ⚠️ **用「路徑後綴」比對，⛔ 不是只比檔名**：文件常寫成
        # `handler/scheduler.go`、`store/model.go`、`migrations/postgres/067_x.sql`，
        # 只比 basename 會把這些判成歧義（repo 裡 scheduler.go / model.go 各有兩個），
        # 於是**帶了足夠路徑的正確引用反而被擋下來**。
        base = name.split("/")[-1]
        # ⚠️ **⛔ 不要用 `ROOT.glob("**/…")`**：它會**先遞迴走進** `.git/worktrees`
        # 之類的目錄、之後才過濾——而臨時 git worktree 隨時可能被移除，
        # 走訪途中目錄消失就丟 `FileNotFoundError`，造成偶發假紅。
        # 改成自己走訪並在**遞迴之前**就剪掉，順便容忍走訪期間消失的路徑。
        hits = sorted(_find_by_basename(base, name))
        if len(hits) == 1:
            out = ("ok", hits[0].read_text(encoding="utf-8", errors="replace").splitlines())
        elif hits:
            out = ("ambiguous", [str(h.relative_to(ROOT)) for h in hits])
        elif name in EXTERNAL_ALLOWLIST:
            out = ("external", EXTERNAL_ALLOWLIST[name])
        else:
            out = ("missing", None)
    _cache[name] = out
    return out


_PRUNE_DIRS = {".git", "node_modules", "__pycache__", ".venv", "venv", ".mypy_cache"}


def _is_acceptable_local_file(path: pathlib.Path) -> bool:
    """直接路徑（`ROOT / name`）也要通過與 walker **同一套**檢查。

    ⛔ **⛔ 不能只寫 `p.is_file()`**（2026-09-21 review）：`is_file()` 會**跟隨 symlink**，
    而且 `name` 可能帶 `..` 或絕對路徑。於是 `linked_out/pkg/a/sample.py`、
    `../outside/sample.py`、甚至 `.git/…` 都會**直接命中、完全繞過 walker 的防線**。
    三道都要驗：

    * `resolve()` 之後必須**仍在 ROOT 內**；
    * 路徑元件⛔ 不得包含 `_PRUNE_DIRS`（`.git`、`node_modules`…）；
    * **任何一段是 symlink 都拒絕**——與 walker 的「一律不跟隨」同一個契約。
    """
    try:
        if not path.is_file():
            return False
        # ⚠️ 逐段檢查 symlink：⛔ 只看最後一段不夠，中間目錄是 symlink 一樣能跳出 ROOT。
        probe = ROOT
        for part in path.relative_to(ROOT).parts if _under_root_lexically(path) else []:
            probe = probe / part
            if probe.is_symlink():
                return False
        resolved = path.resolve()
        if not resolved.is_relative_to(ROOT.resolve()):
            return False
        if any(part in _PRUNE_DIRS for part in resolved.relative_to(ROOT.resolve()).parts):
            return False
        return True
    except (OSError, ValueError):
        return False


def _under_root_lexically(path: pathlib.Path) -> bool:
    try:
        path.relative_to(ROOT)
        return True
    except ValueError:
        return False


def _find_by_basename(base: str, suffix: str) -> list[pathlib.Path]:
    """走訪 repo 找出 basename 相符、且路徑以 `suffix` 結尾的檔案。

    ⚠️ **在遞迴之前剪掉 `.git` 等目錄**，⛔ 不是先全走一遍再過濾——
    臨時 worktree 在走訪途中被移除會丟 `FileNotFoundError`。

    ⛔ **一律跳過 symlink**（2026-09-21 review）：`is_dir()` 會**跟隨** symlink，
    於是一條指向 repo 外的目錄 symlink 就能把外部檔案掃進來當成「本地引用」，
    symlink cycle 還會讓走訪重複甚至打轉。⚠️ 判斷用 `is_symlink()` 先擋，
    ⛔ 不要依賴 `is_dir()`／`is_file()`。

    ⛔ **只容忍「競爭造成的消失」**：`FileNotFoundError`／`NotADirectoryError` 跳過；
    `PermissionError` 與其他 `OSError` **一律往上拋**——⚠️ 讀不到目錄代表**候選可能漏掉**，
    而漏掉候選會讓原本該判 `ambiguous` 的引用**靜默通過**，那是 fail-open。
    """
    found: list[pathlib.Path] = []
    stack = [ROOT]
    while stack:
        current = stack.pop()
        try:
            entries = list(current.iterdir())
        except (FileNotFoundError, NotADirectoryError):
            continue            # ⚠️ 走訪期間消失——⛔ 這一種才可以跳過
        for entry in entries:
            try:
                if entry.is_symlink():
                    continue    # ⛔ 不跟隨：可能指向 repo 外，或形成 cycle
                if entry.is_dir():
                    if entry.name not in _PRUNE_DIRS:
                        stack.append(entry)
                elif entry.name == base and str(entry.relative_to(ROOT)).endswith(suffix):
                    found.append(entry)
            except (FileNotFoundError, NotADirectoryError):
                continue
    return found


def _def_line(src: list[str], ident: str) -> int | None:
    for n, line in enumerate(src, 1):
        if re.search(rf"^\s*{DEF_RE}\s+(?:\([^)]*\)\s*)?{re.escape(ident)}\b", line):
            return n
    return None


def load_baseline() -> "Counter[str]":
    """⚠️ **要記數量，⛔ 不能只記 key**（2026-09-17 review）：
    同一個 `docs檔|來源檔:行號` 可能在同一份文件裡出現多次
    （`issue.md|selection_report.py:717` 就有 4 處）。只用 set 的話，
    **再新增第 5 處照樣會綠**——「不准新增」就形同虛設。

    格式：`<key>` 或 `<key> x<次數>`。
    """
    counter: Counter[str] = Counter()
    f = ROOT / BASELINE_FILE
    if not f.is_file():
        return counter
    # ⛔ **額度必須是正整數**：`x0`／`x-1`／`xabc` 這種寫法⛔ 不得靜默接受，
    # 否則基準線可以寫出「存在但不生效」的項目，讀的人完全看不出來。
    entry = re.compile(r"^(?P<key>\S+)(?: x(?P<n>-?\d+))?$")
    for lineno, line in enumerate(f.read_text(encoding="utf-8").splitlines(), 1):
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        m = entry.match(line)
        if not m:
            raise ValueError(f"{BASELINE_FILE}:{lineno} 格式不合法：{line!r}"
                             "（預期 `<key>` 或 `<key> x<正整數>`）")
        n = int(m.group("n")) if m.group("n") else 1
        if n < 1:
            raise ValueError(f"{BASELINE_FILE}:{lineno} 額度必須 >= 1，實際是 {n}")
        counter[m.group("key")] += n
    return counter


def check():
    problems, stats, passed = [], {"checked": 0, "line-only": 0, "external": 0}, []
    baseline = load_baseline()
    used: Counter[str] = Counter()
    for md in sorted((ROOT / "docs").glob("*.md")):
        for lineno, text in enumerate(md.read_text(encoding="utf-8").splitlines(), 1):
            # ⚠️ **識別符要綁在「這一個 match 的文字位置」上，⛔ 不是綁在座標上**
            # （2026-09-17 review 兩輪）：
            #   * 只用 (檔名, 起點) → 同一行的 `f.py:4-8` `alpha` 與 `f.py:4-9` `beta`
            #     共用第一個識別符；
            #   * 改成 (檔名, 起點, 終點) 仍不夠 → **座標完全相同**的
            #     `f.py:4-8` `alpha` 與 `f.py:4-8` `deleted_func` 還是共用第一個。
            # 所以記下每個 near match 的 span，配對時**用掉就移除**，
            # 讓第 N 個引用拿到第 N 個識別符。
            near_spans: list[tuple[int, int, tuple[str, int, int | None], str]] = []
            for pat in NEAR_PATTERNS:
                for m in pat.finditer(text):
                    end = int(m.group("end")) if m.group("end") else None
                    near_spans.append((m.start(), m.end(),
                                       (m.group("file"), int(m.group("line")), end),
                                       m.group("ident")))
            near_spans.sort()
            # ⛔ **⛔ 不對 occurrence 去重**（2026-09-17 review）：去重會讓同一行寫兩次的
            # 相同 line-only 引用**只計一次**，於是 `x1` 的額度擋不住第二筆——
            # 與「數量也算數」的契約直接矛盾。`ANY_REF` 是單一 pattern，
            # ⛔ 不會對同一段文字重複匹配，本來就不需要去重。
            for m in ANY_REF.finditer(text):
                name, want = m.group("file"), int(m.group("line"))
                end_line = int(m.group("end")) if m.group("end") else None
                status, payload = resolve(name)
                where = f"docs/{md.name}:{lineno}"
                if status == "external":
                    stats["external"] += 1
                    continue
                if status == "missing":
                    problems.append((where, name, want, "",
                                     "repo 內找不到這個檔案——⛔ 拼錯或已刪除？"
                                     "確定是外部套件的話要加進 EXTERNAL_ALLOWLIST 並註明來源"))
                    continue
                if status == "ambiguous":
                    problems.append((where, name, want, "",
                                     f"repo 內有 {len(payload)} 個同名檔案，⛔ 請改用 repo-relative path："
                                     + "、".join(payload[:4])))
                    continue
                src = payload
                if want > len(src):
                    problems.append((where, name, want, "",
                                     f"行號超出檔案長度（{len(src)} 行）"))
                    continue
                # ⚠️ **區間的終點也要驗**——初版把 `-456` 丟掉只比起點，
                # 於是 `10-99999` 這種指到檔案外的區間照樣通過。
                if end_line is not None:
                    if end_line < want:
                        problems.append((where, name, want, "",
                                         f"區間終點 {end_line} 小於起點 {want}"))
                        continue
                    if end_line > len(src):
                        problems.append((where, name, want, "",
                                         f"區間終點 {end_line} 超出檔案長度（{len(src)} 行）"))
                        continue
                ident = None
                for idx, (ns, ne, nkey, nident) in enumerate(near_spans):
                    if nkey == (name, want, end_line) and ns <= m.start() and m.end() <= ne:
                        ident = nident
                        del near_spans[idx]      # ⚠️ 用掉就移除——⛔ 不讓下一筆重複配到
                        break
                if ident is None:
                    key = f"{md.name}|{name}:{want}"
                    used[key] += 1
                    if used[key] <= baseline[key]:
                        stats["line-only"] += 1
                        passed.append((where, name, want, "", "line-only（在 legacy 基準線內）"))
                    else:
                        problems.append((where, name, want, "",
                                         f"新的 line-only 引用（這是本檔第 {used[key]} 筆，"
                                         f"基準線只認 {baseline[key]} 筆）"
                                         "——⛔ 請改成「識別符緊鄰行號」的格式"))
                    continue
                # ⚠️ **用識別符邊界比對，⛔ 不是子字串**：`foo` 會被 `foobar` 誤命中。
                word = re.compile(rf"\b{re.escape(ident)}\b")
                whole = "\n".join(src)
                lo, hi = max(0, want - 1 - WINDOW), min(len(src), want + WINDOW)
                near_text = "\n".join(src[lo:hi])
                def_at = _def_line(src, ident)
                # ⚠️ **引用「呼叫處」也是合法的**——識別符就出現在該行號附近時直接通過，
                # ⛔ 不要因為「它的定義在別的地方」就判成漂移（`runIntradayJob` 轉給
                # `runIntradayBatch` 的那一行就是這種）。定義範圍只是**備援判準**。
                if word.search(near_text):
                    ok, why = True, "ok（識別符就在該行號附近）"
                elif def_at is not None:
                    end = next((n for n, l in enumerate(src, 1)
                                if n > def_at and re.match(rf"^\s*{DEF_RE}\s", l)), len(src) + 1)
                    ok = def_at - WINDOW <= want < end
                    why = f"定義在第 {def_at} 行，函式範圍 {def_at}~{end - 1}"
                elif not word.search(whole):
                    # ⛔ **整份檔案都沒有這個識別符 → 一定要失敗**。初版只在
                    # 「檔案其他處有」時才報，於是識別符被刪掉反而靜默通過。
                    ok, why = False, "該識別符已不在這個檔案裡"
                else:
                    ok = False
                    why = "該行號附近找不到它（檔案其他處有）"
                if ok:
                    stats["checked"] += 1
                    passed.append((where, name, want, ident, "ok"))
                else:
                    problems.append((where, name, want, ident, why))
    # ⚠️ 基準線裡多出來的份額要報出來——⛔ 不然它會永遠留著，變成沒人維護的清單。
    stale = sorted(
        f"{k} x{baseline[k] - used[k]}" if baseline[k] - used[k] > 1 else k
        for k in baseline if baseline[k] > used[k]
    )
    return problems, stats, passed, stale


def main(argv: list[str]) -> int:
    global ROOT
    show_list = False
    rest = list(argv)
    while rest:
        arg = rest.pop(0)
        if arg == "--list":
            show_list = True
        elif arg == "--root":
            if not rest:
                print("ERROR: --root 需要一個目錄", file=sys.stderr)
                return 2
            ROOT = pathlib.Path(rest.pop(0)).resolve()
            _cache.clear()
        else:
            print(f"ERROR: 未知參數 {arg}（只接受 --list / --root DIR）", file=sys.stderr)
            return 2
    try:
        problems, stats, passed, stale = check()
    except ValueError as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 2
    total = stats["checked"] + stats["line-only"] + stats["external"] + len(problems)
    summary = (f"引用共 {total} 個："
               f"識別符已驗 {stats['checked']}、"
               f"僅驗檔案與行號 {stats['line-only']}、"
               f"外部套件略過 {stats['external']}、"
               f"問題 {len(problems)}")
    if stale:
        # ⛔ **這是失敗，⛔ 不是警告**（2026-09-17 review）：只警告卻回 0 的話，
        # 「基準線只准變短」就沒有任何東西在執行——殘留會一直留著。
        print(f"⛔ {BASELINE_FILE} 有 {len(stale)} 筆已不存在（引用被改掉或刪掉了）——"
              f"請從基準線移除，⛔ 基準線只准變短：", file=sys.stderr)
        for k in stale[:10]:
            print(f"    {k}", file=sys.stderr)
    if show_list:
        for where, name, want, ident, why in passed:
            print(f"  {where}  `{name}:{want}`" + (f" `{ident}`" if ident else "") + f" —— {why}")
    if not problems and not stale:
        print(f"==> check-doc-refs：{summary}")
        return 0
    if not problems:
        return 1
    print(f"==> check-doc-refs：{summary}", file=sys.stderr)
    for where, name, want, ident, why in problems:
        print(f"  {where}  `{name}:{want}`" + (f" `{ident}`" if ident else "") + f" —— {why}",
              file=sys.stderr)
    print("\n⛔ 改完原始碼要回頭更新文件引用；⚠️ 只指向函式定義的引用，"
          "改寫成不帶行號的 `檔名` 的 `函式()` 就不會再漂。", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
