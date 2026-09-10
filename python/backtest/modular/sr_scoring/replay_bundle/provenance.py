"""執行身分：這份結果是「哪一份程式碼、在哪一個環境、用什麼參數」算出來的。

⛔ **不能用「`__file__` 在 `source_root` 下」來定義專案模組**（那是循環定義）：若真的從別的
路徑 import 了一份 `backtest.modular.sr_scoring`，它會因為**路徑在 root 外**而被歸類成
第三方套件，**正好繞過那道檢查**——而那正是這道檢查要抓的情況。所以**先用模組名稱界定**，
再驗這些模組 resolved 後的 `__file__` 必須落在唯讀 `source_root` 內。

⛔ **不對 stdlib／site-packages 做 containment 檢查、也不逐檔 hash**：replay 必然載入
stdlib、pandas、numpy、joblib，那些本來就在 `source_root` 外，逐檔擋會讓正常執行中止。
它們改由 `image_digest` ＋ `python_version` ＋ `pip_freeze` 識別。

⚠️ **`base_commit`／`tooling_patch_sha256`／`image_digest`／`source_root` 的值由官方腳本
決定並注入**，這裡只負責記錄與（能驗的部分）驗證：容器內的 Python **無法自己
`docker image inspect` 得知自己跑在哪個 image**，所以那一層驗證只能發生在 shell 層。
"""
from __future__ import annotations

import sys
from pathlib import Path
from typing import Any, Iterable

from .canonical import canonical_json_bytes, sha256_file, sha256_hex

# 專案模組的名稱界定。⚠️ 依**名稱**，不是依路徑。
PROJECT_MODULE_PREFIXES = ("backtest.modular.sr_scoring",)
PROJECT_MODULE_NAMES = ("config", "db")
# 明列的 runner／loader 模組：它們不在 sr_scoring 底下，但 replay 的結果直接依賴它們。
PROJECT_EXTRA_MODULES = ("backtest.modular.dataset", "backtest.modular.service")

# ⛔ 不記 DSN／密碼。只記「會改變結果、且不敏感」的設定。
RUNTIME_SETTING_KEYS = (
    "DB_DRIVER",
    "SR_SCORING_MODEL_PATH",
    "SR_SCORING_EVIDENCE_ENABLED",
    "SR_SCORING_EVIDENCE_MAX_ZONES",
    "SR_SCORING_ADAPTIVE_ZONE_BUILDERS_ENABLED",
)


class ProvenanceError(ValueError):
    """provenance 推導失敗——⛔ 一律中止，不留一份說不清出處的證據。"""


def is_project_module(name: str) -> bool:
    if name in PROJECT_MODULE_NAMES or name in PROJECT_EXTRA_MODULES:
        return True
    return any(name == p or name.startswith(p + ".") for p in PROJECT_MODULE_PREFIXES)


def project_module_hashes(source_root: str | Path, modules: dict | None = None) -> dict[str, str]:
    """已載入的專案模組 → 逐檔 SHA-256。

    ⚠️ 任一專案模組的檔案落在 `source_root` 外即中止：那代表跑的不是掛載進來的那份程式碼。
    """
    root = Path(source_root).resolve()
    if not root.is_dir():
        raise ProvenanceError(f"source_root 不是目錄：{source_root}")

    out: dict[str, str] = {}
    for name, module in sorted((modules if modules is not None else sys.modules).items()):
        if not is_project_module(name) or module is None:
            continue
        file = getattr(module, "__file__", None)
        if not file:
            # namespace package 沒有 __file__——它沒有可 hash 的內容，跳過。
            continue
        resolved = Path(file).resolve()
        try:
            resolved.relative_to(root)
        except ValueError as exc:
            raise ProvenanceError(
                f"專案模組 {name} 的檔案落在 source_root 外：{resolved}（root={root}）。"
                "⚠️ 這代表跑的不是掛載進來的那份程式碼，⛔ 不得產出 provenance。"
            ) from exc
        out[name] = sha256_file(resolved)
    if not out:
        raise ProvenanceError("找不到任何已載入的專案模組——provenance 會是空的，⛔ 中止。")
    return out


def pip_freeze_sha256(distributions: Iterable[tuple[str, str]] | None = None) -> str:
    """已安裝套件清單的指紋。

    用 `importlib.metadata` 而不是 `pip freeze` 子行程：離線容器不見得裝了 pip，
    而且**多開一個行程只為了拿一份清單**在 2GiB 的 host 上不值得。
    """
    if distributions is None:
        from importlib import metadata

        distributions = [
            (dist.metadata["Name"] or "", dist.version or "")
            for dist in metadata.distributions()
        ]
    items = sorted({(str(name), str(version)) for name, version in distributions if name})
    return sha256_hex(canonical_json_bytes([list(item) for item in items]))


def runtime_settings(config_module=None) -> dict[str, Any]:
    """非敏感的執行期設定。⛔ 不含 DSN／密碼。"""
    if config_module is None:
        import config as config_module  # noqa: PLC0415 - 延後 import，避免 module 期副作用
    out: dict[str, Any] = {}
    for key in RUNTIME_SETTING_KEYS:
        if hasattr(config_module, key):
            out[key.lower()] = getattr(config_module, key)
    return out


def build_provenance(
    *,
    source_root: str | Path,
    image_digest: str,
    base_commit: str | None,
    tooling_patch_sha256: str | None,
    runner_sha256: str | None,
    argv: list[str],
    modules: dict | None = None,
    config_module=None,
    python_version: str | None = None,
) -> dict[str, Any]:
    """組出 manifest 的 provenance 區。

    ⚠️ `base_commit`／`tooling_patch_sha256` 在 Stage 0 可以是 `None`——Stage 0 不做
    before／after 比對，沒有 worktree 可推導；Stage 1／2 一律由腳本注入。
    """
    if not image_digest:
        raise ProvenanceError("image_digest 缺少——它是執行環境的客觀識別值，⛔ 不得留空。")
    if not runner_sha256:
        raise ProvenanceError(
            "runner_sha256 缺少——它由官方腳本算好後注入（腳本本身 ＋ scripts/lib/replay-args.sh）。"
            "⛔ 不得留空：少了它就說不清這份結果是哪一支腳本、哪一版跑出來的。"
        )
    return {
        "source_root": str(Path(source_root).resolve()),
        "image_digest": image_digest,
        "python_version": python_version or sys.version.split()[0],
        "pip_freeze_sha256": pip_freeze_sha256(),
        "project_modules_sha256": project_module_hashes(source_root, modules=modules),
        # ⚠️ runner 是**容器外**的 shell 腳本，容器內的 Python 讀不到它——所以 hash 由
        # 腳本自己算好後注入。⛔ 缺它就中止：少了 runner hash，「用哪一支腳本、哪一版」
        # 這件事就沒有記錄，而它決定了掛載、注入值與離線與否。
        "runner_sha256": runner_sha256,
        "base_commit": base_commit,
        "tooling_patch_sha256": tooling_patch_sha256,
        "argv": list(argv),
        "runtime_settings": runtime_settings(config_module),
    }
