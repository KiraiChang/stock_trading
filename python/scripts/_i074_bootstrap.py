"""host 端載入 `replay_bundle` 的最小 package context（I-074 Stage 1）。

⚠️ **為什麼需要 bootstrap**：host 沒有 pandas，而
`backtest/modular/sr_scoring/__init__.py` 會 `import pandas`。要在 `docker run` **之前**用
**同一份** validator 驗 identity，就得繞過那個 `__init__`。

⚠️ **抽成共用模組**是刻意的：`validate-i074-run-identity.py` 與
`ensure-i074-run-identity.py` 都要它，各寫一份就是**雙真相源**（這個計畫已經在
`NO_ZONE_SCORES_ERROR` 上犯過一次）。

⛔ **這條路徑上的模組必須 dependency-light**：只能 import 標準庫與彼此，
⛔ 不得（直接或間接）碰 pandas／sklearn／lightgbm，否則 host 端守門會失效。
"""
from __future__ import annotations

import importlib.util
import sys
import types
from pathlib import Path

_PKG = "_i074_rb"
# ⚠️ 依賴順序：canonical → publish → calendar → artifacts → bundle → run_identity。
_MODULES = ("canonical", "publish", "calendar", "artifacts", "bundle", "run_identity")
# ⚠️ I-074 Stage 2：host 端要跑 Stage 1 信任錨（見證趟的 E7 前置守門）時，再多載這幾個
# （同樣 dependency-light；2026-09-23 在沒有 pandas 的 host 上實測可載入並跑完）。
STAGE2_MODULES = _MODULES + ("provenance", "crossday", "probe", "evidence", "stream", "stage2_evidence")
# ⚠️ ③d：host 端取合成守門的宣告值（`i074-stage2-patch-claims.py`）還要 envcheck 與 stage2_archive
# （同樣 dependency-light；2026-09-24 在沒有 pandas 的 host 上實測可載入）。
STAGE2_ARCHIVE_MODULES = STAGE2_MODULES + ("envcheck", "stage2_archive")


def load_replay_bundle(repo_python: Path | None = None, modules: tuple[str, ...] = _MODULES) -> dict:
    """回傳 `{模組名: module}`。

    ⚠️ **一律以 `_i074_rb.` 前綴註冊**：`replay_bundle/calendar.py` 與**標準庫的
    `calendar` 同名**，用裸名會把標準庫那個蓋掉。
    """
    if repo_python is None:
        repo_python = Path(__file__).resolve().parent.parent
    root = repo_python / "backtest" / "modular" / "sr_scoring" / "replay_bundle"
    if not root.is_dir():
        raise SystemExit(f"找不到 replay_bundle：{root}")
    if _PKG not in sys.modules:
        pkg = types.ModuleType(_PKG)
        pkg.__path__ = [str(root)]
        sys.modules[_PKG] = pkg
    loaded = {}
    for name in modules:
        key = f"{_PKG}.{name}"
        if key not in sys.modules:
            spec = importlib.util.spec_from_file_location(key, root / f"{name}.py")
            module = importlib.util.module_from_spec(spec)
            sys.modules[key] = module
            spec.loader.exec_module(module)
        loaded[name] = sys.modules[key]
    return loaded
