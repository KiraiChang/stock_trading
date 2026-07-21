"""Small shared numeric helpers for SR Zone scoring modules."""
from __future__ import annotations


def distance_pct_to_zone_bounds(price_low: float, price_high: float, current_price: float) -> float:
    if price_low <= current_price <= price_high:
        return 0.0
    if current_price < price_low:
        return (price_low - current_price) / current_price
    return (current_price - price_high) / current_price
