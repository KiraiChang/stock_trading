#!/usr/bin/env bash
# 驗證 live 的股價還原（T-042 Phase 1）是否正確生效。**唯讀，不寫任何一張表。**
#
# 用法：
#   scripts/verify-adjustment.sh                 # 全部檢查
#   SYMBOL=0050 scripts/verify-adjustment.sh     # 只看單一標的的明細
#
# 可覆寫的環境變數：
#   PG_CONTAINER   live postgres 的 container 名（預設 postgres-postgres-1）
#   APP_CONTAINER  用來讀 DSN 的 container 名（預設 stock_trading-python-server-1）
#   DB_NAME / DB_USER  直接指定，略過從 container 讀 DSN
#   SYMBOL         指定時額外印出該標的的還原明細
#
# 設計重點：
#   - **唯讀**：只有 SELECT。CLAUDE.md 規定驗收不得動 live 資料；重算是 backend 排程的事
#     （每天 06:30 的 corporate_action_sync），這支只負責檢查結果對不對。
#   - **獨立重新推導，而不是重跑同一份程式碼**：檢查 3 用 SQL 從 corporate_actions
#     重算一次期望係數，再與 candles.adj_factor 逐列比對。這比「再跑一次 job 看結果一不一樣」
#     強得多——後者只要邏輯本身是錯的就會一致地錯下去，而這裡是用另一個實作（SQL）
#     去對照 Go 算出來的值。
#   - DSN 不寫進 repo，執行時從 live container 讀（密碼不進版控）。
#   - 任一檢查失敗就以非零結束，可以直接當閘門用。
set -euo pipefail

PG_CONTAINER="${PG_CONTAINER:-postgres-postgres-1}"
APP_CONTAINER="${APP_CONTAINER:-stock_trading-python-server-1}"
SYMBOL="${SYMBOL:-}"

if ! docker inspect "$PG_CONTAINER" >/dev/null 2>&1; then
  echo "ERROR: 找不到 postgres container：$PG_CONTAINER（用 PG_CONTAINER= 指定）" >&2
  exit 1
fi

if [ -z "${DB_USER:-}" ] || [ -z "${DB_NAME:-}" ]; then
  dsn="$(docker exec "$APP_CONTAINER" sh -c 'printf %s "$DATABASE_DSN"' 2>/dev/null || true)"
  if [ -z "$dsn" ]; then
    echo "ERROR: 從 $APP_CONTAINER 讀不到 DATABASE_DSN，請改用 DB_USER= DB_NAME= 指定" >&2
    exit 1
  fi
  DB_USER="${DB_USER:-$(printf %s "$dsn" | sed -E 's#.*//([^:]+):.*#\1#')}"
  DB_NAME="${DB_NAME:-$(printf %s "$dsn" | sed -E 's#.*/([^/?]+)(\?.*)?$#\1#')}"
fi

psql_q() {
  docker exec -i "$PG_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -tAF'|' -v ON_ERROR_STOP=1 -c "$1"
}

fail=0
ok()   { printf '  \033[32m✓\033[0m %s\n' "$1"; }
bad()  { printf '  \033[31m✗\033[0m %s\n' "$1"; fail=1; }
note() { printf '    %s\n' "$1"; }

echo "==> 目標：$PG_CONTAINER / db=$DB_NAME user=$DB_USER"

# ── 1. schema 是否已部署 ───────────────────────────────────────────────
echo "[1/6] migration 061／062 是否已套用"
version="$(psql_q "SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version WHERE is_applied")"
has_table="$(psql_q "SELECT COUNT(*) FROM information_schema.tables WHERE table_name='corporate_actions'")"
has_column="$(psql_q "SELECT COUNT(*) FROM information_schema.columns WHERE table_name='candles' AND column_name='adj_factor'")"
has_vol="$(psql_q "SELECT COUNT(*) FROM information_schema.columns WHERE table_name='candles' AND column_name='vol_factor'")"
note "goose 版本 = $version"
if [ "$has_table" = "1" ] && [ "$has_column" = "1" ] && [ "$has_vol" = "1" ]; then
  ok "corporate_actions、candles.adj_factor 與 candles.vol_factor 都存在"
else
  bad "schema 尚未部署（corporate_actions=$has_table, adj_factor=$has_column, vol_factor=$has_vol）"
  note "061 會在 backend 啟動時由 RunMigrations 自動套用，重新部署後再跑這支。"
  exit 1
fi

# ── 2. 事件表有沒有資料 ───────────────────────────────────────────────
echo "[2/6] corporate_actions 內容"
events="$(psql_q "SELECT COUNT(*) FROM corporate_actions")"
if [ "$events" -gt 0 ]; then
  ok "共 $events 筆事件"
  psql_q "SELECT symbol || ' ' || event_date || ' ' || action_type
          || ' ' || before_price || '→' || after_price
          || ' 係數=' || ROUND(factor, 4)
          FROM corporate_actions
          WHERE symbol IN (SELECT DISTINCT symbol FROM candles)
          ORDER BY event_date" | while read -r line; do note "$line"; done
else
  bad "事件表是空的——corporate_action_sync 還沒跑過？"
  note "排程在每天 06:30（週一到五）。"
fi

# ── 3. 係數是否等於由事件表獨立重算的期望值 ──────────────────────────
#
# 這是本腳本的核心。用 SQL 重新推導一次：
#
#   expected(bar) = Π factor(e)  for e where e.event_date > taipei_date(bar.ts)
#
# 連乘用 exp(sum(ln))——postgres 沒有 product 聚合，而 factor > 0 由 CHECK 保證。
# as-of 邊界用 `>` 而不是 `>=`：event_date 是新價的第一個交易日，當日不調整。
echo "[3/6] adj_factor 是否等於獨立重算的期望值"
mismatch="$(psql_q "
  WITH expected AS (
    SELECT c.id, c.symbol, c.ts, c.adj_factor,
           COALESCE(EXP(SUM(LN(a.factor))), 1) AS want
    FROM candles c
    LEFT JOIN corporate_actions a
           ON a.symbol = c.symbol
          AND a.event_date > (c.ts AT TIME ZONE 'Asia/Taipei')::date
    WHERE c.symbol IN (SELECT DISTINCT symbol FROM corporate_actions)
    GROUP BY c.id, c.symbol, c.ts, c.adj_factor
  )
  SELECT COUNT(*) FROM expected WHERE ABS(adj_factor - want) > 1e-9
")"
if [ "$mismatch" = "0" ]; then
  ok "所有受影響 K 棒的係數都與獨立重算一致"
else
  bad "有 $mismatch 根 K 棒的係數與期望值不符"
  note "代表重算沒跑、只跑了一半，或 as-of 邊界算錯。前 5 筆："
  psql_q "
    WITH expected AS (
      SELECT c.symbol, c.ts, c.adj_factor,
             COALESCE(EXP(SUM(LN(a.factor))), 1) AS want
      FROM candles c
      LEFT JOIN corporate_actions a
             ON a.symbol = c.symbol
            AND a.event_date > (c.ts AT TIME ZONE 'Asia/Taipei')::date
      WHERE c.symbol IN (SELECT DISTINCT symbol FROM corporate_actions)
      GROUP BY c.symbol, c.ts, c.adj_factor
    )
    SELECT symbol || ' ' || (ts AT TIME ZONE 'Asia/Taipei')::date
        || ' 實際=' || ROUND(adj_factor, 6) || ' 期望=' || ROUND(want::numeric, 6)
    FROM expected WHERE ABS(adj_factor - want) > 1e-9
    ORDER BY symbol, ts LIMIT 5" | while read -r line; do note "$line"; done
fi

# ── 4. 黃金案例：0050 的 1:4 分割 ─────────────────────────────────────
#
# 這組數字是 2026-08-10 從 live 實際查到的，寫死在這裡當回歸基準：
#   2025-06-10 收 188.65（分割前最後一個交易日）→ 還原後應為 47.16
#   2025-06-18 收 47.57（新價第一個交易日）      → 不調整，係數為 1
echo "[4/6] 黃金案例：0050 2025-06 的 1:4 分割"
if [ "$(psql_q "SELECT COUNT(*) FROM candles WHERE symbol='0050' AND timeframe='1d'")" -eq 0 ]; then
  note "略過：這個 DB 沒有 0050 的資料"
else
  before="$(psql_q "
    SELECT ROUND(adj_factor, 4) || '|' || ROUND(close * adj_factor, 2)
    FROM candles WHERE symbol='0050' AND timeframe='1d'
      AND (ts AT TIME ZONE 'Asia/Taipei')::date = DATE '2025-06-10'")"
  after="$(psql_q "
    SELECT ROUND(adj_factor, 4)
    FROM candles WHERE symbol='0050' AND timeframe='1d'
      AND (ts AT TIME ZONE 'Asia/Taipei')::date = DATE '2025-06-18'")"
  want_factor="0.2500"
  got_factor="${before%%|*}"
  got_price="${before##*|}"
  if [ "$got_factor" = "$want_factor" ]; then
    ok "2025-06-10 係數 = $got_factor（還原價 $got_price，原始 188.65）"
  else
    bad "2025-06-10 係數 = ${got_factor:-<無資料>}, want $want_factor"
  fi
  if [ "$after" = "1.0000" ]; then
    ok "2025-06-18（事件當日）係數 = 1，未被重複調整"
  else
    bad "2025-06-18 係數 = ${after:-<無資料>}, want 1.0000——as-of 邊界差了一天"
  fi
fi

# ── 5. 恆等式：只對「純股數事件」成立 ────────────────────────────────
#
# adj_price * adj_volume == price * volume。價乘量除若寫反，這條立刻不成立。
#
# **但它只在 adj_factor = vol_factor 時成立**（分割、反分割、面額變更這類純股數事件）。
# 用容差而不是精確相等挑選這些列：純配股事件的兩個係數數學上相同，但一個算的是
# (prev-0)/ratio/prev、另一個算 1/ratio，浮點可能差 1 ULP——精確相等會讓那些列
# 被靜靜排除在檢查之外，看起來「通過」其實是沒驗。
# 現金股利讓價格下修而股數不變（vol_factor = 1），乘積本來就該變小——那不是 bug，
# 是錢真的離開了公司。不縮限這個條件的話，Phase 2（除權息）一上線這條檢查就會全面誤報。
echo "[5/6] 恆等式 adj_close × adj_volume == close × volume（限純股數事件）"
share_rows="$(psql_q "SELECT COUNT(*) FROM candles WHERE adj_factor <> 1 AND ABS(adj_factor - vol_factor) <= 1e-9")"
if [ "$share_rows" -eq 0 ]; then
  note "略過：目前沒有任何 K 棒受純股數事件影響"
else
  broken="$(psql_q "
    SELECT COUNT(*) FROM candles
    WHERE adj_factor <> 1 AND ABS(adj_factor - vol_factor) <= 1e-9 AND volume > 0
      AND ABS((close * adj_factor) * (volume / vol_factor) - close * volume)
          > GREATEST(ABS(close * volume) * 1e-9, 1e-6)")"
  if [ "$broken" = "0" ]; then
    ok "$share_rows 根受純股數事件影響的 K 棒全部滿足恆等式"
  else
    bad "有 $broken 根不滿足恆等式——價乘量除的方向可能寫反了"
  fi
fi

# ── 6. vol_factor 是否等於獨立重算的期望值 ───────────────────────────
#
# 與檢查 3 同樣的做法，但只累乘 volume_factor。這條抓的是「價量共用了同一個係數」——
# 若有人把現金股利也算進成交量，這裡會發現實際值偏離期望值。
echo "[6/6] vol_factor 是否等於獨立重算的期望值"
vol_mismatch="$(psql_q "
  WITH expected AS (
    SELECT c.id, c.vol_factor,
           COALESCE(EXP(SUM(LN(a.volume_factor))), 1) AS want
    FROM candles c
    LEFT JOIN corporate_actions a
           ON a.symbol = c.symbol
          AND a.event_date > (c.ts AT TIME ZONE 'Asia/Taipei')::date
    WHERE c.symbol IN (SELECT DISTINCT symbol FROM corporate_actions)
    GROUP BY c.id, c.vol_factor
  )
  SELECT COUNT(*) FROM expected WHERE ABS(vol_factor - want) > 1e-9
")"
if [ "$vol_mismatch" = "0" ]; then
  ok "所有受影響 K 棒的成交量係數都與獨立重算一致"
else
  bad "有 $vol_mismatch 根 K 棒的 vol_factor 與期望值不符"
  note "常見成因：現金股利被誤算進成交量（純現金的 volume_factor 應為 1）。"
fi

# ── 選用：單一標的明細 ────────────────────────────────────────────────
if [ -n "$SYMBOL" ]; then
  echo "==> $SYMBOL 的還原明細（事件前後各 3 根）"
  psql_q "
    WITH ev AS (SELECT event_date FROM corporate_actions WHERE symbol='$SYMBOL' ORDER BY event_date LIMIT 1)
    SELECT (c.ts AT TIME ZONE 'Asia/Taipei')::date
        || ' 原始=' || c.close
        || ' 係數=' || ROUND(c.adj_factor, 4)
        || ' 還原=' || ROUND(c.close * c.adj_factor, 2)
    FROM candles c, ev
    WHERE c.symbol='$SYMBOL' AND c.timeframe='1d'
      AND c.ts >= (ev.event_date - INTERVAL '10 days')
      AND c.ts <= (ev.event_date + INTERVAL '10 days')
    ORDER BY c.ts" | while read -r line; do note "$line"; done
fi

echo
if [ "$fail" -eq 0 ]; then
  echo "==> 全部通過"
else
  echo "==> 有檢查未通過（見上方 ✗）" >&2
fi
exit "$fail"
