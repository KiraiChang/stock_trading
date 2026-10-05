#!/usr/bin/env python3
"""I-074 Stage 2 ⑦d：memory／disk acceptance 的 replay launcher（容器內，Stage 2 image）。

規格見 issue.md I-074「Stage 2 步驟 ⑦d 細部計畫 v1」「二之二」。⛔ 不是正式入口：只有 acceptance harness 的 shim 在 role
`replay` 時把容器指令 `python -m backtest.modular.sr_scoring.evaluation <參數>` 換成 `python /acceptance/replay_stub.py <參數>`。

做的事只有一件：**只替換** `evaluation._decision_replay_rows`（約 180 分鐘的計算），其餘全部是 ⑩ 會執行的真實程式碼與資料，
argv 逐 token 不變（provenance 的 `argv` 與 ⑩ 相同），結束碼照 `evaluation.main()`。

* `I074_ACCEPTANCE_COMPUTE=stub`：讀錨定的 D+1 after（`--after-artifact`），依原順序收成整份 list（與真正的計算一樣常駐）；
  ⚠️ 逐列解析的列以 `sys.intern` 共用鍵與字串值（`compact()`）——真正的計算產出的列，鍵與常數字串都是共用的物件；
  ⑦d 實測 13,417 列：未共用 ＋324 MiB、共用 ＋141 MiB，而 ③c 見證趟的完整計算（含整份 rows）峰值約 315 MiB——未共用的
  stub 在 mem-guard 的 444m 下被 OOM（⑦d 開發驗證第二次實跑），⛔ 不是真正 replay 的形狀；
* `I074_ACCEPTANCE_COMPUTE=full`（⑨-1 的量測趟）：呼叫**原本的** `_decision_replay_rows()`（容器的 `/app` 是 `e1cbbbd` ＋ tooling、
  ⛔ 不套 counterfactual）；先斷言 cohort 的列全部仍是候選（證明⛔ 沒有套 counterfactual），再套 success 的合成；
* `I074_ACCEPTANCE_REPLAY=success`：cohort 的 156 列改成 `lifecycle_phase="TESTING"`、`rr_decoupling_candidate=False`
  （與 sizing fixture 相同的合成）→ 反事實生效 → rc=0；`failure`：逐列不變 → `rr_not_restored` → rc=6。
"""
from __future__ import annotations

import os
import sys
from typing import Any, Callable

MODES = ("success", "failure")
COMPUTES = ("stub", "full")


class StubError(RuntimeError):
    pass


def compact(obj: Any) -> Any:
    """遞迴地以 `sys.intern` 共用 dict 的鍵與字串值（stub 的列才會像真正計算產出的列一樣共用這些物件）。"""
    if isinstance(obj, dict):
        return {sys.intern(k): compact(v) for k, v in obj.items()}
    if isinstance(obj, list):
        return [compact(v) for v in obj]
    if isinstance(obj, str):
        return sys.intern(obj)
    return obj


def argv_value(argv: list[str], flag: str) -> str:
    """`flag` 必須恰好出現一次（`--flag value` 的形式；⛔ 不接受 `=` 的寫法）。"""
    hits = [i for i, tok in enumerate(argv) if tok == flag or tok.startswith(flag + "=")]
    if len(hits) != 1 or argv[hits[0]] != flag or hits[0] + 1 >= len(argv):
        raise StubError(f"{flag} 必須恰好出現一次（`{flag} <值>`）")
    return argv[hits[0] + 1]


def synthesize_success(rows: list[dict[str, Any]], cohort: set, row_key: Callable[[Any], Any]) -> list[dict[str, Any]]:
    """反事實生效：cohort 的列 RR 加回去 → 不再是 CONTINUATION、flag 為 false（其餘列是同一個物件）。"""
    out, hit = [], 0
    for row in rows:
        if row_key(row) in cohort:
            row = dict(row, lifecycle_phase="TESTING", rr_decoupling_candidate=False)
            hit += 1
        out.append(row)
    if hit != len(cohort):
        raise StubError(f"cohort 有 {len(cohort)} 列，rows 裡只找到 {hit} 列")
    return out


def assert_cohort_still_candidates(rows: list[dict[str, Any]], cohort: set, row_key: Callable[[Any], Any]) -> None:
    """full：真實計算的輸出裡，cohort 的列必須全部仍是候選——否則容器裡的程式碼套了 counterfactual（⛔ 不得提前看到結果）。"""
    seen = 0
    for row in rows:
        if row_key(row) in cohort:
            seen += 1
            if not (row.get("lifecycle_phase") == "CONTINUATION" and row.get("setup_rr_qualified") is False
                    and row.get("rr_decoupling_candidate") is True):
                raise StubError(f"cohort 的列 {row_key(row)} ⛔ 不是候選——full 的程式碼不該套 counterfactual")
    if seen != len(cohort):
        raise StubError(f"cohort 有 {len(cohort)} 列，真實計算只產出 {seen} 列")


def main(argv: list[str] | None = None) -> int:
    argv = list(sys.argv[1:] if argv is None else argv)
    mode = os.environ.get("I074_ACCEPTANCE_REPLAY")
    compute = os.environ.get("I074_ACCEPTANCE_COMPUTE")
    try:
        if mode not in MODES or compute not in COMPUTES or (compute == "full" and mode != "success"):
            raise StubError(f"I074_ACCEPTANCE_REPLAY={mode!r}、I074_ACCEPTANCE_COMPUTE={compute!r} 不符"
                            "（stub：success／failure；full：只限 success）")
        after = argv_value(argv, "--after-artifact")
        cohort_path = argv_value(argv, "--cohort-manifest")
    except StubError as exc:
        print(f"[acceptance stub] {exc}", file=sys.stderr)
        return 1
    sys.path.insert(0, os.getcwd())       # runner 的 -w /app：與 `python -m` 的 sys.path[0] 相同
    from backtest.modular.sr_scoring import evaluation as ev
    from backtest.modular.sr_scoring.replay_bundle import (AFTER_KIND, COHORT_KIND, load_canonical_evidence_artifact,
                                                           row_key, stream_canonical_artifact, validate_cohort_manifest)

    cohort = set(validate_cohort_manifest(load_canonical_evidence_artifact(cohort_path, COHORT_KIND).parsed))
    original = ev._decision_replay_rows

    def stub_rows(*_args: Any, **_kwargs: Any) -> list[dict[str, Any]]:
        rows: list[dict[str, Any]] = []
        stream_canonical_artifact(after, AFTER_KIND, on_row=lambda row, _raw: rows.append(compact(row)))
        return synthesize_success(rows, cohort, row_key) if mode == "success" else rows

    def full_rows(*args: Any, **kwargs: Any) -> list[dict[str, Any]]:
        rows = original(*args, **kwargs)
        assert_cohort_still_candidates(rows, cohort, row_key)
        return synthesize_success(rows, cohort, row_key)

    ev._decision_replay_rows = stub_rows if compute == "stub" else full_rows
    sys.argv = ["evaluation", *argv]
    try:
        ev.main()
    except StubError as exc:
        print(f"[acceptance stub] {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
