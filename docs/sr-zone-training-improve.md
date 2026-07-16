```
                     歷史 OHLCV
                          │
          ┌───────────────┴───────────────┐
          │                               │
          ▼                               ▼
   Transformer                    Feature Engineering
(學習時序模式)                 (技術指標、支撐壓力、籌碼...)
          │                               │
          ▼                               ▼
   Sequence Embedding            LightGBM / XGBoost
          │                               │
          └───────────────┬───────────────┘
                          ▼
                  Ensemble / Meta Model
                          ▼
             勝率、預期報酬、EV、RR、Confidence
                          ▼
                  Go Decision Engine
                          ▼
                   Telegram / Web API
```