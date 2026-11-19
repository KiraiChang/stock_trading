# 基底 image 固定到 digest（T-083）：tag 會被重新發布（安全更新），只寫 tag 的話同一份 Dockerfile 隔一陣子
# build 出來的系統與 pip／setuptools／wheel 就不一樣。這個 digest 是 2026-11-16 拉到的
# python:3.11.18-slim-trixie（Python 3.11.18、Debian 13.7、pip 24.0、setuptools 79.0.1、wheel 0.46.3），
# 它的 4 層 layer 跟 dev 當時的 python image 的基底相同（live 的比對：git show 85849b9:docs/plans/t083-python-pin-deps.md）。升級程序見 docs/development-workflow.md
# 「python 的套件版本與升級」。
FROM python:3.11.18-slim-trixie@sha256:07d4824036af339fa1b6ef769cc99d9c10a141769538b52b3b597c84f7a7b535

WORKDIR /app

# lightgbm 的原生庫需要 OpenMP runtime（libgomp.so.1），python 的 slim image 預設沒有，
# 缺這個 import lightgbm 會 OSError: libgomp.so.1: cannot open shared object file。
# ⚠️ libgomp1 刻意不釘版本：Debian 的套件庫會移除舊版，釘了會讓之後的 build 直接失敗；
#    版本在驗收時記錄（2026-11-16 live：14.2.0-19），跟 live 不同要由人判斷。
RUN apt-get update \
    && apt-get install -y --no-install-recommends libgomp1 \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
# requirements.txt 是完整的精確釘版（T-083）；pip check 確認依賴彼此不衝突，衝突就讓 build 失敗。
RUN pip install --no-cache-dir -r requirements.txt \
    && pip check

COPY . .

CMD ["python", "worker.py"]
