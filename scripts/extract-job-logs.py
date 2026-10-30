#!/usr/bin/env python3
"""I-122 的 E2：取出 `job_runs` 時間窗內的 backend log，遮罩並驗證之後才寫檔。

規格：`docs/plans/i122-live-batch-failures.md`「三」E2（順序寫死的 1～7）與「十二」實作備註。

⚠️ **這支工具會在記憶體裡經手含 secret 的原始 log 與容器的環境變數**，所以：

* 磁碟上只會出現「遮罩 ＋ 序列化 ＋ L3 全部通過」之後的輸出；任何一步失敗都⛔ 不寫任何檔（連暫存檔也沒有）。
* 失敗時只印類別與計數（來源的序號與檔名、行號、紀錄 id、鍵名），⛔ 不印 log 原文、環境變數的值或例外訊息
  （例外物件的訊息可能帶原文，只印型別名稱）。
* 來源由工具自己逐一以 `docker exec … cat` 讀取並檢查結束碼；⛔ 不經 shell 管線，結束碼非零就丟掉已讀到的部分。
* L3 有兩道：原文裡每一處已知值（含變體）都必須**完整**落在遮罩範圍內（只蓋住一部分也算失敗）；
  輸出與報告的 bytes 裡⛔ 沒有已知值。只證明「⛔ 沒有發現 5 個 secret 環境變數**目前**的值及其指定變體」；
  ⛔ 不涵蓋已輪替的舊值、非環境變數的憑證與其他編碼（計畫書「三」E2 的保證範圍）。

用法（使用者在自己的終端機執行；Claude ⛔ 不對 live 執行）：
    python3 -I scripts/extract-job-logs.py --windows "$EV/e1_job_runs.csv" \\
        --out "$EV/e2_logs_masked.jsonl" --report "$EV/e2_report.txt"

迴歸測試：`scripts/test-extract-job-logs.sh`（fake docker，⛔ 不碰 live）。
"""
from __future__ import annotations

import argparse
import csv
import hashlib
import json
import os
import re
import subprocess
import sys
from collections import Counter, defaultdict
from dataclasses import dataclass
from datetime import date, datetime, timedelta, timezone
from urllib.parse import quote, quote_plus, unquote, urlsplit

CONTAINER = "stock_trading-backend-1"
LOG_DIR = "/app/logs/backend"
MARGIN = timedelta(seconds=5)
TPE = timezone(timedelta(hours=8))   # 台北⛔ 沒有夏令時間，固定 +08:00
UTC = timezone.utc
SECRET_KEYS = ("AUTH_JWT_SECRET", "DATABASE_DSN", "FINMIND_API_KEY", "FUGLE_API_KEY", "REDIS_PASSWORD")
# E1 的 `\copy` 輸出的欄位（計畫書「三」E1）。⛔ 不放寬：表頭不符代表不是那份查詢的結果。
CSV_HEADER = ["id", "job_name", "status", "symbols_total", "symbols_failed", "started_tpe", "finished_tpe", "error"]
MASK = "<masked>"


class Abort(Exception):
    """可以安全印出的失敗：訊息只由本檔的固定字串、計數、日期、行號、id 與鍵名組成。"""


# ── 時間窗 ──────────────────────────────────────────────────────────────

@dataclass(frozen=True)
class Run:
    id: int
    job: str
    status: str
    start: datetime
    end: datetime


@dataclass(frozen=True)
class Window:
    run: Run
    kind: str          # 「目標」＝非 success；「對照」＝同工作同日的一筆 success

    @property
    def lo(self) -> datetime:
        return self.run.start - MARGIN

    @property
    def hi(self) -> datetime:
        return self.run.end + MARGIN


_TPE_TS = re.compile(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}")
_NAME = re.compile(r"[a-z][a-z0-9_]*")


def _parse_tpe(s: str) -> datetime | None:
    if not _TPE_TS.fullmatch(s):
        return None
    try:
        return datetime.strptime(s, "%Y-%m-%dT%H:%M:%S.%f").replace(tzinfo=TPE)
    except ValueError:
        return None


def load_runs(path: str) -> list[Run]:
    try:
        with open(path, newline="", encoding="utf-8") as f:
            rows = list(csv.reader(f))
    except OSError:
        raise Abort("讀不到 --windows 的檔案") from None
    except (UnicodeDecodeError, csv.Error):
        raise Abort("--windows 不是合法的 UTF-8 CSV") from None
    if not rows or rows[0] != CSV_HEADER:
        raise Abort("--windows 的表頭與 E1 的欄位不符")
    runs, seen = [], set()
    for n, r in enumerate(rows[1:], start=2):
        if len(r) != len(CSV_HEADER):
            raise Abort(f"--windows 第 {n} 行的欄位數不符")
        rid, job, status, _total, _failed, s, e, _error = r
        if not rid.isdigit() or not _NAME.fullmatch(job) or not _NAME.fullmatch(status):
            raise Abort(f"--windows 第 {n} 行的 id／job_name／status 格式不符")
        if status == "running" or not e:
            raise Abort(f"--windows 第 {n} 行（id={rid}）沒有結束時間（還在 running？請避開批次時段重跑 E1 與 E2）")
        start, end = _parse_tpe(s), _parse_tpe(e)
        if start is None or end is None:
            raise Abort(f"--windows 第 {n} 行（id={rid}）的時間格式不符")
        if end < start:
            raise Abort(f"--windows 第 {n} 行（id={rid}）的結束時間早於開始時間")
        if int(rid) in seen:
            raise Abort(f"--windows 第 {n} 行（id={rid}）的 id 重複")
        seen.add(int(rid))
        runs.append(Run(int(rid), job, status, start, end))
    return runs


def select_windows(runs: list[Run]) -> list[Window]:
    """非 success 的每一筆是「目標」；每個工作、每個台北日期另取一筆 success 當「對照」。

    對照的選法（計畫書「十二」）：當天有非 success 時，取起跑時間最接近當天最早一筆非 success 的；
    沒有時取當天最早的一筆；當天沒有 success 就不取。
    """
    wins = [Window(r, "目標") for r in runs if r.status != "success"]
    groups: dict[tuple[str, date], list[Run]] = defaultdict(list)
    for r in runs:
        groups[(r.job, r.start.astimezone(TPE).date())].append(r)
    for key in sorted(groups):
        succ = [r for r in groups[key] if r.status == "success"]
        if not succ:
            continue
        bad = [r for r in groups[key] if r.status != "success"]
        if bad:
            ref = min(r.start for r in bad)
            pick = min(succ, key=lambda r: (abs(r.start - ref), r.start, r.id))
        else:
            pick = min(succ, key=lambda r: (r.start, r.id))
        wins.append(Window(pick, "對照"))
    if not wins:
        raise Abort("--windows 推不出任何時間窗")
    wins.sort(key=lambda w: (w.run.start, w.run.id))
    return wins


def source_dates(wins: list[Window]) -> list[date]:
    """log 檔名是 UTC 日期：每個時間窗涵蓋的每一個 UTC 日期都要讀（含跨日的窗）。"""
    ds = set()
    for w in wins:
        d, last = w.lo.astimezone(UTC).date(), w.hi.astimezone(UTC).date()
        while d <= last:
            ds.add(d)
            d += timedelta(days=1)
    return sorted(ds)


# ── 1、2：讀取來源並過濾 ────────────────────────────────────────────────

_LOG_TS = re.compile(r"(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d{1,9}))?(Z|[+-]\d{2}(?::?\d{2})?)")


def parse_ts(v: object) -> datetime | None:
    """zap 的 ISO8601TimeEncoder（`2006-01-02T15:04:05.000Z0700`），時區依容器而定，一律帶 offset。"""
    if not isinstance(v, str):
        return None
    m = _LOG_TS.fullmatch(v)
    if not m:
        return None
    y, mo, d, h, mi, s, frac, off = m.groups()
    try:
        if off == "Z":
            tz = UTC
        else:
            digits = off[1:].replace(":", "")
            delta = timedelta(hours=int(digits[:2]), minutes=int(digits[2:4] or 0))
            tz = timezone(delta if off[0] == "+" else -delta)
        us = int((frac or "0").ljust(6, "0")[:6])
        return datetime(int(y), int(mo), int(d), int(h), int(mi), int(s), us, tzinfo=tz)
    except ValueError:
        return None


@dataclass
class SourceStat:
    name: str
    total: int = 0
    taken: int = 0
    tail_complete: int = 0      # 末段沒有換行、但是完整的一行（照常處理）
    tail_dropped: int = 0       # 末段沒有換行、也不完整（只有正在寫入的當天檔可以丟掉）


def read_sources(dates: list[date], wins: list[Window]) -> tuple[list[tuple[str, int, dict, list[int]]], list[SourceStat]]:
    """逐一讀每個 UTC 日期的檔，邊讀邊過濾：只有時間窗內的行留在記憶體（host 只有 2 GiB）。

    ⚠️ 結束碼在讀完之後才知道：非零就中止，已讀到的部分⛔ 不採用（下游⛔ 不會拿到「部分成功」的內容）。
    """
    kept: list[tuple[str, int, dict, list[int]]] = []
    stats: list[SourceStat] = []
    for k, d in enumerate(dates, start=1):
        name = f"backend-{d.isoformat()}.log"
        cand = [w for w in wins if w.lo.astimezone(UTC).date() <= d <= w.hi.astimezone(UTC).date()]
        st = SourceStat(name)
        # ⚠️ 「正在寫入」以讀取當下的 UTC 日期判斷（backend 依寫入當下的 UTC 日期選檔）；
        # ⛔ 不能用「查詢範圍的最後一天」——那一天可能早就是歷史檔（review 2026-10-30）。
        live = d == datetime.now(UTC).date()
        try:
            proc = subprocess.Popen(["docker", "exec", CONTAINER, "cat", f"{LOG_DIR}/{name}"],
                                    stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
        except OSError:
            raise Abort("無法執行 docker") from None
        try:
            assert proc.stdout is not None
            for n, raw in enumerate(proc.stdout, start=1):
                if raw.endswith(b"\n"):
                    obj, ts = _parse_line(raw, k, name, n)
                else:
                    # 末段沒有換行：能完整解析就是完整的一行，照常處理（zap 的一行只有最後一個 `}` 在頂層）；
                    # 不完整的只有正在寫入的當天檔可以丟掉，歷史檔 → 中止。
                    try:
                        obj, ts = _parse_line(raw, k, name, n)
                    except Abort:
                        if not live:
                            raise Abort(f"來源 {k}（{name}）第 {n} 行沒有換行結尾而且不完整"
                                        "（只有正在寫入的當天檔可以）") from None
                        st.tail_dropped += 1
                        continue
                    st.tail_complete += 1
                st.total += 1
                ids = [w.run.id for w in cand if w.lo <= ts <= w.hi]
                if ids:
                    kept.append((name, n, obj, sorted(set(ids))))
                    st.taken += 1
            rc = proc.wait()
        finally:
            if proc.poll() is None:
                proc.kill()
                proc.wait()
            if proc.stdout is not None:
                proc.stdout.close()
        if rc != 0:
            raise Abort(f"來源 {k}（{name}）讀取失敗（docker exec 的結束碼 {rc}），⛔ 不採用已讀到的部分")
        stats.append(st)
    return kept, stats


def _parse_line(raw: bytes, k: int, name: str, n: int) -> tuple[dict, datetime]:
    try:
        obj = json.loads(raw.decode("utf-8"))
    except (UnicodeDecodeError, ValueError):
        raise Abort(f"來源 {k}（{name}）第 {n} 行不是 UTF-8 的 JSON") from None
    if not isinstance(obj, dict):
        raise Abort(f"來源 {k}（{name}）第 {n} 行不是 JSON 物件")
    ts = parse_ts(obj.get("ts"))
    if ts is None:
        raise Abort(f"來源 {k}（{name}）第 {n} 行的 ts 欄位無法解析")
    return obj, ts


# ── 3：遮罩 ─────────────────────────────────────────────────────────────
#
# ⚠️ **先在原文上算出每一條規則命中的範圍，合併之後才一次換掉**（⛔ 不依序替換）：L3 的覆蓋檢查要知道
# 「原文的哪些位置被遮掉」，才擋得住「遮罩只蓋住 secret 的一部分、破壞了完整值，實值比對反而通過」（review 2026-10-30）。

# 敏感的鍵名（`token=…`、`"token":"…"`、`secret_key: …`）。⚠️ 前面⛔ 不加字界：`access_token`、`jwt_secret` 也要中。
_SENSITIVE = r"(?:token|api[_-]?key|apikey|secret|password|passwd|authorization|bearer|dsn)"
_SENSITIVE_KEY = re.compile(_SENSITIVE, re.IGNORECASE)
# ⚠️ L1、連線字串與 URL 帳密的開頭⛔ 不加字界：`base_https://…` 的 `_` 是字元，加了 `\b` 就漏掉。
_URL_QUERY = re.compile(r"(?i)(?:https?|wss?)://[^?#\s\"'\\<>]*\?([^#\s\"'\\<>]*)")
_CONN = re.compile(r"(?i)(?:postgres(?:ql)?|redis|rediss|mysql|mongodb(?:\+srv)?|amqps?)://[^\s\"'\\<>]+")
_USERINFO = re.compile(r"(?i)[a-z][a-z0-9+.-]*://([^/\s:@\"'\\<>]+:[^/\s@\"'\\<>]*)@")
_JWT = re.compile(r"eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*")
_BEARER = re.compile(r"(?i)\bbearer\s+([^\s\"'\\&,;<>]+)")
# 鍵名 ＋ 分隔符；group 1 是值的開頭引號（`"`、`'`、`\"`、`\'`），沒有引號時是 None。
_KV = re.compile(r"(?i)" + _SENSITIVE + r"[A-Za-z0-9_-]*(?:\\?[\"'])?\s*[:=]\s*(\\?[\"'])?")
_UNQUOTED = re.compile(r"[^\s\"'\\&,;<>]+")
# 長度 ≥ 32、英文與數字都有的長字串（已輪替的舊 token ⛔ 不在 L3 的比對範圍，這一條是它們的防線）。
_LONG = re.compile(r"(?<![A-Za-z0-9+/=_-])[A-Za-z0-9+/=_-]{32,}(?![A-Za-z0-9+/=_-])")
_ALPHA = re.compile(r"[A-Za-z]")
_DIGIT = re.compile(r"[0-9]")
RULE_NAMES = ["L1 URL query", "L2 連線字串", "L2 URL 帳密", "L2 JWT", "L2 bearer", "L2 鍵值", "L2 長字串", "L2 敏感鍵"]
QUERY_MASK = "<query removed>"


def _closing_quote(s: str, start: int, q: str) -> int:
    """引號內的值一路取到對應的收尾引號（含空白、`&`、`,`、`;`）；找不到 → 取到字串結尾（fail-closed）。"""
    i = start
    while True:
        j = s.find(q, i)
        if j < 0:
            return len(s)
        if len(q) == 2:               # `\"`、`\'`：Go 的 %q 裡的引號
            return j
        b = 0                         # 單一引號：前面有奇數個反斜線代表跳脫
        while j - 1 - b >= start and s[j - 1 - b] == "\\":
            b += 1
        if b % 2 == 0:
            return j
        i = j + 1


def _kv_spans(s: str) -> list[tuple[int, int]]:
    out, pos = [], 0
    while True:
        m = _KV.search(s, pos)
        if not m:
            return out
        start, q = m.end(), m.group(1)
        if q:
            end = _closing_quote(s, start, q)
        else:
            u = _UNQUOTED.match(s, start)
            end = u.end() if u else start
        if end > start:
            out.append((start, end))
        # 從值的後面接著找：⛔ 不在值的內部重新起算（值裡剛好有 `token` 之類的字時，會把後面的 `": context` 也當成鍵值）
        pos = max(end, m.end())


def _spans(s: str) -> list[tuple[int, int, str]]:
    """每一條規則在原文 s 上命中的範圍（半開區間）與規則名。"""
    out = []
    for m in _URL_QUERY.finditer(s):
        if m.end(1) > m.start(1):
            out.append((m.start(1), m.end(1), "L1 URL query"))
    for m in _CONN.finditer(s):
        out.append((m.start(), m.end(), "L2 連線字串"))
    for m in _USERINFO.finditer(s):
        out.append((m.start(1), m.end(1), "L2 URL 帳密"))
    for m in _JWT.finditer(s):
        out.append((m.start(), m.end(), "L2 JWT"))
    for m in _BEARER.finditer(s):
        out.append((m.start(1), m.end(1), "L2 bearer"))
    for a, b in _kv_spans(s):
        out.append((a, b, "L2 鍵值"))
    for m in _LONG.finditer(s):
        if _ALPHA.search(m.group(0)) and _DIGIT.search(m.group(0)):
            out.append((m.start(), m.end(), "L2 長字串"))
    return out


def _merge(spans: list[tuple[int, int, str]]) -> list[tuple[int, int]]:
    """重疊或相鄰的範圍合併成一個。"""
    merged: list[list[int]] = []
    for a, b, _ in sorted(spans):
        if merged and a <= merged[-1][1]:
            merged[-1][1] = max(merged[-1][1], b)
        else:
            merged.append([a, b])
    return [(a, b) for a, b in merged]


def mask_str(s: str, counts: Counter) -> str:
    spans = _spans(s)
    if not spans:
        return s
    counts.update(rule for _, _, rule in spans)
    queries = {(a, b) for a, b, rule in spans if rule == "L1 URL query"}
    out, pos = [], 0
    for a, b in _merge(spans):
        out.append(s[pos:a])
        out.append(QUERY_MASK if (a, b) in queries else MASK)
        pos = b
    out.append(s[pos:])
    return "".join(out)


def _masked_whole(key: str, x: object) -> bool:
    """敏感鍵名的值整個換掉（`"token":"…"` 這種結構化欄位，值本身不帶 `token=`）。"""
    return bool(_SENSITIVE_KEY.search(key)) and x is not None and not isinstance(x, bool)


def mask_obj(v: object, counts: Counter) -> object:
    """遞迴遮罩每一個字串（含鍵名、巢狀欄位與 stacktrace）；敏感鍵名的值整個換掉。"""
    if isinstance(v, str):
        return mask_str(v, counts)
    if isinstance(v, list):
        return [mask_obj(x, counts) for x in v]
    if isinstance(v, dict):
        out: dict = {}
        for key, x in v.items():
            mk = mask_str(key, counts)
            while mk in out:          # 遮罩後撞名：⛔ 不覆蓋前一個欄位
                mk += "#"
            if _masked_whole(key, x):
                counts["L2 敏感鍵"] += 1
                out[mk] = MASK
            else:
                out[mk] = mask_obj(x, counts)
        return out
    return v


def masked_strings(v: object):
    """mask_obj 會逐字遮罩的每一個原文字串（鍵名與值）；整個換掉的值⛔ 不會以原文輸出，不列。"""
    if isinstance(v, str):
        yield v
    elif isinstance(v, list):
        for x in v:
            yield from masked_strings(x)
    elif isinstance(v, dict):
        for key, x in v.items():
            yield key
            if not _masked_whole(key, x):
                yield from masked_strings(x)


# ── 4：序列化 ───────────────────────────────────────────────────────────

def serialize(kept: list[tuple[str, int, dict, list[int]]], counts: Counter) -> tuple[bytes, Counter]:
    per_win: Counter = Counter()
    lines = []
    for name, n, obj, ids in kept:
        rec = {"src": name, "line": n, "windows": ids, "entry": mask_obj(obj, counts)}
        try:
            line = json.dumps(rec, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
        except UnicodeEncodeError:      # 單獨的 surrogate：改用 \u 跳脫
            line = json.dumps(rec, separators=(",", ":")).encode("ascii")
        lines.append(line + b"\n")
        per_win.update(ids)
    return b"".join(lines), per_win


# ── 5：L3 實值比對 ───────────────────────────────────────────────────────

def _variants(v: str) -> set[str]:
    """原值、URL 編碼值、JSON 跳脫後的值（計畫書「三」E2 的第 5 步）。"""
    forms = {v, quote(v, safe=""), quote(v), quote_plus(v),
             json.dumps(v)[1:-1], json.dumps(v, ensure_ascii=False)[1:-1]}
    return {f for f in forms if f}


def _dsn_passwords(dsn: str) -> list[str]:
    if "://" in dsn:
        try:
            pw = urlsplit(dsn).password
        except ValueError:
            return []
        return sorted({pw, unquote(pw)}) if pw else []
    m = re.search(r"(?:^|\s)password\s*=\s*('(?:[^'\\]|\\.)*'|[^\s']+)", dsn)
    if not m:
        return []
    raw = m.group(1)
    if raw.startswith("'"):
        raw = re.sub(r"\\(.)", r"\1", raw[1:-1])
    return [raw] if raw else []


def load_secrets() -> tuple[list[tuple[str, set[str]]], list[str]]:
    """在記憶體裡取得 5 個 secret 環境變數目前的值；⛔ 不印、⛔ 不寫檔。

    鍵不存在 → 中止；值是空字串 → 沒有內容可比對，列為「空值、未比對」（計畫書「十二」）。
    """
    try:
        p = subprocess.run(["docker", "container", "inspect", "--format", "{{json .Config.Env}}", CONTAINER],
                           stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    except OSError:
        raise Abort("無法執行 docker") from None
    if p.returncode != 0:
        raise Abort(f"L3：讀不到容器的環境變數（docker container inspect 的結束碼 {p.returncode}）")
    try:
        items = json.loads(p.stdout.decode("utf-8"))
    except (UnicodeDecodeError, ValueError):
        raise Abort("L3：容器環境變數的格式不符") from None
    if not isinstance(items, list) or not all(isinstance(x, str) for x in items):
        raise Abort("L3：容器環境變數的格式不符")
    env: dict[str, str] = {}
    for item in items:
        k, _, v = item.partition("=")
        if k in SECRET_KEYS:
            if k in env:
                raise Abort(f"L3：容器的環境變數 {k} 重複")
            env[k] = v
    missing = [k for k in SECRET_KEYS if k not in env]
    if missing:
        raise Abort("L3：容器的環境變數缺少 " + "、".join(missing))
    checks: list[tuple[str, set[str]]] = []
    empty: list[str] = []
    for k in SECRET_KEYS:
        if env[k] == "":
            empty.append(k)
            continue
        checks.append((k, _variants(env[k])))
        if k == "DATABASE_DSN":
            pws = _dsn_passwords(env[k])
            if not pws:
                raise Abort("L3：DATABASE_DSN 拆不出密碼")
            checks.append(("DATABASE_DSN 的密碼", set().union(*(_variants(pw) for pw in pws))))
    return checks, empty


def l3_gaps(kept: list[tuple[str, int, dict, list[int]]], checks: list[tuple[str, set[str]]]) -> list[str]:
    """原文裡每一處已知值（含變體）都必須完整落在同一個遮罩範圍內；只蓋到一部分、或完全沒蓋到都算缺口。

    ⚠️ 只比對輸出的話，「遮罩只蓋住一部分」會破壞完整值、讓比對通過，殘留的片段卻寫進了檔案（review 2026-10-30）。
    """
    gaps: Counter = Counter()
    for _name, _n, obj, _ids in kept:
        for s in masked_strings(obj):
            merged = None
            for label, forms in checks:
                for f in forms:
                    i = s.find(f)
                    while i >= 0:
                        if merged is None:
                            merged = _merge(_spans(s))
                        j = i + len(f)
                        if not any(a <= i and j <= b for a, b in merged):
                            gaps[label] += 1
                        i = s.find(f, i + 1)
    return [f"{label}（{n} 處）" for label, n in gaps.items()]


def l3_hits(blob: bytes, checks: list[tuple[str, set[str]]]) -> list[str]:
    hits = []
    for label, forms in checks:
        n = sum(blob.count(f.encode("utf-8", "surrogatepass")) for f in forms)
        if n:
            hits.append(f"{label}（{n} 次）")
    return hits


# ── 7：報告（只有計數） ─────────────────────────────────────────────────

def _fmt(t: datetime, tz: timezone) -> str:
    s = t.astimezone(tz).strftime("%Y-%m-%dT%H:%M:%S.%f")[:-3]
    return s + ("Z" if tz is UTC else "+08:00")


def build_report(runs, wins, stats, counts, per_win, checks, empty, out_bytes, n_lines, out_name) -> bytes:
    zero = [w.run.id for w in wins if per_win[w.run.id] == 0]
    r = [
        "extract-job-logs 報告（I-122 E2）",
        f"產生時間：{_fmt(datetime.now(UTC), UTC)}",
        f"容器：{CONTAINER}；log 目錄：{LOG_DIR}；時間窗前後各加 {int(MARGIN.total_seconds())} 秒",
        f"--windows：{len(runs)} 筆紀錄",
        "",
        f"== 時間窗：{len(wins)} 個（目標 {sum(w.kind == '目標' for w in wins)}、對照 {sum(w.kind == '對照' for w in wins)}）==",
    ]
    for w in wins:
        r.append(f"id={w.run.id} {w.run.job} {w.run.status} {w.kind}："
                 f"台北 {_fmt(w.lo, TPE)} ～ {_fmt(w.hi, TPE)}（UTC {_fmt(w.lo, UTC)} ～ {_fmt(w.hi, UTC)}）"
                 f"；log {per_win[w.run.id]} 行")
    r.append("⚠️ 沒有任何 log 行的時間窗：" + ("、".join(f"id={i}" for i in zero) + "（⛔ 不代表沒有錯誤）" if zero else "無"))
    r += ["", f"== 來源：{len(stats)} 個 UTC 日期，全部讀取成功（docker exec 的結束碼都是 0）=="]
    for st in stats:
        r.append(f"{st.name}：總行數 {st.total}、取用 {st.taken}、"
                 f"末段未換行：完整 {st.tail_complete}、不完整而丟棄 {st.tail_dropped}")
    r += ["", "== 遮罩（命中次數；重疊的範圍合併成一個遮罩）=="]
    r += [f"{name}：{counts[name]}" for name in RULE_NAMES]
    r += ["", "== L3 實值比對 ==",
          "比對的項目：" + ("、".join(label for label, _ in checks) or "無"),
          "空值、未比對：" + ("、".join(empty) or "無"),
          "結果：通過（原文裡的每一處都完整落在遮罩範圍內；輸出與本報告都⛔ 沒有發現上列項目的原值、URL 編碼值、JSON 跳脫後的值）",
          "⚠️ 保證範圍：只涵蓋這些環境變數目前的值；⛔ 不涵蓋已輪替的舊值、非環境變數的憑證與其他編碼",
          "", "== 輸出 ==",
          f"{out_name}：{n_lines} 行、{len(out_bytes)} bytes、SHA-256 {hashlib.sha256(out_bytes).hexdigest()}"]
    return ("\n".join(r) + "\n").encode("utf-8")


# ── 6：寫檔 ─────────────────────────────────────────────────────────────

def write_outputs(pairs: list[tuple[str, bytes]]) -> None:
    """mode 600 的暫存檔 → fsync → rename。任何一步失敗就刪掉自己寫出的暫存檔與已 rename 的檔。"""
    temps: list[str] = []
    done: list[str] = []
    try:
        for path, data in pairs:
            tmp = os.path.join(os.path.dirname(os.path.abspath(path)),
                               f".{os.path.basename(path)}.tmp-{os.getpid()}")
            fd = os.open(tmp, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
            temps.append(tmp)
            with os.fdopen(fd, "wb") as f:
                f.write(data)
                f.flush()
                os.fsync(f.fileno())
        for (path, _), tmp in zip(pairs, temps):
            if os.path.lexists(path):
                raise Abort("輸出檔在執行期間出現了，⛔ 不覆寫")
            os.rename(tmp, path)
            done.append(path)
        for d in sorted({os.path.dirname(os.path.abspath(p)) for p, _ in pairs}):
            dfd = os.open(d, os.O_RDONLY)
            try:
                os.fsync(dfd)
            finally:
                os.close(dfd)
    except BaseException:
        for p in temps + done:
            try:
                os.unlink(p)
            except FileNotFoundError:
                pass
        raise


# ── 主流程 ──────────────────────────────────────────────────────────────

def run(args: argparse.Namespace) -> int:
    # 讀任何來源之前先擋：⛔ 不覆寫既有的證據
    for opt, p in (("--out", args.out), ("--report", args.report)):
        if os.path.lexists(p):
            raise Abort(f"{opt} 的檔案已存在，⛔ 不覆寫")
        if not os.path.isdir(os.path.dirname(os.path.abspath(p))):
            raise Abort(f"{opt} 的目錄不存在")
    if os.path.abspath(args.out) == os.path.abspath(args.report):
        raise Abort("--out 與 --report 是同一個檔")

    runs = load_runs(args.windows)
    wins = select_windows(runs)
    kept, stats = read_sources(source_dates(wins), wins)            # 1、2
    counts: Counter = Counter({name: 0 for name in RULE_NAMES})
    out_bytes, per_win = serialize(kept, counts)                    # 3、4
    checks, empty = load_secrets()                                  # 5
    gaps = l3_gaps(kept, checks)
    if gaps:
        raise Abort("L3：遮罩⛔ 沒有完整蓋住原始 log 裡的 " + "、".join(gaps))
    hits = l3_hits(out_bytes, checks)
    if hits:
        raise Abort("L3：遮罩後的輸出裡仍有 " + "、".join(hits))
    report = build_report(runs, wins, stats, counts, per_win, checks, empty,
                          out_bytes, len(kept), os.path.basename(args.out))
    hits = l3_hits(report, checks)
    if hits:
        raise Abort("L3：報告裡出現 " + "、".join(hits))
    del checks
    write_outputs([(args.out, out_bytes), (args.report, report)])  # 6
    print(f"ok：時間窗 {len(wins)} 個、來源 {len(stats)} 個日期、輸出 {len(kept)} 行；L3 通過。報告：{args.report}")
    return 0


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description="I-122 E2：取出 job_runs 時間窗內的 backend log，遮罩並驗證之後才寫檔。")
    ap.add_argument("--windows", required=True, help="E1 的 e1_job_runs.csv")
    ap.add_argument("--out", required=True, help="遮罩後的 JSONL（⛔ 不覆寫既有檔）")
    ap.add_argument("--report", required=True, help="只有計數的報告（⛔ 不覆寫既有檔）")
    args = ap.parse_args(argv)
    try:
        return run(args)
    except Abort as e:
        print(f"extract-job-logs：中止——{e}。⛔ 沒有寫出任何檔案。", file=sys.stderr)
        return 1
    except KeyboardInterrupt:
        print("extract-job-logs：中斷。⛔ 沒有寫出任何檔案。", file=sys.stderr)
        return 130
    except BaseException as e:  # noqa: BLE001 — 例外訊息可能帶原文，只印型別名稱
        print(f"extract-job-logs：中止——內部錯誤（{type(e).__name__}；⛔ 不印例外訊息）。⛔ 沒有寫出任何檔案。",
              file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
