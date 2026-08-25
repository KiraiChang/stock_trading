-- job_runs 的「每個 job 取最新一筆」查詢索引。
-- 現況規格見 docs/api-reference.md 的 GET /scheduler/status。
--
-- 要解的問題：/scheduler/status 原本用 `GetRecent(50)`（ORDER BY started_at DESC
-- LIMIT 50）取最近 50 筆再在記憶體分組。但 intraday 每 5 分鐘一筆、09:00-13:30 一天
-- 就 55 筆，比視窗還多——每天過了 13:30，06:30 的 corporate_action_sync 與 08:50 的
-- pre_market 全被擠出視窗，狀態頁把它們報成「從未跑過、該跑卻沒跑」。
--
-- 改法是讓 SQL 直接回「每個 job 最新一筆」（PARTITION BY job_name ORDER BY
-- started_at DESC），回傳筆數恆等於「有紀錄的 job 數」而不隨資料量成長
-- （固定 11 列是 handler 補出來的，不是這句 SQL）。本索引就是支撐那句查詢的。
--
-- **既有的 idx_job_runs_job_name 撐不住**：它只有 job_name 一欄，排序仍要另外做。
-- 複合索引把 (job_name, started_at DESC) 一起吃下，涵蓋 window function 的 PARTITION BY
-- 與 ORDER BY 的前綴。**不涵蓋 id DESC 那一段** tie-break：查詢的排序是
-- (started_at DESC, id DESC)，同秒的那幾筆仍要另外決勝。那是刻意的取捨——
-- 同秒撞在一起的筆數極少，為它把 id 也加進索引不划算。
--
-- **和 073 的取捨相反，那是刻意的**：073 的 (created_at DESC, id DESC) 把 id 帶上了，
-- 因為那條查詢要對整個窗口排序再 LIMIT 截斷（可達上萬列），完整的索引排序省得到；
-- 這條每個 job_name 分組只吐一列，排序成本本來就微不足道，帶 id 省不到什麼。
--
-- **這條索引幫不到 DeleteBefore**：那句是 `WHERE started_at < ?`，沒有 job_name 條件，
-- 而本索引的 leading column 正是 job_name，範圍掃描用不上它。保留期拉長後若 DeleteBefore
-- 變慢，要加的是 started_at 單獨的索引，不是指望這一條。
--
-- 不刪 idx_job_runs_job_name：本複合索引的 leading column 相同、理論上可取代它，
-- 但刪索引是不可逆的行為變更，與本筆「只加不減」的範圍無關。

-- +goose Up
CREATE INDEX IF NOT EXISTS idx_job_runs_job_name_started_at
    ON job_runs (job_name, started_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_job_runs_job_name_started_at;
