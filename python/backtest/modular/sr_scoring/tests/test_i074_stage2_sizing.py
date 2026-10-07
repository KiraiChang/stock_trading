"""I-074 Stage 2 步驟 ④：sizing harness 的 Python 段（`python/scripts/i074_stage2_sizing.py`）。

對應 issue.md I-074「Stage 2 步驟 ④：sizing harness 計畫書」（v7 ＋ 差異 1）的測試 a8、a11、a11b、a11c、a13、
b2、b3、c、c2、d0、d。shim、wrapper、I/O 透明、中斷、封閉寫入、metadata twin 與 harness 的參數檢查在
`scripts/test-replay-args.sh`。
"""
from __future__ import annotations

import gzip
import hashlib
import importlib.util
import json
import os
import shutil
from pathlib import Path

import pytest

from ..replay_bundle import envcheck as ec
from ..replay_bundle import stage2_archive as sa
from ..replay_bundle.canonical import CanonicalError, canonical_json_bytes
from . import test_replay_envcheck as tec
from . import test_replay_stage2_archive as t2

_SCRIPT = Path(__file__).resolve().parents[4] / "scripts" / "i074_stage2_sizing.py"
_spec = importlib.util.spec_from_file_location("i074_stage2_sizing", _SCRIPT)
sz = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(sz)

IMG = tec.NEW_IMG


# ── c：fixture 的產出通過**正式**的 validator ───────────────────────────────────

@pytest.fixture
def fixture_env(tmp_path, monkeypatch):
    env = t2._build_env(tmp_path, monkeypatch)
    cache = tmp_path / "cache"
    envcheck = env.root / "baselines" / "i074_stage2" / "envcheck"
    shutil.copytree(envcheck, cache / "envcheck")
    patch = tmp_path / "counterfactual.patch"
    patch.write_bytes(t2.CF)
    return env, cache, patch


def test_c_witness_fixture_republishes_an_equivalent_envcheck(fixture_env, tmp_path):
    env, cache, _ = fixture_env
    out = tmp_path / "runs" / "witness"
    out.mkdir(parents=True)
    result = sz.fixture_witness(cache, out)
    after = out / "witness" / "after_artifact.json"
    archived = gzip.decompress((cache / "envcheck" / "witness" / "after_artifact.json.gz").read_bytes())
    # ⚠️ 串流 writer 寫出的全量檔與 canonical_json_bytes() 的輸出逐位元相同。
    assert after.read_bytes() == canonical_json_bytes(json.loads(archived))
    assert result["after_sha256"] == hashlib.sha256(after.read_bytes()).hexdigest()
    shutil.rmtree(env.root / "baselines" / "i074_stage2" / "envcheck")
    assert tec._publish(env.root, out) == 0          # 正式的 envcheck 發布：EQUIVALENT


def test_c_success_fixture_passes_the_official_finalizer(fixture_env, tmp_path):
    env, cache, patch = fixture_env
    run = tmp_path / "runs" / "success"
    run.mkdir(parents=True)
    # ⚠️ ⑦b：合成 SHA 由 host 傳入；本測試的 tooling 是空的，所以合成 SHA ＝ counterfactual 的 SHA。
    result = sz.fixture_success(env.root, cache, run, hashlib.sha256(patch.read_bytes()).hexdigest())
    assert result["cohort_rows"] == len(t2.COHORT)
    before = run / "stage2" / "before_source_artifact.json"
    assert before.read_bytes() == canonical_json_bytes(json.loads(before.read_bytes()))
    (run / "patches").mkdir()
    (run / "patches" / "counterfactual.patch").write_bytes(patch.read_bytes())
    (run / "patches" / "tooling.patch").write_bytes(b"")
    assert t2._finalize(env, run) == 0


def test_c_failure_fixture_passes_the_official_publisher(fixture_env, tmp_path):
    env, cache, patch = fixture_env
    run = tmp_path / "runs" / "failure"
    run.mkdir(parents=True)
    composed = hashlib.sha256(patch.read_bytes()).hexdigest()
    assert sz.fixture_failure(env.root, cache, run, composed)["failure_reason"] == sa.FAILURE_RR_NOT_RESTORED
    (run / "patches").mkdir()
    (run / "patches" / "counterfactual.patch").write_bytes(patch.read_bytes())
    (run / "patches" / "tooling.patch").write_bytes(b"")
    root = t2._publish_failure(env, run)
    assert root.is_dir()


# ── c2：串流 writer ─────────────────────────────────────────────────────────

TOP = {"schema_version": 1, "kind": "k", "provenance": {"a": [1, {"z": "é"}]}, "zeta": "尾"}


@pytest.mark.parametrize("rows", [
    [],
    [{"x": 1}],
    [{"s": "引號\" 反斜線\\ 控制\x01\x1f 換行\n", "n": [1, 2.5, None, True], "d": {"b": {"c": "é🙂"}}}] * 3,
], ids=["empty", "single", "many_unicode_escapes_nested"])
def test_c2_stream_writer_is_byte_identical(tmp_path, rows):
    sha = sz.stream_write_canonical(tmp_path, "a.json", TOP, iter(rows))
    data = (tmp_path / "a.json").read_bytes()
    assert data == canonical_json_bytes({**TOP, "rows": rows})
    assert sha == hashlib.sha256(data).hexdigest()
    assert not list(tmp_path.glob(".a.json.*.tmp"))


def test_c2_rows_are_consumed_lazily(tmp_path):
    """⚠️ 64 KiB buffer：用遠超過 64 KiB 的 generator，在它尚未耗盡時斷言 temp 已經成長。"""
    grew: list[int] = []

    def rows():
        for i in range(60):
            if i == 40:   # 已經送出約 400 KiB
                temps = list(tmp_path.glob(".a.json.*.tmp"))
                grew.append(temps[0].stat().st_size if temps else 0)
            yield {"i": i, "pad": "x" * 10_000}

    sz.stream_write_canonical(tmp_path, "a.json", TOP, rows())
    assert grew and grew[0] > 0          # 偷偷 list(rows) 的實作在這裡會看到 0


def _no_outputs(tmp_path):
    return not (tmp_path / "a.json").exists() and not list(tmp_path.glob(".a.json.*.tmp"))


def test_c2_non_finite_value_leaves_nothing(tmp_path):
    with pytest.raises(CanonicalError):
        sz.stream_write_canonical(tmp_path, "a.json", TOP, iter([{"x": 1}, {"x": float("nan")}]))
    assert _no_outputs(tmp_path)


def test_c2_iterator_error_leaves_nothing(tmp_path):
    def rows():
        yield {"x": 1}
        raise RuntimeError("來源壞了")

    with pytest.raises(RuntimeError):
        sz.stream_write_canonical(tmp_path, "a.json", TOP, rows())
    assert _no_outputs(tmp_path)


def test_c2_write_failure_before_replace_leaves_nothing(tmp_path, monkeypatch):
    real_open = open

    class Broken:
        def __init__(self, fh):
            self.fh = fh

        def __enter__(self):
            return self

        def __exit__(self, *exc):
            self.fh.close()

        def write(self, data):
            raise OSError("ENOSPC（注入）")

        def flush(self):
            pass

        def fileno(self):
            return self.fh.fileno()

    monkeypatch.setattr(sz, "open", lambda path, mode="r", *a, **k: Broken(real_open(path, mode, *a, **k)),
                        raising=False)
    with pytest.raises(OSError, match="注入"):
        sz.stream_write_canonical(tmp_path, "a.json", TOP, iter([{"pad": "x" * 100_000}]))
    assert _no_outputs(tmp_path)


def test_c2_temp_fsync_failure_leaves_nothing(tmp_path, monkeypatch):
    def boom(fd):
        raise OSError("fsync（注入）")

    monkeypatch.setattr(sz, "_fsync_fd", boom)
    with pytest.raises(OSError, match="注入"):
        sz.stream_write_canonical(tmp_path, "a.json", TOP, iter([{"x": 1}]))
    assert _no_outputs(tmp_path)


def test_c2_directory_fsync_failure_after_replace(tmp_path, monkeypatch):
    """replace 之後的目錄 fsync 失敗：正式檔可以存在、temp 已不存在，例外照樣往外拋。"""
    def boom(directory):
        raise OSError("dir fsync（注入）")

    monkeypatch.setattr(sz, "_fsync_dir", boom)
    with pytest.raises(OSError, match="注入"):
        sz.stream_write_canonical(tmp_path, "a.json", TOP, iter([{"x": 1}]))
    assert (tmp_path / "a.json").exists() and not list(tmp_path.glob(".a.json.*.tmp"))


def test_c2_witness_does_not_publish_cohort_after_directory_fsync_failure(fixture_env, tmp_path, monkeypatch):
    env, cache, _ = fixture_env
    out = tmp_path / "wout"
    out.mkdir()

    def boom(directory):
        raise OSError("dir fsync（注入）")

    monkeypatch.setattr(sz, "_fsync_dir", boom)
    with pytest.raises(OSError):
        sz.fixture_witness(cache, out)
    assert (out / "witness" / "after_artifact.json").exists()
    assert not (out / "witness" / "cohort_manifest.json").exists()     # ⛔ 指標檔不得發布


def test_c2_witness_does_not_publish_cohort_when_sha_differs(fixture_env, tmp_path):
    env, cache, _ = fixture_env
    path = cache / "envcheck" / "witness" / "cohort_manifest.json.gz"
    cohort = json.loads(gzip.decompress(path.read_bytes()))
    cohort["after_artifact_sha256"] = "0" * 64
    from ..replay_bundle.canonical import canonical_gzip_bytes

    path.write_bytes(canonical_gzip_bytes(canonical_json_bytes(cohort)))
    out = tmp_path / "wout"
    out.mkdir()
    with pytest.raises(sz.SizingError, match="不發布 cohort'"):
        sz.fixture_witness(cache, out)
    assert not (out / "witness" / "cohort_manifest.json").exists()


# ── a8：json-file log 的上界 ──────────────────────────────────────────────────

def _reference_json_file(data: bytes, stream: str) -> int:
    """依 json-file 格式實際編碼（Go 的 encoder 會把 <>& 也跳脫；這裡取更保守的 ensure_ascii）。"""
    total = 0
    lines = data.split(b"\n")
    pieces = [line + b"\n" for line in lines[:-1]] + ([lines[-1]] if lines[-1] else [])
    for piece in pieces:
        for start in range(0, len(piece), 16 * 1024):
            chunk = piece[start:start + 16 * 1024].decode("utf-8", "replace")
            entry = json.dumps({"log": chunk, "stream": stream, "time": "2026-09-24T07:00:00.123456789Z"},
                               ensure_ascii=True).replace("<", "\\u003c").replace(">", "\\u003e").replace("&", "\\u0026")
            total += len(entry) + 1
    return total


@pytest.mark.parametrize("stdout,stderr", [
    (b"", b""),
    (b'{"mode":"finalize"}\n', b"InconsistentVersionWarning: x\n"),
    (b'quote " backslash \\ ctrl \x01\x02\x1f <tag>&amp;\n', b"\xe4\xb8\xad\xe6\x96\x87 \xf0\x9f\x99\x82\n"),
    (b"no trailing newline", b"x" * 40_000),
    (b"\xff\xfe invalid utf-8\n" * 3, b"\n\n\n"),
    (b"\x01" * 10_000 + b"\n", b"\xff" * 10_000),       # ⚠️ 每個 byte 都跳脫成 \u00XX——固定開銷蓋不過去
], ids=["empty", "normal", "escapes", "no_newline_long_line", "invalid_utf8_blank_lines", "worst_case_escaping"])
def test_a8_log_bound_covers_json_file_encoding(stdout, stderr):
    bound = sz.log_bound(stdout, stderr)
    assert bound >= _reference_json_file(stdout, "stdout") + _reference_json_file(stderr, "stderr")
    assert bound % sz.BLOCK == 0


def test_a8_log_model_guards():
    assert sz.log_config_problems({"Type": "json-file", "Config": {}}) == []
    assert sz.log_config_problems({"Type": "local", "Config": {}})
    assert sz.log_config_problems({"Type": "json-file", "Config": {"max-size": "10m"}})
    assert sz.log_config_problems({"Type": "json-file", "Config": {"max-file": "3"}})


# ── d0：allocated bytes ─────────────────────────────────────────────────────

def test_d0_allocated_counts_blocks_and_directories(tmp_path):
    for i in range(20):
        (tmp_path / f"f{i}").write_bytes(b"x")                 # 1 byte 的檔案
    deep = tmp_path / "a" / "b" / "c" / "d"
    deep.mkdir(parents=True)                                    # 多層空目錄
    (tmp_path / "odd").write_bytes(b"y" * 5000)                 # 非整 block
    measured = sz.allocated_tree(tmp_path)
    naive = sum(p.stat().st_size for p in tmp_path.rglob("*") if p.is_file())
    assert measured["allocated"] > naive                        # ⚠️ st_size 相加會低估
    assert measured["dirs"] == 5
    assert measured["allocated"] >= sum(os.lstat(p).st_blocks * 512 for p in [tmp_path, *tmp_path.rglob("*")])


def test_d0_temp_rename_sequence_is_covered_by_the_accounting(tmp_path):
    peak = 0
    for i in range(5):
        tmp = tmp_path / f".f{i}.tmp"
        tmp.write_bytes(b"z" * (3000 + i * 7000))
        peak = max(peak, sz.allocated_tree(tmp_path)["allocated"])
        os.replace(tmp, tmp_path / f"f{i}")
        peak = max(peak, sz.allocated_tree(tmp_path)["allocated"])
    final = sz.allocated_tree(tmp_path)
    assert final["allocated"] + final["dirs"] * sz.BLOCK >= peak


def test_d0_index_and_lock_coexisting(tmp_path):
    admin = tmp_path / "admin"
    admin.mkdir()
    (admin / "index").write_bytes(b"i" * 94_808)
    (admin / "HEAD").write_bytes(b"e" * 41)
    shutil.copy(admin / "index", admin / "index.lock")          # index 與 index.lock 同時存在
    peak = sz.allocated_tree(admin)["allocated"]
    (admin / "index.lock").unlink()
    accounted = sz.allocated_tree(admin)["allocated"] + sz.allocated_tree(admin / "index")["allocated"]
    assert accounted >= peak


# ── a13：inventory ─────────────────────────────────────────────────────────

@pytest.fixture
def tree(tmp_path):
    (tmp_path / "keep").write_text("k")
    (tmp_path / "gone").write_text("g")
    (tmp_path / "flip").mkdir()
    allowed = tmp_path / "allowed"
    allowed.mkdir()                                              # ⚠️ 允許的根目錄事先建好
    (tmp_path / "src").mkdir()
    return tmp_path


def test_a13_inventory_rules(tree):
    before = sz.inventory([tree])
    (tree / "allowed" / "new").write_text("n")                  # 允許位置內新建（第一個檔案）→ OK
    assert sz.inventory_diff(before, sz.inventory([tree]), allowed=[tree / "allowed"]) == []
    (tree / "gone").unlink()
    (tree / "flip").rmdir()
    (tree / "flip").write_text("now a file")
    (tree / "keep").write_text("modified")
    (tree / "outside").write_text("o")
    (tree / "src" / "__pycache__").mkdir()
    problems = sz.inventory_diff(before, sz.inventory([tree]), allowed=[tree / "allowed"],
                                 source_roots=[tree / "src"])
    text = "\n".join(problems)
    for needle in ("刪除", "型別改變", "允許位置以外的修改", "允許位置以外的新建", "bytecode"):
        assert needle in text


def test_a13_only_the_current_phase_run_is_allowed(tmp_path):
    """L1 只放行**本路徑自己的** run 根目錄——改到別條路徑的 run 必須被擋下。"""
    runs = tmp_path / "runs"
    (runs / "witness").mkdir(parents=True)
    (runs / "witness" / "after_artifact.json").write_text("w")
    (runs / "success").mkdir()
    before = sz.inventory([tmp_path])
    (runs / "success" / "stage2").mkdir()
    (runs / "success" / "stage2" / "report.json").write_text("r")
    assert sz.inventory_diff(before, sz.inventory([tmp_path]), allowed=[runs / "success"]) == []
    (runs / "witness" / "after_artifact.json").write_text("tampered")
    problems = sz.inventory_diff(before, sz.inventory([tmp_path]), allowed=[runs / "success"])
    assert problems and "witness" in problems[0]


def test_a13_parent_mtime_is_unchanged_when_the_allowed_root_exists(tree):
    before = sz.inventory([tree])
    (tree / "allowed" / "sub").mkdir()
    (tree / "allowed" / "sub" / "x").write_text("x")
    after = sz.inventory([tree])
    assert after[str(tree)] == before[str(tree)]                # 父目錄（不在 allowlist）⛔ 不變


# ── a11b：container spec hash ───────────────────────────────────────────────

SPEC = ["--network", "none", "--cidfile", "/s/cid/o0010.cid", "--name", "i074sz-r-o0010", "-v", "/a:/app:ro",
        IMG, "sh", "-c", "w", "_", "python", "-m", "m", "--name", "x"]


def test_a11b_spec_hash_ignores_only_operation_cidfile_and_name():
    base = sz.spec_sha256(SPEC, IMG)
    # run 與 create 共用同一份 spec（operation 本來就不在 spec 裡）；只換 cidfile／name 的值 → 相同
    twin = sz.twin_argv(SPEC, IMG, cidfile="/s/cid/t0011.cid", name="i074sz-r-t0011")
    assert sz.spec_sha256(twin, IMG) == base
    changed = [
        SPEC[:1] + ["host"] + SPEC[2:],                             # option 的值
        ["--read-only"] + SPEC,                                     # 多一個 option
        SPEC[:8] + ["sha256:" + "8" * 64] + SPEC[9:],               # image（⚠️ 分界也跟著變）
        SPEC[:-1] + ["y"],                                          # 容器指令裡的參數
        SPEC[:-2] + ["--other", "x"],                               # 容器指令裡同名的 --name ⛔ 不被正規化
        SPEC[:2] + SPEC[6:8] + SPEC[2:6] + SPEC[8:],                # 順序
    ]
    for argv in changed:
        image = argv[argv.index("sha256:" + "8" * 64)] if "sha256:" + "8" * 64 in argv else IMG
        assert sz.spec_sha256(argv, image) != base, argv


def test_a11b_container_ids_are_equal_length():
    assert {len(sz.container_id(7)), *(len(sz.container_id(7, twin=k)) for k in (1, 2, 3))} == {5}


# ── a11：invocation 索引、sidecar、twin 的完整性 ──────────────────────────────

def _state(tmp_path, *, footprint_log=4096, peaks=None):
    """合成一份完整的 S：10 個 invocation（witness／success／failure 各 2、memory_only 4）。"""
    s = tmp_path / "S"
    seq = 0
    for phase in sz.PHASES:
        for role in sz.EXPECTED_ROLES[phase]:
            seq += 1
            cid = sz.container_id(seq)
            argv = ["--cidfile", f"/s/{cid}", "--name", f"n-{cid}", IMG, "run", str(seq)]
            index = sz.write_index(s, sequence=seq, cid=cid, phase=phase, role=role,
                                   included=phase in sz.DISK_PHASES, image=IMG, argv=argv)
            sz.write_exclusive(s / "containers" / f"{cid}.json", sz.canonical_dumps({
                "id": cid, "sequence": seq, "container_spec_sha256": index["container_spec_sha256"], "rc": 0,
                "container": "c", "size_rw": 0, "log_config": {"Type": "json-file", "Config": {}},
                "log_bound": footprint_log, "peak_bytes": (peaks or {}).get(seq, 100 << 20),
                "status": "ok", "failures": []}))
            sz.write_exclusive(s / "twins" / f"{cid}.json", sz.canonical_dumps({
                "id": cid, "sequence": seq, "container_spec_sha256": index["container_spec_sha256"],
                "raw_bytes": [8192] * 3, "inspect_lengths": [5000] * 3, "adopted_bytes": 131072,
                "status": "ok", "failures": []}))
            sz.record_event(s, cid, "create_begin")
            sz.record_event(s, cid, "rm_done")
    return s


def test_a11_complete_state_loads(tmp_path):
    assert len(sz.load_invocations(_state(tmp_path))) == 10


@pytest.mark.parametrize("damage,match", [
    ("missing_sidecar", "缺 sidecar"),
    ("missing_twin", "缺 twin"),
    ("extra_sidecar", "沒有索引"),
    ("gap", "不連續"),
    ("sha", "不一致"),
    ("memory_only_included", "included_in_disk_path"),
    ("sidecar_failed", "量測失敗"),
    ("size_rw", "SizeRw"),
], ids=lambda x: x if isinstance(x, str) else None)
def test_a11_damaged_state_fails_closed(tmp_path, damage, match):
    s = _state(tmp_path)
    if damage == "missing_sidecar":
        (s / "containers" / "o0030.json").unlink()
    elif damage == "missing_twin":
        (s / "twins" / "o0030.json").unlink()
    elif damage == "extra_sidecar":
        shutil.copy(s / "containers" / "o0010.json", s / "containers" / "o0990.json")
    elif damage == "gap":
        (s / "index" / "0003.json").rename(s / "index" / "0011.json")
    else:
        path = {"sha": s / "twins" / "o0020.json", "memory_only_included": s / "index" / "0008.json",
                "sidecar_failed": s / "containers" / "o0020.json", "size_rw": s / "containers" / "o0020.json"}[damage]
        entry = json.loads(path.read_text())
        if damage == "sha":
            entry["container_spec_sha256"] = "0" * 64
        elif damage == "memory_only_included":
            entry["included_in_disk_path"] = True
        elif damage == "sidecar_failed":
            entry.update(status="measure_failed", failures=["inspect 失敗"])
        else:
            entry["size_rw"] = 4096
        path.write_bytes(sz.canonical_dumps(entry))
    with pytest.raises(sz.SizingError, match=match):
        sz.load_invocations(s)


def test_a11_index_rejects_unknown_phase_role_and_inconsistent_inclusion(tmp_path):
    for kwargs in ({"phase": "other", "role": "fixture", "included": True},
                   {"phase": "success", "role": "other", "included": True},
                   {"phase": "memory_only", "role": "check", "included": True}):
        with pytest.raises(sz.SizingError):
            sz.write_index(tmp_path, sequence=1, cid="o0010", image=IMG, argv=[IMG], **kwargs)
    sz.write_index(tmp_path, sequence=1, cid="o0010", phase="success", role="fixture", included=True,
                   image=IMG, argv=[IMG])
    with pytest.raises(FileExistsError):              # 不可變：⛔ 不覆寫
        sz.write_index(tmp_path, sequence=1, cid="o0010", phase="success", role="fixture", included=True,
                       image=IMG, argv=[IMG])


def test_a11_twin_refuses_an_existing_cidfile(tmp_path):
    s = tmp_path / "S"
    sz.write_index(s, sequence=1, cid="o0010", phase="witness", role="fixture", included=True, image=IMG,
                   argv=["--cidfile", "/x", "--name", "n", IMG, "true"])
    (s / "cid").mkdir()
    (s / "cid" / "t0011.cid").write_text("stale")
    sz.run_twins(s, docker="/bin/false", run_id="r", fs_path=str(tmp_path))
    twin = json.loads((s / "twins" / "o0010.json").read_text())
    assert twin["status"] == "measure_failed" and "已存在" in twin["failures"][0]


# ── a11c：容器足跡的合併 ─────────────────────────────────────────────────────

def _containers():
    return [{"id": "o0010", "sequence": 1, "footprint": 100}, {"id": "o0020", "sequence": 2, "footprint": 300}]


@pytest.mark.parametrize("events,rule", [
    ({("o0010", "create_begin"): [1], ("o0010", "rm_done"): [2], ("o0020", "create_begin"): [3],
      ("o0020", "rm_done"): [4]}, "max"),
    ({("o0010", "create_begin"): [1], ("o0020", "create_begin"): [2], ("o0010", "rm_done"): [3],
      ("o0020", "rm_done"): [4]}, "sum"),                                          # 交錯
    ({("o0010", "create_begin"): [1], ("o0020", "create_begin"): [3], ("o0020", "rm_done"): [4]}, "sum"),  # 缺一個
    ({("o0010", "create_begin"): [1], ("o0010", "rm_done"): [2], ("o0020", "create_begin"): [2],
      ("o0020", "rm_done"): [4]}, "sum"),                                          # 事件號重複
    ({("o0010", "create_begin"): [4], ("o0010", "rm_done"): [1], ("o0020", "create_begin"): [3],
      ("o0020", "rm_done"): [5]}, "sum"),                                          # ⚠️ 同一容器反序（review 的反例）
    ({("o0010", "create_begin"): [1], ("o0010", "rm_done"): [2], ("o0020", "create_begin"): [3],
      ("o0020", "rm_done"): [4], ("o0020", "other"): [5]}, "sum"),                 # 未知事件
    ({("o0010", "create_begin"): [1, 6], ("o0010", "rm_done"): [2], ("o0020", "create_begin"): [3],
      ("o0020", "rm_done"): [4]}, "sum"),                                          # 多一筆 create_begin
])
def test_a11c_footprint_combination(events, rule):
    total, how = sz.combine_footprints(_containers(), events)
    assert (total, how.split("_")[0]) == ((300, "max") if rule == "max" else (400, "sum"))


# ── b3：record 目錄 ──────────────────────────────────────────────────────────

def test_b3_record_diff():
    root = "/w/failed"
    assert sz.record_diff(["a"], ["a", "b"], published="/w/failed/b", failed_root=root) == "/w/failed/b"
    for before, after, published in ((["a"], ["a"], "/w/failed/a"), ([], ["a", "b"], "/w/failed/a"),
                                     ([], ["a"], "/w/failed/other")):
        with pytest.raises(sz.SizingError):
            sz.record_diff(before, after, published=published, failed_root=root)


# ── d／b2：報告 ──────────────────────────────────────────────────────────────

BUDGET = 1 << 40    # 有效性條件計畫：build_report() 的 p_b_budget（夠大 ＝ 沒有預算相關的判定）

def _full_state(tmp_path, *, failure_extra=0, dev="42", docker_dev="42", rc_override=None, drop_sample=False, fs_extra=None):
    """`fs_extra`：phase → L0 的已用量另外多出的 bytes（量測範圍外的寫入；有效性條件的測試用）。"""
    s = _state(tmp_path)
    (s / "meta.tsv").write_text("".join(f"{k}\t{v}\n" for k, v in {
        "run_id": "r", "mode": "validation", "image": IMG, "l0_dev": "42", "docker_root_dev": docker_dev,
        "repo_dev": "42", "state_fstype": "tmpfs", "repo_head": "a" * 40, "clone_head": "a" * 40}.items()))
    (s / "components.tsv").write_text("".join(f"{k}\t{v}\n" for k, v in {
        "wt_head": 30 << 20, "wt_base": 15 << 20, "wt_base_patched": 15 << 20, "git_head": 131072,
        "git_base": 131072, "git_base_patched": 131072, "index_head": 98304, "index_base": 98304,
        "index_base_patched": 98304, "snapshot": 24576, "probe": 24576,
        "frozen_witness": 8192, "frozen_patched": 266240}.items()))
    steps = ("fixture_witness", "envcheck", "fixture_success", "finalize", "fixture_failure",
             "publish_failed_record", "recover_envcheck", "recover_durability", "check_failed_record",
             "recover_failed_record")
    rcs = dict.fromkeys(steps, "0\t0")
    rcs.update(rc_override or {})
    (s / "rc.tsv").write_text("".join(f"{k}\t{v}\n" for k, v in rcs.items()))
    for phase, run_mb in (("witness", 80), ("success", 85), ("failure", 1 + failure_extra)):
        pdir = s / "phases" / phase
        pdir.mkdir(parents=True)
        base = {"L1": 0, "L2": 7 << 20, "L3": 0, "L5": 1 << 20}
        (pdir / "baseline.json").write_bytes(sz.canonical_dumps({
            "locations": base, "L0_used": 10 << 30,
            "devices": {"L0": dev, "L1": "42", "L2": "42", "L3": "42", "L5": "42", "L4": "42"}}))
        samples = [] if drop_sample else [
            {"t_ns": 1, "L0": (10 << 30) + (run_mb << 20) + (fs_extra or {}).get(phase, 0), "L1": run_mb << 20,
             "L2": 7 << 20, "L3": 15 << 20, "L5": 1 << 20}]
        (pdir / "samples.jsonl").write_text("".join(json.dumps(x) + "\n" for x in samples))
        (pdir / "end.json").write_bytes(sz.canonical_dumps({
            "run_dir": {"allocated": run_mb << 20, "dirs": 3}, "archive": {"allocated": 7 << 20, "dirs": 4},
            "inventory_violations": []}))
    _manifest(s, "sizing")
    return s


def _manifest(s, profile):
    """⑦d：快照與 MANIFEST（報告綁住完整的清單 ①）。"""
    lines = []
    for rel in sz.SNAPSHOT_FILES[profile]:
        path = s / "harness" / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(f"content of {rel}".encode())
        lines.append(f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {rel}")
    (s / "harness" / "MANIFEST").write_text("\n".join(lines) + "\n")


def test_d_report_ok(tmp_path):
    report = sz.build_report(_full_state(tmp_path), p_b_budget=BUDGET)
    assert report["status"] == "ok"
    assert report["P_B"] == max(p["P_path"] for p in report["paths"].values())
    success = report["paths"]["success"]
    assert success["P_path"] >= success["accounted"] >= success["peaks"]["dirs_peak"]
    assert success["container_rule"] == "max_non_overlapping"
    assert report["host_measurement_artifact_bytes"] == 0
    assert report["docker_instrumentation_overhead"] == "included_unseparated"
    assert "代量" in report["notes"][0]
    assert sz.report_text(report).startswith("status: ok")


def test_d_assumption_violated_still_reports(tmp_path):
    report = sz.build_report(_full_state(tmp_path, failure_extra=200), p_b_budget=BUDGET)
    assert report["status"] == "assumption_violated"
    assert report["P_B"] == report["paths"]["failure"]["P_path"]       # P_B 仍取三者最大


@pytest.mark.parametrize("kwargs,match", [
    ({"dev": "7"}, "同一個檔案系統"),
    ({"docker_dev": "7"}, "docker_root_dev"),                           # b2：Docker Root Dir 在別的檔案系統
    ({"rc_override": {"finalize": "0\t1"}}, "結束碼"),
    ({"drop_sample": True}, "沒有任何取樣"),
])
def test_d_report_fails_closed(tmp_path, kwargs, match):
    with pytest.raises(sz.SizingError, match=match):
        sz.build_report(_full_state(tmp_path, **kwargs), p_b_budget=BUDGET)


@pytest.mark.parametrize("key,value,match", [
    ("clone_head", "b" * 40, "工作複本的 HEAD"), ("repo_head", "", "meta 缺少 repo_head"), ("clone_head", "", "meta 缺少 clone_head"),
])
def test_d_report_binds_clone_head_to_repo_head(tmp_path, key, value, match):
    """⑦d 實作第一輪 review #1：sizing 的報告也要驗 clone_head ＝ repo_head（兩者都必須存在）。"""
    s = _full_state(tmp_path)
    meta = sz._read_tsv(s / "meta.tsv")
    meta[key] = value
    (s / "meta.tsv").write_text("".join(f"{k}\t{v}\n" for k, v in meta.items()))
    with pytest.raises(sz.SizingError, match=match):
        sz.build_report(s, p_b_budget=BUDGET)


def test_d_report_fails_closed_on_missing_component_and_aborted_window(tmp_path):
    s = _full_state(tmp_path)
    lines = [l for l in (s / "components.tsv").read_text().splitlines() if not l.startswith("probe")]
    (s / "components.tsv").write_text("\n".join(lines) + "\n")
    with pytest.raises(sz.SizingError, match="probe"):
        sz.build_report(s, p_b_budget=BUDGET)
    s2 = _full_state(tmp_path / "again")
    (s2 / "phases" / "success" / "aborted").touch()
    with pytest.raises(sz.SizingError, match="aborted"):
        sz.build_report(s2, p_b_budget=BUDGET)


def test_d_report_rejects_events_of_unknown_containers(tmp_path):
    """未知容器 ID 的 lifecycle event ⛔ 不得被忽略（review：它會讓 combine_footprints 錯用 max）。"""
    s = _full_state(tmp_path)
    sz.record_event(s, "o9990", "create_begin")
    with pytest.raises(sz.SizingError, match="不屬於 invocation 索引"):
        sz.build_report(s, p_b_budget=BUDGET)


def test_d_report_rejects_duplicate_event_numbers_across_phases(tmp_path):
    """witness 與 success 各有一個相同的事件號——過濾之後各自唯一，⛔ 必須在過濾之前就被抓到。"""
    s = _full_state(tmp_path)
    lines = (s / "events" / "events.jsonl").read_text().splitlines()
    witness_rm = json.loads(lines[3])            # o0020 的 rm_done（witness）
    success_line = json.loads(lines[4])          # o0030 的 create_begin（success）
    success_line["event"] = witness_rm["event"]
    lines[4] = json.dumps(success_line)
    (s / "events" / "events.jsonl").write_text("\n".join(lines) + "\n")
    with pytest.raises(sz.SizingError, match="事件號重複"):
        sz.build_report(s, p_b_budget=BUDGET)


def test_d_events_are_filtered_per_phase_before_combining(tmp_path):
    report = sz.build_report(_full_state(tmp_path), p_b_budget=BUDGET)
    assert {p["container_rule"] for p in report["paths"].values()} == {"max_non_overlapping"}


# ── 有效性條件（issue.md I-074「Stage 2 量測的有效性條件與 ⑨-1 fail-fast 計畫」「六」的 pytest：有效性） ─────────────

T = sz.FS_UNEXPLAINED_TOLERANCE


def _basis(tmp_path):
    """每條路徑的 P_basis 與 fs_peak（基準狀態，⛔ 沒有量測範圍外的寫入）。"""
    report = sz.build_report(_full_state(tmp_path / "base"), p_b_budget=BUDGET)
    return {ph: sz.disk_numbers(report["paths"][ph]) for ph in sz.DISK_PHASES}


def _fs_for(base, phase, unexplained):
    """讓 phase 的 fs_unexplained 恰好 ＝ unexplained 的 fs_extra。"""
    return {phase: base[phase]["P_basis"] - base[phase]["fs_peak"] + unexplained}


def test_validity_fs_unexplained_boundary(tmp_path):
    base = _basis(tmp_path)
    assert all(d["fs_unexplained"] < 0 for d in base.values())             # 基準：會計上界成立（負值有效）
    report = sz.build_report(_full_state(tmp_path / "eq", fs_extra=_fs_for(base, "success", T)), p_b_budget=BUDGET)
    assert sz.disk_numbers(report["paths"]["success"])["fs_unexplained"] == T  # ＝ 容差：有效
    with pytest.raises(sz.SizingError, match=r"量測無效.*success.*fs_unexplained.*1048577"):
        sz.build_report(_full_state(tmp_path / "over", fs_extra=_fs_for(base, "success", T + 1)), p_b_budget=BUDGET)


def test_validity_ambiguous_exceed_is_invalid_not_a_verdict(tmp_path):
    """模糊區：P_path ＞ 預算但 P_basis ≤ 預算——容差⛔ 不得決定結果 → 無效（可以重跑），⛔ 不是超標的判定。"""
    base = _basis(tmp_path)
    budget = base["success"]["P_basis"]                                     # P_basis ＝ 預算（⛔ 沒有超標）
    state = _full_state(tmp_path / "amb", fs_extra=_fs_for(base, "success", 1))   # fs_peak ＝ 預算 ＋ 1（容差內）
    with pytest.raises(sz.SizingError, match="模糊區"):
        sz.build_report(state, p_b_budget=budget)
    report = sz.build_report(_full_state(tmp_path / "at"), p_b_budget=budget)    # P_path ＝ 預算 → 有效、沒有超標
    assert report["P_B"] == budget and sz.sizing_validity_problems(report, budget) == []


def test_validity_ambiguous_requires_the_unexplained_within_tolerance(tmp_path):
    """實作第一輪 review：模糊區 ＝「超標**完全由容許範圍內**的 fs_unexplained 造成」——fs_unexplained 已超過容差時只是 ①，
    ⛔ 不同時記成模糊區（原因分類要精確）。"""
    budget = 160 << 20
    def disk(unexplained):
        return {"success": {"P_path": budget + unexplained, "dirs_peak": 1, "fs_peak": budget + unexplained,
                            "accounted": budget, "P_basis": budget, "fs_unexplained": unexplained}}
    assert [p["kind"] for p in sz.disk_validity_problems(disk(T), budget)] == ["ambiguous_disk_exceed"]
    assert [p["kind"] for p in sz.disk_validity_problems(disk(T + 1), budget)] == ["fs_unexplained"]
    base = _basis(tmp_path)
    state = _full_state(tmp_path / "overlap", fs_extra=_fs_for(base, "success", T + 1))
    with pytest.raises(sz.SizingError) as err:
        sz.build_report(state, p_b_budget=base["success"]["P_basis"])
    assert "fs_unexplained" in str(err.value) and "模糊區" not in str(err.value)


def test_validity_firm_exceed_wins_and_the_report_is_still_written(tmp_path):
    """確定的違反優先：P_basis ＞ 預算 → 照常寫報告（P_B ＞ 預算，freeze record 照舊拒寫），同一次的有效性問題列在文字版。"""
    base = _basis(tmp_path)
    budget = base["success"]["P_basis"] - 1
    state = _full_state(tmp_path / "mixed", fs_extra=_fs_for(base, "witness", 3 * T))
    report = sz.build_report(state, p_b_budget=budget)
    assert report["P_B"] > budget
    problems = sz.sizing_validity_problems(report, budget)
    assert [(p["subject"], p["kind"]) for p in problems] == [("witness", "fs_unexplained")]
    text = sz.report_text(report, validity_problems=problems)
    assert "有效性問題" in text and "P_basis=" in text and "fs_unexplained=" in text
    report2 = sz.build_report(_full_state(tmp_path / "firm"), p_b_budget=budget)   # 只有確定的超標
    assert report2["P_B"] > budget and sz.sizing_validity_problems(report2, budget) == []


def test_validity_build_report_requires_the_budget(tmp_path):
    with pytest.raises(TypeError):
        sz.build_report(_full_state(tmp_path))                              # ⛔ 沒有預設值：呼叫端一律傳正式常數


def test_failure_summary_lists_leftover_containers(tmp_path):
    s = tmp_path / "S"
    (s / "phases" / "success").mkdir(parents=True)
    (s / "phases" / "success" / "aborted").touch()
    leftovers = tmp_path / "left"
    leftovers.write_text("cid-b\ncid-a\ncid-b\n")
    out = tmp_path / "summary.json"
    assert sz.main(["failure-summary", "--state", str(s), "--stage", "x", "--rc", "1", "--out", str(out),
                    "--leftover-cids", str(leftovers)]) == 0
    summary = json.loads(out.read_text())
    assert summary["leftover_containers"] == ["cid-a", "cid-b"] and summary["P_B"] is None
    assert summary["phases"] == {"success": "aborted"}


def test_d_delta_is_clamped_per_location(tmp_path):
    baseline = {"locations": {"L1": 100, "L2": 100, "L3": 100, "L5": 100}, "L0_used": 1000}
    peaks = sz.phase_peaks(baseline, [{"L0": 900, "L1": 50, "L2": 300, "L3": 100, "L5": 100}])
    assert peaks["dirs_peak"] == 200          # L1 的減少⛔ 不抵銷 L2 的增加
    assert peaks["fs_peak"] == -100


# ── twin 的 docker rm 失敗：保留 CID，外層以 rm -f 收尾（review 中高） ───────────

def _fake_docker(tmp_path, *, rm_fails=False, rm_force_fails=False):
    log = tmp_path / "docker.log"
    script = tmp_path / "docker"
    script.write_text(f"""#!/usr/bin/env bash
printf '%s\\n' "$*" >> {log}
case "$1" in
  create)
    shift; while [ "$#" -gt 0 ]; do [ "$1" = --cidfile ] && {{ printf 'cid-%s' "$(basename "$2" .cid)" > "$2"; break; }}; shift; done
    exit 0 ;;
  inspect) echo '[{{"Id":"x"}}]' ;;
  rm)
    if [ "$2" = -f ]; then {"exit 1" if rm_force_fails else "exit 0"}; fi
    {"exit 1" if rm_fails else "exit 0"} ;;
esac
exit 0
""")
    script.chmod(0o755)
    return script, log


def test_twin_rm_failure_keeps_the_cid_for_forced_cleanup(tmp_path):
    s = tmp_path / "S"
    sz.write_index(s, sequence=1, cid="o0010", phase="witness", role="fixture", included=True, image=IMG,
                   argv=["--cidfile", "/x", "--name", "n", IMG, "true"])
    docker, log = _fake_docker(tmp_path, rm_fails=True)
    sz.run_twins(s, docker=str(docker), run_id="r", fs_path=str(tmp_path))
    twin = json.loads((s / "twins" / "o0010.json").read_text())
    assert twin["status"] == "measure_failed" and "docker rm 失敗" in twin["failures"][0]
    assert twin["twin_containers"] == ["cid-t0011"]
    assert (s / "cid" / "t0011.cid").read_text() == "cid-t0011"         # ⚠️ cidfile 保留
    assert sz.cleanup_cids(s, docker=str(docker)) == []                  # 外層的失敗清理
    assert "rm -f cid-t0011" in log.read_text().splitlines()
    assert not (s / "cid" / "t0011.cid").exists()


def test_twins_only_fill_in_missing_invocations(tmp_path):
    """有效性條件計畫「二」：full 模式在量測趟之前先建一次 twin、量測趟之後再補它那一個——已有的⛔ 不重建。"""
    s = tmp_path / "S"
    argv = ["--cidfile", "/x", "--name", "n", IMG, "true"]
    sz.write_index(s, sequence=1, cid="o0010", phase="witness", role="fixture", included=True, image=IMG, argv=argv)
    docker, log = _fake_docker(tmp_path)
    sz.run_twins(s, docker=str(docker), run_id="r", fs_path=str(tmp_path))
    first = (s / "twins" / "o0010.json").read_bytes()
    creates = [line for line in log.read_text().splitlines() if line.startswith("create")]
    assert len(creates) == sz.TWIN_COUNT
    sz.write_index(s, sequence=2, cid="o0020", phase="witness", role="finalizer", included=True, image=IMG, argv=argv)
    sz.run_twins(s, docker=str(docker), run_id="r", fs_path=str(tmp_path))
    creates = [line for line in log.read_text().splitlines() if line.startswith("create")]
    assert len(creates) == 2 * sz.TWIN_COUNT and all("t002" in line for line in creates[sz.TWIN_COUNT:])
    assert (s / "twins" / "o0010.json").read_bytes() == first                # 已有的⛔ 不重建
    assert json.loads((s / "twins" / "o0020.json").read_text())["status"] == "ok"


def test_cleanup_reports_containers_it_could_not_remove(tmp_path):
    s = tmp_path / "S"
    (s / "cid").mkdir(parents=True)
    (s / "cid" / "o0010.cid").write_text("cid-o0010")
    docker, _ = _fake_docker(tmp_path, rm_force_fails=True)
    assert sz.cleanup_cids(s, docker=str(docker)) == ["cid-o0010"]
    assert (s / "cid" / "o0010.cid").exists()                            # ⛔ 不吞掉、⛔ 不刪 CID


def test_twin_success_removes_its_cidfiles(tmp_path):
    s = tmp_path / "S"
    sz.write_index(s, sequence=1, cid="o0010", phase="witness", role="fixture", included=True, image=IMG,
                   argv=["--cidfile", "/x", "--name", "n", IMG, "true"])
    docker, _ = _fake_docker(tmp_path)
    sz.run_twins(s, docker=str(docker), run_id="r", fs_path=str(tmp_path))
    assert json.loads((s / "twins" / "o0010.json").read_text())["status"] == "ok"
    assert not list((s / "cid").glob("t*.cid"))


# ── 差異 1 的常駐上限：佇列 1 列 ＋ 兩端各 1 列（review 低） ──────────────────────

def test_streamed_rows_keeps_at_most_three_rows(monkeypatch, tmp_path):
    from ..replay_bundle import stream as st

    produced = [0]

    def fake_stream(path, kind, *, on_row, **kwargs):
        for i in range(500):
            produced[0] += 1
            on_row({"i": i}, b"")

    monkeypatch.setattr(st, "stream_canonical_artifact", fake_stream)
    consumed, worst = 0, 0
    for _row in sz._streamed_rows(tmp_path / "x.json"):
        consumed += 1
        import time as _t
        _t.sleep(0.0005)                  # 讓讀取端有機會往前跑
        worst = max(worst, produced[0] - consumed)
    assert consumed == 500 and worst <= 2   # 已產生但尚未被取走的 ≤ 2（佇列 1 ＋ 讀取端手上 1）
