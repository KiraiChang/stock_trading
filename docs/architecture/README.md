# Architecture Pipeline Index

本目錄補充 [../architecture.md](../architecture.md) 的 pipeline 細節。`architecture.md`
保留系統總覽與既有入口；本目錄負責把升級後的系統邊界拆成四條 pipeline。

## Pipeline 總覽

```
Data Pipeline
    ↓ 已落地、可追溯的資料
Analysis Pipeline
    ↓ 可解釋的分析快照與 evidence
AI Pipeline
    ↓ 模型機率、模型狀態、訓練 metrics
Decision Pipeline
    ↓ 交易行動、風控、部位調整建議
```

| Pipeline | 文件 | 核心職責 |
|----------|------|----------|
| Data Pipeline | [data-pipeline.md](./data-pipeline.md) | 資料取得、清洗、時間對齊、持久化 |
| Analysis Pipeline | [analysis-pipeline.md](./analysis-pipeline.md) | 指標、籌碼分數、SR Zone、分析快照與 evidence |
| AI Pipeline | [ai-pipeline.md](./ai-pipeline.md) | 模型訓練、推論、模型狀態、metrics |
| Decision Pipeline | [decision-pipeline.md](./decision-pipeline.md) | 交易決策、position sizing、停損停利、風控輸出 |

## 升級計畫

| 文件 | 說明 |
|------|------|
| [sr-zone-pipeline-upgrade-plan.md](./sr-zone-pipeline-upgrade-plan.md) | `docs/sr-zone-improve.md` 對齊四條 pipeline 後的 P0/P1/P2 實作順序 |

## 共同規則

- 上游 pipeline 只透過明確資料契約提供輸出，下游不得回頭偷抓上游內部狀態。
- Data Pipeline 不做交易判斷，也不訓練模型。
- Analysis Pipeline 可以產生分析快照與 evidence，但不負責模型治理或最終交易行動。
- AI Pipeline 只輸出模型 artifact、模型狀態、訓練 metrics 與機率，不直接輸出買賣動作。
- Decision Pipeline 只消費 Data / Analysis / AI 已產出的資料，不重新抓資料、不重新訓練模型。
- 跨 pipeline 的資料要能追溯來源、時間、模型版本或設定 hash。

## 目前階段

本次拆分是文件型架構升級，不改程式碼、不改 API、不改 DB schema。後續若要把程式碼也按
pipeline 邊界重構，先以 [../todo.md](../todo.md) 的 T-034 追蹤。
