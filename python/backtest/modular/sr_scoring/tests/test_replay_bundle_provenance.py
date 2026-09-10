"""provenance：專案模組的界定與 containment（I-100 五）。

⚠️ **`--image-digest`／`--base-commit`／`--tooling-patch-sha256`／`--source-root` 的
spoof 與「實際值不符」驗證在 shell 層**（`scripts/test-replay-args.sh`）——容器內的
Python 無法自己 `docker image inspect` 得知跑在哪個 image。這一檔只驗 Python 這一側
做得到的部分：模組界定、containment、以及不敏感的 runtime settings。
"""
from __future__ import annotations

import sys
from types import ModuleType

import pytest

from ..replay_bundle import (
    ProvenanceError,
    build_provenance,
    is_project_module,
    pip_freeze_sha256,
    project_module_hashes,
)
from ..replay_bundle.provenance import runtime_settings


@pytest.mark.parametrize("name", [
    "backtest.modular.sr_scoring",
    "backtest.modular.sr_scoring.evaluation",
    "backtest.modular.sr_scoring.replay_bundle.bundle",
    "backtest.modular.dataset",
    "config",
    "db",
])
def test_project_modules_are_identified_by_name(name):
    assert is_project_module(name)


@pytest.mark.parametrize("name", ["pandas", "numpy", "json", "sklearn.metrics", "joblib",
                                  "backtest_other", "sr_scoring"])
def test_third_party_and_stdlib_are_not_project_modules(name):
    """⛔ stdlib／site-packages 不做 containment 檢查——replay 必然載入它們。"""
    assert not is_project_module(name)


def test_hashes_only_cover_project_modules(tmp_path):
    hashes = project_module_hashes("/app")
    assert any(name.startswith("backtest.modular.sr_scoring") for name in hashes)
    assert "pandas" not in hashes and "numpy" not in hashes
    assert all(len(digest) == 64 for digest in hashes.values())


def test_project_module_outside_source_root_aborts(tmp_path):
    """⚠️ 這正是 containment 要抓的：從別的路徑 import 了一份同名的專案模組。

    ⛔ 不能用「`__file__` 在 source_root 下」來定義專案模組——那樣它會因為路徑在 root 外
    而被歸類成第三方套件，**正好繞過這道檢查**。
    """
    rogue = tmp_path / "rogue_scoring.py"
    rogue.write_text("x = 1", encoding="utf-8")
    fake = ModuleType("backtest.modular.sr_scoring.rogue")
    fake.__file__ = str(rogue)
    modules = {"backtest.modular.sr_scoring.rogue": fake}
    with pytest.raises(ProvenanceError) as exc:
        project_module_hashes("/app", modules=modules)
    assert "落在 source_root 外" in str(exc.value)


def test_third_party_outside_source_root_is_fine():
    fake = ModuleType("pandas")
    fake.__file__ = "/usr/local/lib/python3.11/site-packages/pandas/__init__.py"
    real = {k: v for k, v in sys.modules.items() if is_project_module(k)}
    hashes = project_module_hashes("/app", modules={**real, "pandas": fake})
    assert "pandas" not in hashes


def test_empty_project_modules_aborts():
    with pytest.raises(ProvenanceError):
        project_module_hashes("/app", modules={})


def test_bad_source_root_aborts():
    with pytest.raises(ProvenanceError):
        project_module_hashes("/no/such/root")


def test_pip_freeze_hash_is_stable_and_order_independent():
    a = pip_freeze_sha256([("pandas", "2.0.0"), ("numpy", "1.26.0")])
    b = pip_freeze_sha256([("numpy", "1.26.0"), ("pandas", "2.0.0")])
    assert a == b
    assert a != pip_freeze_sha256([("pandas", "2.0.1"), ("numpy", "1.26.0")])


def test_runtime_settings_never_include_the_dsn():
    """⛔ 不記 DSN／密碼。"""
    settings = runtime_settings()
    joined = " ".join(f"{k}{v}" for k, v in settings.items()).lower()
    assert "dsn" not in joined and "password" not in joined
    assert "db_driver" in settings


def test_build_provenance_requires_an_image_digest():
    with pytest.raises(ProvenanceError):
        build_provenance(source_root="/app", image_digest="", base_commit=None,
                         tooling_patch_sha256=None, runner_sha256="c" * 64, argv=[])


def test_build_provenance_requires_a_runner_sha256():
    """⛔ 少了它就說不清這份結果是哪一支腳本、哪一版跑出來的。

    ⚠️ runner 是**容器外**的 shell 腳本，容器內的 Python 讀不到——所以只能由腳本注入。
    """
    with pytest.raises(ProvenanceError) as exc:
        build_provenance(source_root="/app", image_digest="sha256:" + "a" * 64,
                         base_commit=None, tooling_patch_sha256=None,
                         runner_sha256=None, argv=[])
    assert "runner_sha256" in str(exc.value)


def test_build_provenance_records_argv_and_injected_values():
    prov = build_provenance(
        source_root="/app", image_digest="sha256:" + "a" * 64,
        base_commit="0" * 40, tooling_patch_sha256="1" * 64,
        runner_sha256="c" * 64, argv=["--bundle", "/b"],
    )
    assert prov["argv"] == ["--bundle", "/b"]
    assert prov["base_commit"] == "0" * 40
    assert prov["tooling_patch_sha256"] == "1" * 64
    assert prov["source_root"] == "/app"
    assert prov["runner_sha256"] == "c" * 64
    assert len(prov["pip_freeze_sha256"]) == 64
