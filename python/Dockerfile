FROM python:3.11-slim

WORKDIR /app

# lightgbm 的原生庫需要 OpenMP runtime（libgomp.so.1），python:3.11-slim 預設沒有，
# 缺這個 import lightgbm 會 OSError: libgomp.so.1: cannot open shared object file。
RUN apt-get update \
    && apt-get install -y --no-install-recommends libgomp1 \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

CMD ["python", "worker.py"]
