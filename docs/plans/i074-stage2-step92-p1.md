# Stage 2 步驟 ⑨-2 P1 修正計畫 v1（2026-10-27，⚠️ 待確認）

⚠️ 2026-10-27 自 `docs/issue.md` I-074 搬入（[`claude-md-rules-only.md`](./claude-md-rules-only.md)「五」）；內容原樣，只改標題層級。文中的「⑨-2「二」」「⑨-1 結果一節」等都指 `docs/issue.md` I-074 的章節。

需求確認（2026-10-27）：⑨-1 結果的 commit `cbde340`（＝ `R91`，已 push 到 `origin/init`）同時含 `CLAUDE.md` 協作流程的新規則（Claude 一律不 commit），
⑨-2 的 P1（`git diff --name-only "$E91" HEAD -- . ':(exclude)docs'` 必須沒有輸出）因此⛔ 不成立。使用者選擇**修改 P1**——⛔ 不改寫已 push 的歷史、⛔ 不補還原 `CLAUDE.md` 的 commit。
⚠️ 本計畫確認之前⛔ 不改 ⑨-2 的區塊、⛔ 不執行 ⑨-2。

## 〇、唯讀盤點（2026-10-27，HEAD `cbde340`）

| 項目 | 現況 |
|---|---|
| P1 現況（✅ 實測） | `git diff --name-only d3df284 cbde340 -- . ':(exclude)docs'` → `CLAUDE.md`（rc 0）→ ⛔ 沒有「P1 ⑨-1 之後只動 docs/ ok」 |
| P1 的用意 | ⑨-1 量到的程式 ＝ ⑩ 要跑的程式（⑨-2「二」P1 一列）；⑨-2 計畫第一輪 review 的「低」：只檢查三個範圍時，`backend/`、`frontend/`、根目錄的設定改了也會印 ok → 改成「`docs/` 以外一律⛔ 不允許」 |
| 誰讀 `CLAUDE.md` | 程式⛔ 沒有讀它的內容：13 個程式檔只在註解裡提到它；唯一的實際引用是 `scripts/test-replay-args.sh` 的一個測試案例（建一個指向它的 symlink，只要求檔案存在）；`--formal` 的守門與 harness 的快照只涵蓋 `scripts/`、`python/`、`.gitattributes` |
| 磁碟 | `code_worktree` 是 HEAD 的整個 worktree（含 `CLAUDE.md`）：12,236 → 12,589 bytes（＋353），allocated 最多多一個 4 KiB 區塊 |
| 候選的寫法（✅ 實測，真實的歷史） | 多排除根目錄的 `CLAUDE.md`：`d3df284..cbde340` → 沒有輸出；`496a5f7..d3df284`（有程式改動）→ 照樣列出 6 個路徑（`python/`、`scripts/`） |
| 其他副本 | `development-workflow.md` ⛔ 沒有 ⑨-2 的 P1；⑨-1「執行紀錄格式 v2」裡的 P1 草稿（`i074_stage2_step92_precheck_p1_draft`）是歷史紀錄、⛔ 不改 |

## 一、目標、範圍與⛔ 不做

| 項目 | 內容 |
|---|---|
| 目標 | P1 允許 `E91` 之後的變更只落在**文件**：`docs/` 與根目錄的 `CLAUDE.md`；其他任何路徑（含 `backend/`、`frontend/`、根目錄的其他檔案、子目錄裡同名的 `CLAUDE.md`）仍⛔ 不允許；`git` 失敗⛔ 不能當作成立（fail-closed ⛔ 不變） |
| 範圍 | ⑨-2「二」P1 的表格列與區塊（`i074_stage2_step92_precheck_002` → `_003`：只換 P1 的 3 行，加一行註記）；⑨-2「六」與 D7 的「只 commit 文件」加上「文件」的定義；I-119「處置方向」的一句；本節 |
| ⛔ 不做 | ⛔ 不改寫歷史、⛔ 不補還原 commit；⛔ 不改 P1 的祖先與 clean 檢查、P2～P10 與 ⑨-2 的其他指令；⛔ 不改程式；⛔ 不改歷史章節（執行紀錄格式 v2 的草稿與測試紀錄） |

## 二、設計

P1 的 3 行換成（`E91` 之後的路徑排除 `docs/` 與根目錄的 `CLAUDE.md`；印出與判定的文字從「docs/」改成「文件」）：

```text id="i074_stage2_step92_precheck_p1_003_draft"
DD="$(git diff --name-only "$E91" HEAD -- . ':(exclude)docs' ':(exclude)CLAUDE.md')"; RDD=$?
printf 'P1 ⑨-1 之後文件（docs/ 與根目錄的 CLAUDE.md）以外的變更：%s（git diff 的 rc %s）\n' "${DD:-無}" "$RDD"
[ "$RDD" = 0 ] && [ -z "$DD" ] && echo "P1 ⑨-1 之後只動文件 ok"                                              # E91 錯 ⛔ 不能當作只動文件
```

* 區塊一開始就 `cd` 到 repo 的根目錄，pathspec 以根目錄為準；`CLAUDE.md` 沒有萬用字元，只排除根目錄的那一個檔案（子目錄的同名檔案照樣列出——「三」實測）。
* 開跑的條件與 P1 的表格列：「P1 ⑨-1 之後只動 docs/ ok」改成「P1 ⑨-1 之後只動文件 ok」。
* ⑨-2「六」與 D7 的「只 commit 文件」：文件 ＝ `docs/` 與根目錄的 `CLAUDE.md`（加註記，⛔ 不改寫原裁決）。

## 三、測試與驗證（實作之後；⛔ 不在真正的 repo 建 commit）

* 從文件抽出新的 P1 3 行，在 zsh 與 bash 各執行（非互動、唯讀）：
  * 真正的 repo（HEAD `cbde340`）、`E91` ＝ `d3df284` → 有「P1 ⑨-1 之後只動文件 ok」；**對照組**：同一個狀態跑現行的 3 行 → 列出 `CLAUDE.md`、⛔ 沒有 ok（重現）。
  * `E91` 打錯、空白 → `git diff` 的 rc ≠ 0、⛔ 沒有 ok。
* 隔離 clone（`git clone --no-hardlinks` 到 scratchpad；`.git` 約 217 MB、可用 5.5 GB）裡各建一個 commit，`E91` ＝ `d3df284`：只改 `docs/` → ok；只改 `CLAUDE.md` → ok；新增 `python/CLAUDE.md` → 列出、⛔ 沒有 ok；改根目錄的其他檔案（例如 `docker-compose.yml`）→ 列出、⛔ 沒有 ok；改 `scripts/` → 列出、⛔ 沒有 ok。跑完整個刪除 clone。
* 區塊：佔位換成 `R91`、`E91` 之後 `bash -n`／`zsh -n` 通過；P1 以外的行與 `_002` 逐位元相同。
* 文件：doc-refs、表格欄數、`git diff --check`、新增的行掃禁止符號後面的否定詞。

## 四、風險與回滾

| 風險 | 處置 |
|---|---|
| 放寬之後漏掉會影響 ⑩ 的變更 | 只多排除一個根目錄的檔名、⛔ 不用萬用字元；`CLAUDE.md` 的內容⛔ 沒有程式讀取（「〇」）。之後若有程式開始讀它，本節的前提就不成立 → 回到「只允許 `docs/`」 |
| 之後又改 `CLAUDE.md` | 允許（它是文件）；只要 `scripts/`、`python/`、`.gitattributes` 與其他路徑⛔ 沒有變，⑨-1 量到的程式仍 ＝ ⑩ 要跑的程式 |
| 磁碟的餘裕 | `CLAUDE.md` 的增量與 `docs/` 一樣是 KB 級，照 ⑨-2 的判定實測 |
| 回滾 | revert 本次的文件 commit（P1 回到只允許 `docs/`）；⛔ 不影響任何程式與 artifact |

## 五、決定

| # | 決定 | 建議 |
|---|---|---|
| D1 | 允許的範圍 | **建議 (a)**：根目錄的 `CLAUDE.md` 任何內容都允許（它是文件，之後再改協作守則也⛔ 不會再擋下 ⑨-2）。(b) 只允許已 review 的那一版：另加 `git diff --quiet cbde340 HEAD -- CLAUDE.md`——更嚴，但 `CLAUDE.md` 之後再改就又擋下 |

⚠️ 2026-10-27 註記（[`claude-md-rules-only.md`](./claude-md-rules-only.md)「五」）：該計畫會再改一次 `CLAUDE.md`（只留開發規則），實作之後 (b) 就⛔ 不成立——D1 實際上只剩 (a) 可行；裁決仍由使用者做。

## 六、歸檔與執行步驟

1. 本計畫確認（含 D1）。
2. 實作（只動 `docs/issue.md`：「一」的範圍）→「三」的驗證 → ⛔ 不 commit、停下等 review → 使用者 commit → ⑨-2（P1 用新的 3 行）。
3. 收斂：⑨-2 的計畫收斂時，P1 的現況規格（「文件」的定義）隨 ⑨-2 的程序搬進 `development-workflow.md` 的 acceptance／sizing 一節；本節隨 ⑨-2 的計畫一起移除。
