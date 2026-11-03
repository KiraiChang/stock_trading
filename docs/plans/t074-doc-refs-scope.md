# `check-doc-refs.py` 擴大到 `docs/` 的子目錄 計畫 v1.1（2026-11-03，✅ D1～D3 使用者確認照建議；已實作、⚠️ 待 review）

需求（`todo.md` T-074，使用者 2026-11-03 確認）：`scripts/check-doc-refs.py` 也要掃 `docs/plans/`；`scripts/test-doc-refs.sh` 補對應的 fixture；
擴充之後再決定要不要放寬 `CLAUDE.md` 的「計畫書⛔ 不寫 `檔名:行號`」。原本的前提「I-074 ⑩ 之前⛔ 不改 `scripts/`」已解除（I-074 於 2026-10-29 收斂）。

## 〇、唯讀盤點（2026-11-03）

| 項目 | 現況 |
|---|---|
| 掃哪些文件 | `check()` 只走 `docs/` 這一層的 `*.md`（非遞迴）→ **`docs/plans/` 與 `docs/architecture/` 都⛔ 沒有掃**。後者是 `CLAUDE.md` 文件索引列的「四條 pipeline 的細節」，屬於現況規格，T-074 原本沒提到 |
| 子目錄現有的引用 | `docs/plans/` 兩份、`docs/architecture/` 六份，`檔名:行號` 形式的引用都是 **0 個** → 擴充之後⛔ 不會冒出既有問題，repo 的引用總數仍是 127 |
| 子目錄 | `docs/` 底下只有 `plans/`、`architecture/` 兩個子目錄；⛔ 沒有 symlink；三處⛔ 沒有同名的 `.md` |
| 基準線的 key | `<文件檔名>\|<來源檔>:<行>`（`scripts/doc-refs-legacy.txt`），回報位置寫成 `docs/<文件檔名>:<行>`——只用檔名，子目錄的同名檔會和頂層共用額度、回報的位置也會錯 |
| 走訪的既有規則 | 找來源檔的 walker 在遞迴之前剪掉 `.git` 等目錄、一律跳過 symlink、只容忍「走訪中消失」，`PermissionError` 往上拋（漏掉候選＝fail-open）。腳本註解⛔ 不准用 `ROOT.glob("**/…")`（會先走進 `.git/worktrees`） |
| 誰會跑它 | `python/scripts/test.sh` 先跑 `scripts/test-doc-refs.sh`、再跑 `check-doc-refs.py`；Python 3.9 |

## 一、目標與不做的範圍

* **G1** `docs/` 底下所有子目錄的 `*.md` 都會被掃（D1），之後新增的子目錄⛔ 不會再靜默漏掉。
* **G2** 子目錄的文件用「`docs/` 底下的相對路徑」當基準線的 key 與回報位置；頂層文件的相對路徑就是檔名，**既有的基準線⛔ 不需要改**。
* **G3** 計畫書的 `檔名:行號` 依 D3 處理。
* **G4** 測試：每個新的 fixture 都要先證明「舊版 checker 抓不到」（對 HEAD 的版本跑會失敗），再證明新版抓得到。

不做：⛔ 不改引用的判定規則（四種緊鄰寫法、函式範圍、±15 行、line-only 基準線）；⛔ 不改其他文件的內容；⛔ 不碰 live。

## 二、做法

### A. 文件清單（D1）

從 `docs/` 往下走，收集所有 `*.md`，排序後逐份檢查。走訪照既有 walker 的規則：一律跳過 symlink（⛔ 不跟隨，可能指到 repo 外或形成 cycle）、
只容忍走訪中消失、`PermissionError` 等其他錯誤**往上拋**（讀不到目錄＝可能漏掉文件，⛔ 不能當成「沒有引用」）。
（v1.1）剪枝名單與既有 walker 共用（`_PRUNE_DIRS`）——`docs/` 底下一樣可能出現 `.git/worktrees`；起點 `docs/` 本身是 symlink、不存在或不是目錄都 rc＝2。v1 寫的「只從 `docs/` 開始走，所以⛔ 不會碰到 `.git/worktrees`」是錯的（review 第 1 輪）。

### B. key 與回報位置

key 改成 `<docs/ 底下的相對路徑>|<來源檔>:<行>`、回報位置 `docs/<相對路徑>:<行>`。頂層文件的相對路徑等於檔名 → 既有基準線的 key 不變；
子目錄是 `plans/…`、`architecture/…`，⛔ 不會和頂層的同名檔共用額度。

### C. 計畫書（D3，建議 (c)）

`docs/plans/` 裡出現任何 `檔名:行號` 形式的引用都報錯（⛔ 不論行號對不對），訊息寫明規則與替代寫法（不帶行號的「`檔名` 的 `函式()`」）。

### D. 文件

* 腳本的 docstring、`development-workflow.md` 講 doc-refs 的那一節：掃描範圍改成 `docs/` 全部，寫明計畫書的規則由工具擋。
* `CLAUDE.md`「協作流程」的「（`scripts/check-doc-refs.py` 不掃這個目錄）」依 D3 改寫（⚠️ 動 `CLAUDE.md` 要使用者同意，這裡是 D3 的一部分）。
* `todo.md` T-074 改成「已實作／待 review」。

## 三、決定（✅ 2026-11-03 使用者確認：D1～D3 照建議）

| # | 問題 | 選項 | 建議 |
|---|---|---|---|
| D1 | 掃描範圍 | (a) 只加 `docs/plans/`／(b) `docs/` 底下全部（含 `architecture/` 與之後新增的子目錄） | **(b)**：`architecture/` 是現況規格，同樣的漏洞；明列目錄的話，下一個新目錄又會靜默漏掉。現有子目錄 0 個引用，⛔ 沒有額外風險 |
| D2 | 走訪方式 | (a) 照既有 walker 的規則自己走（跳過 symlink、`PermissionError` 往上拋）／(b) `pathlib` 的遞迴 glob | **(a)**：與找來源檔的 walker 同一套威脅模型；Python 3.9 的遞迴 glob 對 symlink 與權限錯誤的行為不好控制 |
| D3 | 計畫書的 `檔名:行號` | (a) 規則不變，工具照一般規則驗（寫了也會驗）／(b) 放寬：計畫書可以寫，照一般規則驗／(c) 規則不變，**工具強制**：`docs/plans/` 出現就報錯 | **(c)**：計畫書寫的是「要改的」程式，實作期間行號一直動——寫了會在實作中途變紅、或逼人回頭追計畫書的行號；規則的理由仍在。(c) 把每次 review 前手動掃的「計畫書沒有 `檔名:行號`」變成自動的。`CLAUDE.md` 的括號改成「`scripts/check-doc-refs.py` 會擋」 |

## 四、受影響的檔案

* `scripts/check-doc-refs.py`（文件清單、key、計畫書的規則、docstring）、`scripts/test-doc-refs.sh`（新的 fixture）。
* `docs/development-workflow.md`、`CLAUDE.md`（D3）、`docs/todo.md`（T-074 狀態）。
* ⛔ 沒有資料流、API、DB 或 runtime 的變化；`python/scripts/test.sh` 照常呼叫兩支腳本。

## 五、風險與回滾

| 風險 | 對策／回滾 |
|---|---|
| 新的走訪漏掉文件（fail-open） | fixture 驗子目錄真的被掃（舊版抓不到的漂移，新版要抓得到）；讀不到目錄要非零結束 |
| key 改變讓既有基準線失效 | 頂層的相對路徑＝檔名；實跑 repo 的結果要與改之前相同（127 個、問題 0、⛔ 沒有殘留份額） |
| 計畫書強制規則誤擋（D3 (c)） | 只比對既有的 `檔名:行號` pattern；歷史 commit 的寫法（「`<commit>` 的 `檔名` 第 N 行」）與不帶行號的寫法都⛔ 不會命中 |
| 回滾 | revert 兩支腳本與文件即可；⛔ 沒有其他狀態 |

## 六、測試與驗證

`scripts/test-doc-refs.sh` 新增（每一個都要**先對 HEAD 的舊版 checker 跑、確認會失敗**，證明測試抓得到）：

1. `docs/architecture/` 裡漂移的引用 → rc 1，回報位置是 `docs/architecture/…`。
2. 同一個位置的正確引用 → rc 0，而且計入「識別符已驗」（⛔ 不是被略過才通過）。
3. 子目錄的 line-only 引用：只有頂層檔名的基準線額度⛔ 不能抵用（rc 1）；寫成 `architecture/…` 的 key 才通過。
4. 計畫書裡的 `檔名:行號`（行號是對的）→ rc 1，訊息寫明計畫書的規則（D3 (c)）。
5. `docs/` 底下指到 repo 外的目錄 symlink（裡面有漂移的引用）→ ⛔ 不跟隨（rc 0）。
6. `docs/` 的子目錄讀不到（權限 000）→ 非零結束，⛔ 不能當成通過。

另外：既有的測試全部照常通過；對 repo 實跑 `check-doc-refs.py`，結果與改之前相同（引用 127 個、問題 0）；`bash -n`；新增的禁止標記都接否定詞。

## 七、執行順序

1. 使用者確認 D1～D3。
2. 先寫測試、對舊版跑（預期新的 fixture 失敗）→ 改 checker → 全部通過 → repo 實跑。
3. 文件（含 `CLAUDE.md`）→ ⛔ 不 commit，停下等 review。
4. review 通過、使用者 commit 之後：現況已在 `development-workflow.md` 與腳本的 docstring，刪除本計畫書、收斂 T-074。

## 八、完成後的歸檔位置

* `scripts/check-doc-refs.py` 的 docstring（範圍、key、計畫書規則與理由）。
* `docs/development-workflow.md` 講 doc-refs 的那一節。
* `CLAUDE.md`「協作流程」的計畫書規則。

## 十、實作紀錄

### 2026-11-03（⚠️ 待 review）

* checker：`check()` 改走新的 `_doc_files()`（從 `docs/` 往下走，規則同 `_find_by_basename`：⛔ 不跟隨 symlink、只容忍走訪中消失、其他 `OSError` 往上拋）；key 與回報位置改用 `docs/` 底下的相對路徑；計畫書的 `檔名:行號` 在 `resolve()` 之前就報錯（`PLAN_RULE`）；`main()` 多接 `OSError` → rc＝2、訊息寫明「⛔ 不能當成沒有引用」；docstring 補範圍、key、計畫書規則與理由。
* 測試：`test-doc-refs.sh` 新增 ⑮-a～g（8 項）。**先對未修改的 checker 跑**：⑮-a、b、c（兩項）、d、f 共 6 項失敗（舊版 rc 0 或只報基準線殘留），⑮-e（目錄 symlink）、g（計畫書不帶行號）通過——舊版⛔ 不遞迴，這兩項本來就踩不到，是新 walker 的防線（與「六」寫的「每一個都要對舊版失敗」不同，照實記錄）。改完之後 53 項全部通過；⑮-f 在非 root 下實測 rc＝2。
* repo 實跑：新舊版（HEAD 的版本以 `--root` 對同一個 repo）的 `--list` 輸出**逐字相同**——引用 127 個、識別符已驗 10、僅驗檔案與行號 110、外部套件略過 7、問題 0。現有計畫書（含本檔）都⛔ 沒有 `檔名:行號`。
* 文件：`development-workflow.md` 講 doc-refs 的那一節補範圍、symlink、讀不到的處理、相對路徑 key 與計畫書規則；`CLAUDE.md` 的括號改成「會擋：計畫書裡出現就報錯」；`todo.md` T-074 改成已實作／待 review。

### 2026-11-03 review 第 1 輪（兩項 P2，✅ 使用者確認修法；⚠️ 修正待 review）

* **起點沒檢查 symlink**：`docs/` 本身是指到 repo 外的 symlink 時，外部文件被當成 `docs/…` 掃進來。修正：`docs/` 是 symlink → rc＝2、⛔ 不跟隨（⛔ 不能跳過——跳過等於什麼都沒掃卻回 0）。**延伸**（使用者確認）：`docs/` 不存在或不是目錄也 rc＝2——原本回「引用共 0 個、rc 0」，同樣是沒驗卻通過；⛔ 沒有東西依賴舊行為（fixture 一定建 `docs/`，repo 也有）。
* **沒有沿用 `_PRUNE_DIRS`**：掃進了 `docs/.git/worktrees/…`。修正：遞迴之前剪掉 `_PRUNE_DIRS`（與 `_find_by_basename` 共用），docstring 裡那句錯誤的說明改掉。
* 測試：新增 ⑮-h（`docs/` 是 symlink）、⑮-i（`docs/.git/worktrees`、`docs/node_modules`）、⑮-j（`docs/` 不存在）；**先對修正前的版本跑，3 項都失敗**（rc 1、rc 1、rc 0），修正後 56 項全部通過；repo 實跑的 `--list` 仍與改之前的舊版逐字相同（127 個、問題 0）。

## 九、修訂紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-11-03 | 初版 |
| v1.1 | 2026-11-03 | review 第 1 輪：起點 `docs/` 的檢查（symlink、不存在）與 `_PRUNE_DIRS` 剪枝 |
