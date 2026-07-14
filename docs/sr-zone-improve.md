1. 把 Action State 拆成 WAIT_CONFIRMATION → PROBE_ENTRY → SMALL_ENTRY → ACCUMULATE → BUY，並修正 PENDING_VALIDATION + AWAIT_CONFIRMATION 與「小量試單」的語意衝突。
2. 確認並實作獨立 Event Detection Layer，讓 7/14 必須明確輸出 EXTREME_VOLUME + HIGH_VOLUME_BREAKDOWN + INTRADAY_RECLAIM + REVERSAL_CANDIDATE。
3. 修正 Confluence 計算，使用 independent evidence family，避免 ×10 的 correlated evidence inflation。