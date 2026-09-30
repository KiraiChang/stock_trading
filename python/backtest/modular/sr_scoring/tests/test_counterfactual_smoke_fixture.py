"""I-074 Stage 2 反事實 smoke 的切片 fixture（`python/scripts/make_counterfactual_smoke_bundle.py`）。

⚠️ 正向測試讀的是**真正已進版控的** bundle（唯讀使用）；負向測試用它的**暫存複本**，⛔ 不碰真正的 bundle。
對照 issue.md I-074「Stage 2 步驟 ⑦a 細部計畫」B7 的「切片 fixture 的契約」與「守門」。
"""
from __future__ import annotations

import hashlib
import importlib.util
import shutil
from pathlib import Path

import pytest

from ..replay_bundle import load_bundle, validate_provenance

_PYTHON = Path(__file__).resolve().parents[4]
_SCRIPT = _PYTHON / "scripts" / "make_counterfactual_smoke_bundle.py"
_spec = importlib.util.spec_from_file_location("make_counterfactual_smoke_bundle", _SCRIPT)
fx = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(fx)

SOURCE = _PYTHON / "baselines" / fx.SOURCE_BUNDLE_ID


def _tree_digest(root: Path) -> dict[str, str]:
    return {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in sorted(root.rglob("*")) if p.is_file()}


@pytest.mark.skipif(not SOURCE.is_dir(), reason="正式 bundle 不在這個 checkout")
def test_derived_bundle_follows_the_contract(tmp_path):
    before = _tree_digest(SOURCE)
    bundle_id = fx.derive(SOURCE, tmp_path / "out")
    assert bundle_id == fx.EXPECTED_BUNDLE_ID == "b1_20260901_1d_de3ab843_a7c9ffb4"
    derived = load_bundle(tmp_path / "out" / bundle_id)
    source = load_bundle(SOURCE)
    assert derived.manifest["symbols"] == [fx.SYMBOL] == ["6243"]
    assert set(derived.candles) == {fx.SYMBOL}
    rows = derived.candles[fx.SYMBOL]
    assert len(rows) == fx.BARS == 250 and all(r["symbol"] == fx.SYMBOL for r in rows)
    assert rows == source.candles[fx.SYMBOL][-fx.BARS:]
    for field in ("as_of", "timeframe", "limit", "replay_scope", "report_max_rows", "readiness", "calendar",
                  "captured_at"):
        assert derived.manifest[field] == source.manifest[field], field
    assert derived.replay_config == dict(source.replay_config, dataset_from="2025-08-20T16:00:00+00:00")
    assert derived.trading_calendar == source.trading_calendar
    assert derived.model_path.read_bytes() == source.model_path.read_bytes()
    # provenance：smoke 專用的佔位值，⛔ 不是來源的；而且是**合法的** stage0 provenance。
    prov = derived.manifest["provenance"]
    validate_provenance(prov, role="stage0")
    assert prov != source.manifest["provenance"]
    assert prov["image_digest"] == fx.PLACEHOLDER_IMAGE and prov["runner_sha256"] == fx.PLACEHOLDER_RUNNER
    assert prov["base_commit"] is None and prov["tooling_patch_sha256"] is None
    assert _tree_digest(SOURCE) == before                     # 來源⛔ 未被修改


@pytest.mark.skipif(not SOURCE.is_dir(), reason="正式 bundle 不在這個 checkout")
@pytest.mark.parametrize("where", ["equal", "inside"])
def test_output_inside_the_source_is_rejected_before_writing(tmp_path, where):
    copy = tmp_path / fx.SOURCE_BUNDLE_ID
    shutil.copytree(SOURCE, copy)
    before = _tree_digest(copy)
    out = copy if where == "equal" else copy / "nested"
    with pytest.raises(fx.FixtureError, match="不得落在來源"):
        fx.derive(copy, out)
    assert _tree_digest(copy) == before and not (copy / "nested").exists()


@pytest.mark.skipif(not SOURCE.is_dir(), reason="正式 bundle 不在這個 checkout")
def test_source_name_must_match(tmp_path):
    copy = tmp_path / "b1_other"
    shutil.copytree(SOURCE, copy)
    with pytest.raises(fx.FixtureError, match="來源必須是"):
        fx.derive(copy, tmp_path / "out")
    assert not (tmp_path / "out").exists()


def test_main_prints_nothing_on_failure(tmp_path, capsys):
    assert fx.main([str(tmp_path / "missing"), str(tmp_path / "out")]) == 1
    assert capsys.readouterr().out == ""
