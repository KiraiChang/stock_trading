#!/usr/bin/env python3
"""把 shell 量到的 peak／host low／cgroup limit 寫成 probe 的 measurement ＋ completion。

⚠️ **為什麼由 Python 寫**：這兩份是受管制 artifact，要走**與其他產物同一套** canonical JSON
與 schema validator。⛔ 讓 shell 自己組 JSON 就是雙真相源。

⚠️ **provenance 從 computation 讀出來**：三份必須是**同一個 runner invocation 的外層產物**，
而 `project_modules_sha256` 只有容器內的 Python 算得出來——shell ⛔ 無從複製，只能沿用。

⚠️ **completion 最後才寫**：量測缺任一項就⛔ 不發布它，於是中止時⛔ 不會留下一份
**看起來完整**的 probe。
"""
from __future__ import annotations

import argparse
import sys
from datetime import datetime, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from _i074_bootstrap import load_replay_bundle  # noqa: E402


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(allow_abbrev=False)
    parser.add_argument("--computation", required=True, help="capacity_probe_computation.json")
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--peak-rss-bytes", type=int, required=True)
    parser.add_argument("--host-low-bytes", type=int, required=True)
    parser.add_argument("--cgroup-limit-bytes", type=int, required=True)
    args = parser.parse_args(argv)

    mods = load_replay_bundle()
    artifacts, canonical = mods["artifacts"], mods["canonical"]
    # ⚠️ probe 模組不在 bootstrap 的清單裡（它 import i074_preflight），另外載入。
    import importlib.util

    root = Path(__file__).resolve().parent.parent / "backtest/modular/sr_scoring/replay_bundle"
    for name in ("i074_preflight", "probe"):
        key = f"_i074_rb.{name}"
        if key not in sys.modules:
            spec = importlib.util.spec_from_file_location(key, root / f"{name}.py")
            module = importlib.util.module_from_spec(spec)
            sys.modules[key] = module
            spec.loader.exec_module(module)
    probe = sys.modules["_i074_rb.probe"]

    try:
        computation_load = artifacts.load_canonical_evidence_artifact(
            args.computation, probe.PROBE_COMPUTATION_KIND
        )
        computation = computation_load.parsed
        probe.validate_probe_computation(computation)

        now = datetime.now(timezone.utc).isoformat()
        common = {
            "schema_version": artifacts.ARTIFACT_SCHEMA_VERSION,
            "bundle_id": computation["bundle_id"],
            "generated_at": now,
            # ⚠️ 沿用 computation 的 provenance——⛔ 不各自虛構一份執行身分。
            "provenance": computation["provenance"],
        }
        measurement = dict(
            common,
            kind=probe.PROBE_MEASUREMENT_KIND,
            peak_rss_bytes=args.peak_rss_bytes,
            host_low_bytes=args.host_low_bytes,
            cgroup_limit_bytes=args.cgroup_limit_bytes,
        )
        # ⛔ 量不到就會在這裡被擋下（正整數 bytes，⛔ 不接受 0 佔位）。
        probe.validate_probe_measurement(measurement)

        out = Path(args.output_dir)
        measurement_blob = canonical.canonical_json_bytes(measurement)
        artifacts.write_atomic(out, probe.PROBE_MEASUREMENT_NAME, measurement_blob)

        completion = dict(
            common,
            kind=probe.PROBE_COMPLETION_KIND,
            completed=True,
            computation_artifact_sha256=computation_load.artifact_sha256,
            measurement_artifact_sha256=canonical.sha256_hex(measurement_blob),
        )
        probe.validate_probe_completion(
            completion,
            computation_sha256=computation_load.artifact_sha256,
            measurement_sha256=canonical.sha256_hex(measurement_blob),
        )
        # ⚠️ **最後**才寫 completion。
        artifacts.write_atomic(
            out, probe.PROBE_COMPLETION_NAME, canonical.canonical_json_bytes(completion)
        )
    except Exception as exc:  # noqa: BLE001
        print(f"ERROR: {type(exc).__name__}: {exc}", file=sys.stderr)
        return 1
    print(f"{probe.PROBE_MEASUREMENT_NAME} 與 {probe.PROBE_COMPLETION_NAME} 已寫入 {args.output_dir}",
          file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
