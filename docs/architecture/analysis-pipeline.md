# Analysis Pipeline

Analysis Pipeline 從 Data Pipeline 讀取已落地資料，產生可解釋、可保存、可驗證的分析結果。
它可以使用規則、統計與模型輸出，但不負責模型訓練，也不直接決定最終交易行動。

## 職責

- 將 K 棒、籌碼、部位等資料轉成分析特徵與快照。
- 保存可回查的 evidence / explanation。
- 產出下游 Decision Pipeline 可消費的分析契約。
- 讓分析結果可以被驗證與重算。

## 現有模組歸位

| 類別 | 現有位置 | 說明 |
|------|----------|------|
| 技術指標 | `backend/internal/indicator` | MA、RSI、MACD、ATR、量能等指標快照 |
| 指標資料表 | `indicator_snapshots` | 最新指標快照與 Redis cache |
| 訊號前置分析 | `backend/internal/signal` | 趨勢、突破、量能等訊號基礎分析 |
| 籌碼分數 | `backend/internal/chip/score.go` / `chip_scores` | raw chip data 轉成綜合籌碼分數 |
| 個股分析 | Python `/analyze` + Go `stock_analyses` | 支撐/壓力/進場/停損/停利分析快照 |
| 個股驗證 | Go verifier | 用 candles 驗證已存分析，不重跑 Python |
| SR Zone 分析 | Python `/sr-zones` + Go `stock_sr_zone_analyses` | zone 建立、特徵、score breakdown、evidence |
| SR Zone 驗證 | `analysis.SRZoneVerifier` | 用 candles 更新 zone lifecycle |
| 籌碼面板資料 | `chip_summary` / `SRChipSummary` | 分析快照當下的籌碼 evidence |

## 輸入

- Data Pipeline 的 `candles`
- Data Pipeline 的 raw chip tables
- Data Pipeline 的 `positions` projection
- AI Pipeline 的模型機率與模型 metadata

## 輸出契約

- `indicator_snapshots`
- `chip_scores`
- `signals` 的分析基礎欄位
- `stock_analyses`
- `stock_analysis_levels`
- `stock_sr_zone_analyses`
- `stock_sr_zones`
- `evidence`
- `explanation`
- `probability_context`
- `chip_summary`

## 不負責事項

- 不抓外部原始資料。
- 不管理模型訓練任務。
- 不保存多模型 registry。
- 不產生最終 position sizing。
- 不決定使用者是否應買、賣、加碼或減碼。

## SR Zone 邊界

SR Zone 橫跨 Analysis、AI 與 Decision：

- zone 建立、features、score breakdown、evidence 屬於 Analysis Pipeline。
- `bounce_probability` / `break_probability` 的模型推論與模型狀態屬於 AI Pipeline。
- `decision_summary`、entry permission、RR context 的交易語意屬於 Decision Pipeline。

## 近期整理方向

- 文件上統一使用「analysis snapshot」描述可持久化分析結果。
- 將 feature/evidence 與 final decision 分開描述。
- 對外契約欄位新增時，同步決定前端是否消費，避免只加型別不呈現。
