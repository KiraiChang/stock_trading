# T-077 計畫 v3：I-074 Stage 2 工具與漂移測試的退役

> 狀態：⚠️ 待使用者確認（v3，2026-10-30；D1～D6 已依 v1 的 review 定案，見「七」）。確認之前⛔ 不實作。
> 對應：[`todo.md`](../todo.md) T-077。

## 一、目標與不做的範圍

需求（2026-10-30 使用者確認）：

* **拿掉凍結**：Python 產品碼可以修改，`python/scripts/test.sh` 照樣能過。
* **刪除 Stage 2 的執行工具**：patch 產生器、⑩ 的入口與 supervisor、finalizer 的 shell 入口、晉升、sizing／acceptance harness 與 observer、⑨ 的兩道 guard，
  以及只測它們的測試。
* **保留 B／C 判讀器**與 image 的還原腳本（使用者裁決：保留重跑判讀的能力）。
* **解凍之後的收尾**：
  * 改寫凍結集合裡的 I-074 舊引用（I-074 收斂 D1 留下的）；
  * T-073 的兩個 Python 註解一併改（使用者裁決：併入）。

不做：

* I-120（使用者裁決：另做）。只更新它的處置方向裡「要等 T-077」那一句。
* 交易、決策邏輯與任何產品行為：產品碼**只改註解**。
* `replay_bundle/`、`evaluation.py` 裡 Stage 2 的程式（反事實路徑、`stage2_archive` 等）⛔ 不刪：
  保留的 `run-replay-offline.sh` 仍在使用，`evaluation.py` 也 import 它們（D2）。
* Stage 0、Stage 1、I-100 的工具與它們的測試維持原狀：
  * `run-evaluation.sh`、`run-replay-offline.sh`、`smoke-replay-offline.sh` 的一般段；
  * `pin-replay-image.sh`、`compare-replay-crossday.sh`、`finalize-evidence.sh`、`run-i074-stage1.sh`。

  它們⛔ 不在凍結範圍內。唯一的例外是 smoke 的反事實段（D6）。
* I-118（worktree 洩漏）：成因在保留的工具裡，另依 I-118 處理。
* `python/baselines/**`（證據、兩份 patch、bundle）一個位元組都⛔ 不改。
* ⛔ 不碰 live、⛔ 不重跑 Stage 2。

## 二、現況：凍結怎麼發生

* **漂移測試**是 `scripts/test-replay-args.sh` 的「tooling patch 漂移測試（真正 repo 的 index）」段，由 `python/scripts/test.sh` 呼叫。
  它以 `git write-tree` 的結果呼叫 `scripts/make-i074-tooling-patch.sh --verify`。
* 產生器做三件事：
  * 從 `e1cbbbd` 重算 T1、T2；
  * 驗四條不變條件，其中 ④ 要求 `python/` 扣掉排除的路徑之後與 `e1cbbbd` 完全相同；
  * 把結果與 index 中的 `tooling_e1cbbbd.patch` 逐位元比對。
* 所以：
  * 改產品碼 → ④ 不成立 → 失敗；
  * 改 `evaluation.py` 或 `replay_bundle/` → 重算的 patch 和封存的不同 → 失敗。
* **第二個會失敗的地方**：`scripts/smoke-replay-offline.sh` 的反事實段也用產生器，從工作樹重新產生 tooling patch。
  這一段只在 `REPLAY_SMOKE=1` 時才跑。
* 其他測試⛔ 不檢查「產品碼與 `e1cbbbd` 相同」（2026-10-30 grep 確認）。
  `e1cbbbd` 與兩份 patch 的非註解引用只出現在三類地方：
  * 要刪的工具和測試；
  * `run-replay-offline.sh` 的反事實模式：它讀的是已封存的 patch 檔，⛔ 不讀 HEAD 的產品碼；
  * smoke 的反事實段。

**判讀器**（`scripts/judge-i074-stage2.sh`）的流程：

1. 讀 evidence manifest 裡的 `base_commit`（`832f678`）；
2. 在這個 commit 建暫存複本；
3. 執行**複本裡的** `finalize-stage2-evidence.sh --verify-promotion-staging … --judge`。

HEAD 這一側只用到四樣東西：判讀器本身（它會驗自己的內容等於 HEAD）、`/usr/bin/git`、`/usr/bin/python3`、image。
⛔ 不需要 HEAD 裡的 finalizer 或 `replay_bundle/`。

判讀器目前的測試在 `scripts/test-i074-stage2.sh` 裡，接在晉升的合成情境之後。內容分兩類：

* 11 個執行期案例：
  * 正常判讀，包括 HEAD 的錨點和驗證入口都已改壞、照樣以 `base_commit` 那一版判讀；
  * 驗證模式確實在暫存複本裡、以 `--judge` 驗真正 repo 的 evidence；
  * 呼叫者 PATH 前面放的 fake git、docker、python3 都⛔ 沒有被執行；
  * 拒絕判讀的情況：用 `bash <script>` 啟動、帶參數、`REPLAY_IMAGE_ID` 格式錯、腳本內容和 HEAD 不同、`base_commit` 不可達、`base_commit` 的樹缺錨點、驗證模式不通過；
  * 同一份 evidence 再判一次（唯讀、可重跑）。
* 靜態檢查：
  * sed 副本和正式檔只差常數行；
  * shebang 是 `#!/bin/bash -p`；
  * PATH 只指派一次常數；
  * 信任根常數和 orchestrator 相同。

**還原腳本**（`scripts/restore-replay-image.sh --stage 2 <bundle>`）用到的東西，2026-10-30 確認都還在：

* HEAD 的 `python/scripts/ensure-i074-run-identity.py --peek` 與 `scripts/lib/image-tarball.sh`；
* repo 外的 identity：`~/.local/share/stock_trading/i074_stage2/run_identity.json`；
* tarball：同一目錄的 `images/2a90ad1c….tar`，旁邊有 `.sha256`；
* bundle：repo 裡的 `python/baselines/b1_20260901_1d_74350966_5d7ecb10`。

## 三、處置

### 分層

| 層 | 內容 | 處置 |
|---|---|---|
| L1 凍結機制 | 漂移測試；產生器 `make-i074-tooling-patch.sh` 與它的四條不變條件測試 | 刪 |
| L2 ⑦～⑩ 的執行層 | 下面的「刪除清單」 | 刪 |
| L3 library（產品碼） | `replay_bundle/`、`evaluation.py` | 只改註解（「五」） |
| L4 Stage 0／1／I-100 的工具 | 見「一」的「不做」 | ⛔ 不動。只拿掉它們對 L1、L2 的呼叫，以及 smoke 的反事實段（D6） |
| L5 判讀與還原 | `judge-i074-stage2.sh`、`restore-replay-image.sh`、`lib/image-tarball.sh`、`pin-replay-image.sh`（含 `--stage 2`）、`ensure-i074-run-identity.py` | 保留。唯一的修改：判讀器常數區塊的註解改指新的測試（D1）；程式⛔ 不改 |

### 刪除清單

**`scripts/`**：14 個檔，約 8,000 行。

| 檔 | 是什麼 |
|---|---|
| `make-i074-tooling-patch.sh` | L1 的產生器 |
| `run-i074-stage2.sh`、`lib/i074-stage2-supervisor.py`、`lib/i074-stage2-docker-label-shim.sh` | ⑩ 的入口、supervisor、label shim |
| `finalize-stage2-evidence.sh` | finalizer 的 shell 入口。Python 那一半在 `stage2_archive`，屬 L3 |
| `i074-stage2-sizing.sh`、`i074-stage2-acceptance.sh`、`lib/i074-stage2-measure.sh`、`lib/i074-sizing-docker-shim.sh` | ④、⑦d 的 harness 與 shim |
| `i074-step9-guards.sh`、`i074-step9-image.sh` | ⑨ 的兩道 guard |
| `test-i074-stage2.sh`、`tests/test_i074_stage2_host.py`、`test-i074-step9.sh` | 只測上面這些檔。其中判讀器的部分先搬到新的測試（D1）再刪 |

**`python/scripts/`**：16 個檔（含 fixture），約 10,400 行。

| 檔 | 是什麼 |
|---|---|
| `i074_stage2_freeze_record.py`、`i074_stage2_preflight.py`、`i074_stage2_promote.py`、`i074_stage2_sizing.py`（含 ⑩ 的 observer）、`i074_stage2_replay_stub.py`、`i074_stage2_rss_wrapper.py`、`i074-stage2-patch-claims.py` | ⑩ 呼叫圖與 harness 的 helper |
| `i074_differential_guard.py`、`i074_decision_summary_producer.py`、`i074_tooling_guard.py`、`i074_step9_runs.py` | ⑨ 的 guard 與完整驗證 |
| `make_counterfactual_smoke_bundle.py` | 只有 smoke 的反事實段與 ⑨ 使用 |
| `fixtures/i074_differential_guard_cases.json`、`fixtures/i074_step9/`（2 個檔） | 上面這些檔的 fixture |

**`python/backtest/modular/sr_scoring/tests/`**：9 個檔，約 5,400 行：
`test_i074_stage2_acceptance.py`、`test_i074_stage2_freeze_record.py`、`test_i074_stage2_preflight.py`、`test_i074_stage2_promote.py`、
`test_i074_stage2_sizing.py`、`test_i074_step9_runs.py`、`test_i074_tooling_guard.py`、`test_i074_differential_guard.py`、
`test_counterfactual_smoke_fixture.py`。

### 新增

| 檔 | 內容 |
|---|---|
| `scripts/test-judge-i074-stage2.sh` | 判讀器的 bootstrap 測試（D1）。由 `python/scripts/test.sh` 在原本呼叫 `test-i074-stage2.sh` 的位置呼叫 |

### 刪除的驗收條件（v1 的 review #2）

刪除之後，**HEAD 上⛔ 沒有未處理的執行依賴**：保留的程式與測試⛔ 不得執行、source 或 import 任何被刪的檔案。

⛔ 不以「被刪的檔名在 `docs/` 以外 0 筆」驗收——保留的檔案本來就有幾類合法的提及。
實作時把被刪檔名在保留檔案裡的每一筆命中逐一歸類，**每一筆都要落在下列其中一類**，並在結果裡列出：

| 類 | 內容 | 處置 |
|---|---|---|
| E1 歷史複本的呼叫 | 判讀器在 `base_commit` 暫存複本裡檢查並執行的 `finalize-stage2-evidence.sh` | ⛔ 不改（執行的是 `832f678` 的版本，⛔ 不是 HEAD） |
| E2 封存的內容 | `python/baselines/**` | ⛔ 不改 |
| E3 字串（R5） | `envcheck.py` 的 `RECOVER_HINT`、`stage2_archive.py` 的兩個 `RECOVER_*_HINT`；fixture `stage2_finalizer_argv.json` 的說明字串 | ⛔ 不改。只會出現在 Stage 2 的修復錯誤訊息裡，而那條路徑的入口已刪 |
| E4 L3 的註解 | `envcheck.py`、`stage2_archive.py` 裡描述 finalizer shell 層的註解（2026-10-30 數到 5 處） | ⛔ 不改。sr 註明這些註解指的是 `c6c2481` 的版本 |
| E5 保留工具的註解與說明 | `lib/replay-args.sh` 列使用者的註解；判讀器常數區塊的註解；`python/scripts/test.sh` 的檔頭；smoke 的檔頭 | 改成現況（只改註解或說明） |

任何一筆落不進上表的命中，就是未處理的依賴：先停下回報，⛔ 不擴大例外。

動態的驗收：`python/scripts/test.sh`、保留的 smoke（D6）、新的判讀器測試都要通過。

### 要修改的保留檔

| 檔 | 修改 |
|---|---|
| `python/scripts/test.sh` | `test-i074-stage2.sh` 的呼叫換成 `test-judge-i074-stage2.sh`；檔頭的 `SKIP_SHELL_TESTS` 說明同步 |
| `scripts/test-replay-args.sh` | 見「四」 |
| `scripts/smoke-replay-offline.sh` | 照 D6 刪除反事實的全部相關部分；一般 Stage 1／2 的段⛔ 不動 |
| `scripts/lib/replay-args.sh` | 只改一處列出使用者的註解（E5） |
| `scripts/judge-i074-stage2.sh` | 只改常數區塊那一行註解，改指新的測試（E5） |
| `python/scripts/_i074_bootstrap.py` | `STAGE2_ARCHIVE_MODULES` 只有 `i074-stage2-patch-claims.py` 和 smoke 的反事實段在用；兩者都拿掉之後，這個常數連同註解一起刪 |
| `python/backtest/modular/sr_scoring/tests/test_replay_counterfactual.py` | 只改檔頭 docstring（D6）：拿掉「真實產品資料流的非空翻轉由 smoke 負責」，改寫成這層覆蓋已隨 T-077 移除、⛔ 沒有替代 |
| `.gitignore` | 刪掉 `python/baselines/i074_stage2/.promote-staging-*`（晉升程序已刪） |
| `.gitattributes` | 刪掉 `fixtures/i074_step9/` 那一條；`python/baselines/i074_stage2/**` 的兩條⛔ 不動（它們保護證據的 bytes） |

## 四、`scripts/test-replay-args.sh` 的段落

| 段 | 處置 |
|---|---|
| I-100 的各段、`run-evaluation.sh`、`run-replay-offline.sh`（含 Stage 2 的結束碼 6） | 保留 |
| host 端 run identity validator、comparator、Stage 1 argv、`pin-replay-image.sh` 的三條分支、Stage 1 的 finalizer、Stage 1 的 orchestrator | 保留 |
| `.gitignore` 的 staging 規則 | 刪（規則本身也一起刪） |
| `finalize-stage2-evidence.sh` 的 argv、mount 與模式 | 刪。⚠️ `S2_TD`、`S2_IMG`、`S2_BUNDLE`、`S2_ID` 這幾個共用變數定義在這一段，後面保留的段也會用到，要移到保留的位置 |
| `.gitattributes` 的巢狀規則 | 保留（保護證據的 bytes） |
| pin 的專用 tag、`pin-replay-image.sh --stage 2`、`run-replay-offline.sh` 的 `I074_STAGE`、見證趟的 E7 前置守門 | 保留 |
| sizing 的 shim、參數、raw-failed freeze record；acceptance 的 profile、參數、快照 | 刪 |
| Stage 2 反事實的 argv（`run-replay-offline.sh` 的 dry-run）、canonical diff、`replay_args_path_clean`／`replay_args_remove_worktrees_under` | 保留（測的是保留的 `run-replay-offline.sh` 與 `lib/replay-args.sh`）；把錯誤訊息裡指向產生器的提示改掉 |
| 產生器的四條不變條件、漂移測試、`test-i074-step9.sh` 的呼叫 | 刪 |
| 新增：封存證據的固定值檢查（D5） | 加 |

刪完之後的驗證：

* 跑 `bash -n`，再把整支跑完。
* 保留的每一段，印出的 `ok` 數要和刪除之前相同。實作前先記下每一段的基準。
* 在 `IMAGE_REQUIRED=1` 下，需要 image 的段⛔ 不得 skip。

## 五、解凍之後的註解改寫

範圍：

* **凍結集合**（`python/` 扣掉 `python/baselines`、`python/scripts`、`sr_scoring/tests`）裡，指向 issue.md I-074、但⛔ 沒帶 `20eacb2` 的註解。
  * 2026-10-30 數到 24 行，分布在 `decision_engine.py`、`evaluation.py`、`lifecycle_engine.py`、`replay_bundle/` 等。
  * 實作時再列出完整清單。
* **T-073 的兩個註解**：`python/backtest/modular/backtester.py`、`python/backtest/modular/exits/structural_stop.py`。
  改指 T-073 表中的「現在的位置」。
* E4 的註解⛔ 不在這裡改（見「三」）。

規則比照 I-074 收斂的 R1／R2：

* **R1**：規格指標改指向主題文件。sr「程式註解裡的 I-074 舊引用」的對照表就是 R1 的對應。
* **R2**：其餘的引用改成 `git show 20eacb2:docs/issue.md` 的 I-074「…」。
* Stage 0、Stage 1 的計畫書照 `86241a8` 的例外處理。
* 只寫章節名的簡寫（例如「v29「六、1」」）⛔ 不逐行改，沿用 sr 的總說明。
* 程式裡的字串（R5）⛔ 不改。

驗證：

* 每一個 `.py`，去掉 docstring 之後的 AST 要和改之前完全相同（比照收斂階段 2 的作法）。
* 改完之後，凍結集合裡寫「issue.md I-074」而沒帶 `20eacb2` 的行要是 0 行。

## 六、文件

**歷史引用的固定 commit：`c6c2481`。**
被刪的檔案在 `c6c2481` 與階段 A 的前一個 commit 完全相同；實作時以 `git diff --quiet` 對被刪的路徑確認。
文件裡寫成「`c6c2481` 的 `<路徑>`」或「`c6c2481` 的 `<路徑>` 第 N 行」。這個形式⛔ 不是 `檔名:行號`，`check-doc-refs.py` 不會拿目前的工作樹驗它。

| 文件 | 變更 | 階段 |
|---|---|---|
| `issue.md` I-119、I-121 | 5 處指向被刪檔案的 `檔名:行號`：I-119 指 `i074_stage2_sizing.py` 的 3 處、I-121 指 `i074-stage2-acceptance.sh` 的 2 處。改成 `c6c2481` 的歷史引用，行號照原值（v1 的 review #1：不改的話，階段 A 的 `check-doc-refs.py` 會報 5 個錯誤）。狀態照 D4 處理 | A |
| `development-workflow.md` | ① 「I-074 Stage 2 的 tooling patch」：版控流程與漂移測試改寫成「已於 T-077 退役」。留下三點：兩份 patch 是封存的證據、⛔ 不再重新產生、D5 的固定值檢查。② 被刪入口的程序照 D3 加退役標記。③ **判讀與 image 還原的程序獨立成一節，可以照著執行**（D3 的界線），內容含前置條件、命令與預期輸出。④ 「I-074 Stage 2 的測試落點」：表改成刪除之後剩下的落點；「真實產品資料流的非空翻轉」那一列改成「已隨 T-077 移除、⛔ 沒有替代」（D6）。⑤ 測試規則裡「隔離 clone 之前先確認可用空間 ≥ 1.3 GB」：只為被刪的測試而寫的部分一併更新（T-072） | A |
| `sr-zone-scoring.md` | ① 「反事實 replay 路徑與 tooling patch」「兩道 guard 與兩份 patch 的封存」：封存規則改寫成 I-074 之後的現況——兩份 patch 與 evidence 是已封存的證據，⛔ 不改、⛔ 不重新產生；「⑨、⑨-1、⑨-2 全部重做」隨 I-074 結束、產生器刪除而失效；E4 的註解指 `c6c2481` 的版本。② 「晉升與 B／C 判讀器」：寫明判讀器的依賴（判讀器本身、git、python3、image）和測試的位置（D1）。③ 「容量驗收」：照 D4 補 I-119 的判讀注意事項 | A |
| `sr-zone-scoring.md` | 「程式註解裡的 I-074 舊引用（凍結集合）」：註解改完之後，拿掉凍結的說明與對照表；保留簡寫與 `86241a8` 例外的說明，標題去掉「凍結集合」 | B |
| `issue.md` | ① 檔頭：刪掉 D1 的例外段。② I-120：處置方向裡的「要等 T-077」改寫成現況。③ I-118 的「比照 `finalize-stage2-evidence.sh`」補上 `c6c2481` 的出處 | B（③ 在 A） |
| `todo.md` | T-072 照 D4 處理（A）；T-077、T-073 改成「已實作／待 review」（B） | A／B |

* 每個階段結束時都跑 `python3 scripts/check-doc-refs.py`，要求問題 0。
* 本計畫書⛔ 不寫 `檔名:行號`。

## 七、決定（✅ 2026-10-30 依 v1 的 review 定案）

| | 定案 | 內容 |
|---|---|---|
| D1 判讀器的測試 | **(b)** | 抽出成獨立的 `scripts/test-judge-i074-stage2.sh`，留在常態測試；**另外**做一次真跑當額外驗收 |
| D2 L3 的 Stage 2 程式 | **(a)** | ⛔ 不動，也⛔ 不另立項目 |
| D3 wf 的操作程序 | **(a)，補界線** | 退役標記只加在已刪入口的程序上；判讀與 image 還原的程序獨立成一節、可以執行，⛔ 不讓整節被標成不可執行 |
| D4 I-119、I-121、T-072 | **(a)** | 一併處理；I-119、I-121 的 5 處行號引用在階段 A 就改成歷史引用 |
| D5 證據檢查 | **(a)，補說明** | 兩份 patch 與 `evidence_manifest.json` 的固定值檢查；文件寫明它⛔ 不驗 manifest 所列 artifact 的實際內容 |
| D6 smoke 的反事實段 | **(a)，補驗證** | 刪除；明記移除了哪一層覆蓋；實際執行保留的 smoke |

### D1 (b)：`scripts/test-judge-i074-stage2.sh`

* **隔離**：
  * 在 `mktemp -d` 裡建合成 repo，⛔ 不碰真正 repo 的工作樹、index 與 refs；
  * ⛔ 不需要 docker 與 image；
  * 結束時斷言真正 repo 的 worktree 登記數不變。
* **合成 repo 的兩個 commit**：
  * base commit：fake 的 `scripts/finalize-stage2-evidence.sh` 只實作 `--verify-promotion-staging`，記錄呼叫並依情境回傳；
    另有 evidence 目錄，manifest 的 `finalizer_provenance.base_commit` 指向這個 commit。
  * HEAD commit：改掉錨點，並把 HEAD 的驗證入口改壞，對應現有的 n9 (ii)(iii)。
* **判讀器副本**：以 sed 只改 `I074_TRUSTED_PATH`、`I074_GIT` 兩行常數，`I074_GIT` 指向轉交真 git 的包裝。
* **案例**：
  * 「二」列的 11 個執行期案例全部保留，期望值不變；
  * 靜態檢查保留四項：
    * sed 副本只差常數行（4 行）；
    * shebang；
    * PATH 恰好一處常數指派加一次 `export`；
    * 信任根常數。原本和 orchestrator 比對，orchestrator 刪除之後改成和寫死的期望值比對：`/usr/bin:/bin`、`/usr/bin/git`、`/usr/bin/python3`。
* **反向驗證**（實作時做一次，記在結果裡，⛔ 不進常態測試）：
  * 在副本裡拿掉「腳本內容 ＝ HEAD」的檢查，對應的案例要變紅；
  * 拿掉 PATH 的重設，「呼叫者 PATH 前面放了 fake 程式、照樣以 `base_commit` 那一版判讀」的成功案例與 PATH 的靜態檢查要變紅。
    「fake git／docker／python3 ⛔ 沒有被執行」在這個變異下⛔ 不會變紅：信任根的 `command -v` 檢查會先擋下（`command -v` ⛔ 不執行程式），
    所以⛔ 不能拿它當這個變異的判定。
* **額外驗收**（真跑）：
  * 階段 B 由使用者在自己的終端機執行判讀器（比照 ⑪）。
  * verdict、`base_commit`、manifest SHA 都要和 ⑪ 相同。
  * image `2a90ad1c…` 不在本機時，先用 `restore-replay-image.sh` 還原。

### D5 (a)：固定值檢查的範圍

* **驗**：`counterfactual_e1cbbbd.patch`、`tooling_e1cbbbd.patch`、`evidence/evidence_manifest.json` 三個檔的 SHA-256 等於寫死的封存值。
  寫入前要和 sr、⑩ 的紀錄核對。
* **⛔ 不驗**：
  * manifest 列出的 artifact（before、comparison、identity、patch 副本）的實際內容；
  * manifest 自己的 SHA 鏈。

  這兩項由判讀器的驗證模式負責（D1 的真跑）。

### D6 (a)：smoke 要刪的部分與移除的覆蓋

* **刪除範圍**（v1 的 review #3）：
  * 檔頭描述反事實路徑的那一段；
  * cleanup 裡專給反事實用的 worktree 清理；
  * 前置的「反事實段的來源 bundle 必須乾淨」檢查：它在 docker build 之前，只為反事實段存在；
  * 反事實段本身；
  * 段尾的來源 bundle 複查與 worktree 清理。

  `source lib/replay-args.sh` 保留與否，看一般段還用不用到它的函式。
* **移除的覆蓋**：
  * 「在真實的產品資料流上，反事實的候選非空、逐列翻轉」這一層覆蓋隨之移除，⛔ 沒有替代：
    * pytest 的 `test_replay_counterfactual.py` 用 `stub_replay`，⛔ 不經過產品資料流；
    * `test-replay-args.sh` 的 dry-run 只驗 argv。
  * 這件事寫進三處：wf 的「測試落點」、`test_replay_counterfactual.py` 的檔頭、T-077 的結果。
* **驗證**：在階段 A 的前後各跑一次保留的 smoke（`scripts/smoke-replay-offline.sh`），一般 Stage 1／2 要都通過（見「九」）。

## 八、風險與回滾

| 風險 | 對策 |
|---|---|
| 刪到保留的工具還在用的 helper | 「三」的驗收條件：每一筆命中都要歸類；跑全量測試與 smoke |
| `test-replay-args.sh` 的共用變數或函式被一起刪掉，保留的段靜默 skip | 先記下每一段 `ok` 數的基準，刪完逐段比對；`IMAGE_REQUIRED=1` |
| 判讀器的覆蓋在搬移時變少 | 新測試的案例逐項對照「二」的清單；做反向驗證 |
| 改註解時改到程式 | AST 比對；shell 檔逐行確認只改了整行註解 |
| 證據被動到 | `git diff --stat python/baselines` 必須為空；D5 的檢查 |
| smoke 留下 worktree 登記（I-118 的既有成因） | 前後記錄登記數；增加的照 I-118 記錄。清理只用 `git worktree remove` 移除存在的項目，⛔ 不用 prune；要清之前先列給使用者確認 |
| 文件的行號引用失效 | 每個階段結束都跑 `check-doc-refs.py` |

回滾：全部變更都是 git 內的刪除、新增一支測試與註解改寫，`git revert` 就能還原。repo 外的 tarball 與 identity ⛔ 不動。

## 九、測試與驗證

1. **實作之前記基準**：
   * `python/scripts/test.sh` 的 pytest 結果（目前 2409 passed、1 skipped）；
   * `test-replay-args.sh` 每一段的 `ok` 數；
   * `scripts/smoke-replay-offline.sh` 的結果；
   * worktree 登記數與可用空間。可用空間要 ≥ 1.3 GB，不足就停。
2. **階段 A 之後**：
   * `python/scripts/test.sh` 全部通過，其中包括新的判讀器測試；pytest 數量的減少要能用刪掉的測試檔解釋；
   * `scripts/smoke-replay-offline.sh` 通過：一般 Stage 1／2 的每一項都和基準相同，只少了反事實段的項目；
   * 「三」的驗收條件：每一筆命中都已歸類；
   * `git diff --name-only` 只有刪除清單、新增的測試、「三」列的保留檔與文件；
   * 產品碼 0 變更。
3. **舊版失敗的對照組**（階段 A）：
   * 在 `--no-hardlinks` 的隔離 clone（`c6c2481`）改一行產品碼的註解，再對它的 tree 跑 `make-i074-tooling-patch.sh --verify`，預期因為 ④ 回 1。
   * ⛔ 不在真正 repo 上跑；可用空間不足就停。
4. **階段 B 之後**：
   * 解凍的正向驗證：階段 B 的註解改寫本身就改了產品碼，`python/scripts/test.sh` 要照樣通過；
   * AST 比對；
   * D1 的真跑（使用者執行）。
   * 前端⛔ 不受影響，不跑。
5. **文件**：
   * `check-doc-refs.py` 問題 0；`git diff --check` 通過；新增的行裡每個禁止標記後面都帶否定詞。
   * `docs/` 裡指向被刪檔案的內容，只能出現在三種地方：帶退役標記的節、帶 `c6c2481` 的歷史引用、E4 的說明。

## 十、執行順序

1. **階段 A**：
   * 刪除清單、新增的判讀器測試、「三」的保留檔、「四」的段落、D6 的 smoke；
   * 「六」標 A 的文件；
   * 驗證照「九」的 1（基準）、2（階段 A 之後）、3（舊版失敗的對照組）。⛔ 不含解凍的正向驗證，它依賴階段 B 的註解改寫。

   完成後 → review → 使用者 commit。
2. **階段 B**：
   * 「五」的註解改寫與 T-073；
   * 「六」標 B 的文件；
   * 驗證照「九」的 4（含解凍的正向驗證與 D1 的真跑）；
   * T-077、T-073 的狀態。

   完成後 → review → 使用者 commit。
3. **收斂**（另一次）：review 確認之後，從清單移除 T-077、T-073 與 D4 的三筆，並刪除本計畫書。

## 十一、完成後的歸檔位置

* 退役之後的現況寫在兩份文件：
  * wf：「I-074 Stage 2 的 tooling patch」（改寫成退役後的現況）、判讀與 image 還原的程序、「I-074 Stage 2 的測試落點」；
  * sr：「兩道 guard 與兩份 patch 的封存」（封存規則的現況）、「晉升與 B／C 判讀器」（依賴與測試）、「容量驗收」（I-119 的注意事項）。
* 收斂時，`todo.md`／`issue.md` 檔頭的記事比照既有的寫法。

## 十二、修訂紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-10-30 | 初版 |
| v2 | 2026-10-30 | 依 v1 的 review 修正。#1：I-119、I-121 的 5 處 `檔名:行號` 在階段 A 就改成 `c6c2481` 的歷史引用（「六」）。#2：刪除的驗收改成「HEAD 上⛔ 沒有未處理的執行依賴」，並列出 E1～E5 五類例外（「三」）。#3：D6 寫明 smoke 的完整刪除範圍、移除的覆蓋與 `test_replay_counterfactual.py` 的檔頭；驗證加上實跑保留的 smoke（「七」「九」）。D1～D6 定案：D1 改採 (b)，新增 `scripts/test-judge-i074-stage2.sh`，真跑改為額外驗收；D3 補界線；D5 補「⛔ 不驗 artifact 內容」 |
| v3 | 2026-10-30 | 依 v2 的 review 修正兩處驗收描述。#1：PATH 的反向驗證改以「惡意 PATH 下照樣判讀成功」的案例與 PATH 的靜態檢查變紅為判定；「fake ⛔ 沒有被執行」在這個變異下仍是綠的（信任根檢查會先擋下）（「七」D1）。#2：階段 A 的驗收只含「九」的基準、階段 A 之後、舊版失敗的對照組；解凍的正向驗證移到階段 B（「九」「十」）。實作方向不變 |
