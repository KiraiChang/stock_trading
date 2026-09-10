"""CLI 的模式判定與輸入所有權（I-100 Phase D／E）。

⚠️ **所有衝突都必須在 `check_connection()` 與 replay 之前中止**——Stage 0 是唯一會碰
live DB 的階段，而 bundle 模式跑滿約 3.7 小時才發現模式不完整是不可接受的。
"""
from __future__ import annotations

import json
from pathlib import Path

import pytest

from .. import evaluation as evaluation_module
from ..evaluation import CliUsageError, resolve_bundle_stage, resolve_cli_mode

FIXTURE = Path(__file__).resolve().parents[4] / "scripts" / "fixtures" / "stage0_argv.json"


def _run_cli(monkeypatch, argv, **stubs):
    """跑 `main()`，回傳 `SystemExit` 的 code（正常結束回 0）。"""
    monkeypatch.setattr("sys.argv", ["evaluation.py", *argv])
    reached: dict[str, object] = {}

    def stub_emit(**kwargs):
        reached["stage0"] = kwargs
        return stubs.get("outcome") or _FakeOutcome()

    def stub_bundle(args, *, stage, argv):
        reached["bundle"] = stage
        return {"stage": stage}

    def forbidden_connect():
        raise AssertionError("⛔ 在參數驗證之前就連上了 DB")

    monkeypatch.setattr(evaluation_module, "emit_replay_bundle", stub_emit)
    monkeypatch.setattr(evaluation_module, "run_bundle_stage", stub_bundle)
    import db

    monkeypatch.setattr(db, "check_connection", stubs.get("check_connection", forbidden_connect))

    try:
        evaluation_module.main()
    except SystemExit as exc:
        return int(exc.code or 0), reached
    return 0, reached


class _FakeOutcome:
    bundle_id = "b1_20260901_1d_aaaaaaaa_bbbbbbbb"
    path = Path("/tmp/fake")
    published = True


_STAGE0 = [
    "--as-of", "2026-09-01", "--symbols", "2330", "--emit-bundle", "/out",
    "--model-path", "/app/models/m.joblib", "--image-digest", "sha256:" + "a" * 64,
    "--runner-sha256", "c" * 64,
]
_BUNDLE = [
    "--bundle", "/b", "--output-dir", "/out", "--before-ref", "ecbc141",
    "--image-digest", "sha256:" + "a" * 64, "--source-root", "/app",
    "--runner-sha256", "c" * 64,
]


# ── 模式判定 ────────────────────────────────────────────────────────────────

def test_mode_resolution():
    assert resolve_cli_mode(set()) == "legacy"
    assert resolve_cli_mode({"emit_bundle"}) == "stage0"
    assert resolve_cli_mode({"bundle"}) == "bundle"
    with pytest.raises(CliUsageError):
        resolve_cli_mode({"emit_bundle", "bundle"})


def test_pairing_rule():
    """兩個都沒給→Stage 1；兩個都給→Stage 2；只給其中一個→⛔ 中止。"""
    assert resolve_bundle_stage({"bundle"}) == 1
    assert resolve_bundle_stage({"bundle", "after_artifact", "cohort_manifest"}) == 2
    for partial in ({"bundle", "after_artifact"}, {"bundle", "cohort_manifest"}):
        with pytest.raises(CliUsageError) as exc:
            resolve_bundle_stage(partial)
        assert "成對出現" in str(exc.value)


def test_stage0_and_stage1_dispatch(monkeypatch):
    code, reached = _run_cli(monkeypatch, _STAGE0)
    assert code == 0 and "stage0" in reached

    code, reached = _run_cli(monkeypatch, _BUNDLE)
    assert code == 0 and reached["bundle"] == 1

    code, reached = _run_cli(monkeypatch, _BUNDLE + ["--after-artifact", "/a.json",
                                                     "--cohort-manifest", "/c.json"])
    assert code == 0 and reached["bundle"] == 2


def test_incomplete_bundle_mode_aborts_before_replay(monkeypatch, capsys):
    code, reached = _run_cli(monkeypatch, _BUNDLE + ["--after-artifact", "/a.json"])
    assert code == 1
    assert "bundle" not in reached  # ⛔ 還沒進到 replay
    assert "成對出現" in capsys.readouterr().err


# ── Stage 0 的封閉式允許清單 ────────────────────────────────────────────────

@pytest.mark.parametrize("extra", [
    ["--decision-replay"],
    ["--write-db"],
    ["--passed", "true"],
    ["--sweep"],
    ["--sweep-decision-replay"],
    ["--bundle", "/b"],
    ["--output-dir", "/o"],
    ["--after-artifact", "/a"],
    ["--cohort-manifest", "/c"],
    ["--csv", "a.csv"],
    ["--chip-json", "c.json"],
    ["--model-governance-json", "g.json"],
    ["--output", "o.json"],
    ["--atr-width-grid", "1.0"],
    ["--max-merge-width-grid", "2.0"],
    ["--atr-width-multiplier", "1.5"],
    ["--min-history-bars", "80"],
    ["--replay-max-rows", "200"],
    ["--before-ref", "main"],
    ["--base-commit", "abc"],
    ["--tooling-patch-sha256", "0" * 64],
])
def test_stage0_rejects_forbidden_args(monkeypatch, capsys, extra):
    code, reached = _run_cli(monkeypatch, _STAGE0 + extra)
    assert code == 1
    assert "stage0" not in reached
    err = capsys.readouterr().err
    # `--bundle` 會先被「兩種模式不可並用」擋下，那也是正確的中止理由。
    assert "不接受這些參數" in err or "不可同時使用" in err


def test_explicitly_passing_a_default_value_still_aborts(monkeypatch, capsys):
    """「明確傳入等於預設值」也要中止——⛔ 不能用「值等於預設」當成沒傳。"""
    code, _ = _run_cli(monkeypatch, _STAGE0 + ["--min-history-bars", "80"])
    assert code == 1
    assert "不接受這些參數" in capsys.readouterr().err


def test_write_db_is_blocked_before_connecting(monkeypatch, capsys):
    """⚠️ `--write-db` 會讓 Stage 0 真的寫進 live DB——必須在連 DB 之前擋下。"""
    code, reached = _run_cli(monkeypatch, _STAGE0 + ["--write-db"])
    assert code == 1 and "stage0" not in reached


@pytest.mark.parametrize("missing", ["--as-of", "--symbols", "--emit-bundle",
                                     "--model-path", "--image-digest"])
def test_stage0_requires_its_mandatory_args(monkeypatch, capsys, missing):
    argv = list(_STAGE0)
    idx = argv.index(missing)
    del argv[idx:idx + 2]
    if missing == "--emit-bundle":
        # 少了它就不是 Stage 0 了——改成驗其餘四個在 Stage 0 下必填。
        return
    code, _ = _run_cli(monkeypatch, argv)
    assert code == 1
    assert "缺少必填參數" in capsys.readouterr().err


# ── bundle 模式的允許清單 ───────────────────────────────────────────────────

@pytest.mark.parametrize("extra", [
    ["--symbols", "2330"], ["--csv", "a.csv"], ["--timeframe", "1d"], ["--limit", "1500"],
    ["--as-of", "2026-09-01"], ["--model-path", "/m"], ["--chip-json", "c.json"],
    ["--model-governance-json", "g.json"], ["--replay-max-rows", "200"],
    ["--report-max-rows", "200"], ["--atr-width-grid", "1.0"], ["--min-history-bars", "80"],
    ["--write-db"], ["--passed", "true"], ["--output", "o.json"],
    ["--emit-bundle", "/e"], ["--sweep"],
])
def test_bundle_mode_rejects_forbidden_args(monkeypatch, capsys, extra):
    code, reached = _run_cli(monkeypatch, _BUNDLE + extra)
    assert code == 1
    assert "bundle" not in reached


# ── 腳本注入參數的重複偵測 ──────────────────────────────────────────────────

@pytest.mark.parametrize("name,value", [
    ("--image-digest", "sha256:" + "b" * 64),
    ("--base-commit", "deadbeef"),
    ("--tooling-patch-sha256", "0" * 64),
    ("--source-root", "/elsewhere"),
    ("--runner-sha256", "d" * 64),
])
def test_duplicate_script_injected_args_abort(monkeypatch, capsys, name, value):
    """⛔ 不靜默採用最後一個——重複代表使用者也傳了一個。"""
    argv = _BUNDLE + [name, value]
    code, reached = _run_cli(monkeypatch, argv)
    if name in _BUNDLE:
        assert code == 1
        assert "出現 2 次" in capsys.readouterr().err
    else:
        # `--base-commit` / `--tooling-patch-sha256` 在基準 argv 裡只出現一次，
        # 這裡再加一次才構成重複。
        code, _ = _run_cli(monkeypatch, argv + [name, value])
        assert code == 1
        assert "出現 2 次" in capsys.readouterr().err


@pytest.mark.parametrize("abbrev", ["--image-d", "--base-c", "--tooling-p",
                                   "--source-r", "--runner-s"])
def test_argparse_abbreviation_cannot_smuggle_a_protected_arg(monkeypatch, capsys, abbrev):
    """⛔ argparse 預設接受唯一前綴縮寫——`--image-d` 會被展開成 `--image-digest`。

    官方注入的值排在使用者參數之前，argparse 取最後一個，所以縮寫形式會讓使用者的值贏。
    `allow_abbrev=False` 之後它變成未知參數，argparse 直接以 exit code 2 中止。
    """
    code, reached = _run_cli(monkeypatch, _BUNDLE + [abbrev, "spoofed"])
    assert code != 0
    assert "bundle" not in reached and "stage0" not in reached


def test_equals_form_counts_as_a_duplicate(monkeypatch, capsys):
    code, _ = _run_cli(monkeypatch, _BUNDLE + ["--image-digest=sha256:" + "c" * 64])
    assert code == 1
    assert "出現 2 次" in capsys.readouterr().err


# ── 官方腳本組出來的 argv ───────────────────────────────────────────────────

def test_official_stage0_argv_fixture_is_accepted(monkeypatch):
    """⚠️ CLI 衝突測試必須用**官方腳本實際組出的完整 Stage 0 argv** 跑一次。

    fixture 由 `scripts/test-replay-args.sh` 那一側斷言「腳本輸出 == 本檔」，
    兩邊夾住同一份檔案才不會漂移。
    """
    argv = json.loads(FIXTURE.read_text(encoding="utf-8"))["stage0_argv"]
    # fixture 存的是完整指令（含 `python -m …`），CLI 只吃它後面的參數。
    cli_args = argv[argv.index("-m") + 2:]
    code, reached = _run_cli(monkeypatch, cli_args)
    assert code == 0, f"官方 Stage 0 argv 被自己擋下了：{cli_args}"
    assert "stage0" in reached


@pytest.mark.parametrize("extra", [["--write-db"], ["--sweep"], ["--decision-replay"],
                                   ["--bundle", "/b"]])
def test_official_stage0_argv_plus_conflict_aborts(monkeypatch, capsys, extra):
    argv = json.loads(FIXTURE.read_text(encoding="utf-8"))["stage0_argv"]
    cli_args = argv[argv.index("-m") + 2:] + extra
    code, reached = _run_cli(monkeypatch, cli_args)
    assert code == 1 and "stage0" not in reached
