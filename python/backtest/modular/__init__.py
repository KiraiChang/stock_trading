"""
模組化、可回測的交易策略框架。

三個可獨立替換的元件（Strategy Pattern），彼此透過抽象介面溝通：

    SupportResistanceStrategy → 算出 support/resistance levels
    EntryStrategy             → 依 levels + OHLCV 判斷進場訊號
    StopLossStrategy          → 依進場資訊算出（並隨每根K棒更新）停損價

三者由 strategy.TradingStrategy 組合成一個完整策略，交給 backtester.BacktestEngine
執行：輸入 OHLCV DataFrame，輸出 entry/exit/pnl（types.Trade 列表）與績效指標。

與既有 backtrader 版本（backtest/strategy/breakout_v1.py）的關係：
    既有版本是寫死在單一 backtrader Strategy 子類別裡的邏輯，難以單獨替換
    S/R、進場、停損邏輯，也不易在沒有 backtrader 事件迴圈的情況下單元測試。
    這裡是獨立於 backtrader 的純 pandas/numpy 實作，可直接單元測試每個元件；
    service.py 提供與 backtest/engine.py 相同輸出格式的入口，供既有的
    worker.py / http_server.py 直接調用，不需改動 Go 端或資料庫結構。
"""
