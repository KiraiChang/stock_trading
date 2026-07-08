# ISSUE：遇到的問題與已知限制

記錄實際發生過的 bug、矛盾結果、文件/程式碼不一致，以及設計上刻意接受的
限制。跟「想做的優化」無關的項目放這裡；未來想做的功能擴充記錄在
[todo.md](./todo.md)。

## 使用說明

- **狀態**：`待修復` / `修復中` / `已修復` / `已知限制（不計畫修復）`
- **嚴重度**：`高`（結果矛盾/資料錯誤）/ `中`（誤導但不影響核心功能）/
  `低`（文件或註解落後，不影響 runtime）
- 新增項目時往下加一筆，編號遞增（`I-0xx`）。修復後若仍需短期追蹤，先把「狀態」改成
  `已修復` 並補上「修復方式」；若修復紀錄已移到對應主題文件或 review 文件，
  則可從本清單移除。

---

### I-001：Docker Compose SR Scoring 模型路徑仍指向 v2

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 中 |
| 分類 | Docker / Python / SR Zone |
| 建立日期 | 2026-07-08 |

`python/backtest/modular/sr_scoring/model.py` 的 `MODEL_VERSION` 目前是 `v3`，且
`python/config.yaml`、`docs/development-guide.md`、`docs/api-reference.md` 都已指向
`models/sr_scoring_v3.joblib`。但 `docker-compose.yml` 的 `python-worker` 與
`python-server` 仍設定：

```yaml
SR_SCORING_MODEL_PATH: /app/models/sr_scoring_v2.joblib
```

這會讓 Docker 環境載入與目前 feature schema 不相容的舊模型路徑，可能在
`/sr-zones` 預測時因特徵數不一致而失敗。建議後續將 compose 環境變數改為
`/app/models/sr_scoring_v3.joblib`，並確認部署目錄已有 v3 模型或重新訓練一次。
