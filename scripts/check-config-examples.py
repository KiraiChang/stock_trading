#!/usr/bin/env python3
"""檢查 repo 裡寫過的範例憑證都在設定契約的公開範例清單裡（I-132 的 E）。

backend 的設定契約（backend/internal/config/contract.go）在 production 會擋下「repo 裡公開的範例值」。
清單是寫死在程式裡的常數，以後有人在 compose、deploy.sh 或 config.yaml 新增一個範例值卻忘了補清單，
production 就會放行它。這支腳本掃那些檔案，找出寫過的範例值，跟 contract.go 的清單比對。

Go 的單元測試只掃得到 backend/config.yaml（backend/scripts/test.sh 只掛 backend/），所以 repo 其他檔案由這裡檢查。

用法：python3 -I scripts/check-config-examples.py
結束碼：0 全部都在清單裡、1 有漏、2 讀不到檔案或解析不了清單。
⛔ 輸出只有檔名、類別與「不在清單」的值；這些值本來就寫在 repo 裡、是公開的範例。
"""
import glob
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
CONTRACT = os.path.join(ROOT, "backend", "internal", "config", "contract.go")
FILES = sorted(glob.glob(os.path.join(ROOT, "docker-compose*.yml"))) + [
    os.path.join(ROOT, "deploy.sh"),
    os.path.join(ROOT, ".env.example"),
    os.path.join(ROOT, "backend", "config.yaml"),
    os.path.join(ROOT, "python", "config.yaml"),
]
LISTS = ["PublicJWTSecrets", "PublicFinMindKeys", "PublicFugleKeys", "PublicRedisPasswords",
         "PublicDBUserPasswords", "PublicDBPasswords"]

# (類別, 正規式, 取值的方式, 要比對的清單)
PATTERNS = [
    ("JWT", re.compile(r'AUTH_JWT_SECRET[:=]\s*"?\$\{AUTH_JWT_SECRET:-([^}]+)\}'), ["PublicJWTSecrets"]),
    ("JWT", re.compile(r'AUTH_JWT_SECRET="([^"]+)"'), ["PublicJWTSecrets"]),
    ("JWT", re.compile(r'jwt_secret:\s*"([^"]+)"'), ["PublicJWTSecrets"]),
    ("FinMind", re.compile(r'FINMIND_API_KEY[:=]\s*"?\$\{FINMIND_API_KEY:-([^}]+)\}'), ["PublicFinMindKeys"]),
    ("FinMind", re.compile(r'FINMIND_API_KEY="([^"]+)"'), ["PublicFinMindKeys"]),
    ("Fugle", re.compile(r'FUGLE_API_KEY[:=]\s*"?\$\{FUGLE_API_KEY:-([^}]+)\}'), ["PublicFugleKeys"]),
    ("Fugle", re.compile(r'FUGLE_API_KEY="([^"]+)"'), ["PublicFugleKeys"]),
    ("API key", re.compile(r'api_key:\s*"([^"]+)"'), ["PublicFinMindKeys", "PublicFugleKeys"]),
    # .env.example 的 KEY=value（機密應該留空；有寫值就要是清單裡的範例值）
    ("JWT", re.compile(r"^AUTH_JWT_SECRET='?([^'\n]+?)'?$", re.M), ["PublicJWTSecrets"]),
    ("FinMind", re.compile(r"^FINMIND_API_KEY='?([^'\n]+?)'?$", re.M), ["PublicFinMindKeys"]),
    ("Fugle", re.compile(r"^FUGLE_API_KEY='?([^'\n]+?)'?$", re.M), ["PublicFugleKeys"]),
    ("Redis", re.compile(r'\$\{DEV_REDIS_PASSWORD:-([^}]+)\}'), ["PublicRedisPasswords"]),
    ("DB 密碼", re.compile(r'(?:POSTGRES_PASSWORD|MYSQL_PASSWORD|MYSQL_ROOT_PASSWORD):\s*"?([^"\s]+)'), ["PublicDBPasswords"]),
]
DSN_URL = re.compile(r'://([^:/@\s"]+):([^@\s"]+)@')
DSN_MYSQL = re.compile(r'"([A-Za-z0-9_]+):([^@\s"]+)@tcp\(')


def load_lists():
    try:
        src = open(CONTRACT, encoding="utf-8").read()
    except OSError:
        return None
    lists = {}
    for name in LISTS:
        m = re.search(name + r'\s*=\s*\[\]string\{([^}]*)\}', src)
        if not m:
            return None
        lists[name] = re.findall(r'"([^"]*)"', m.group(1))
    return lists


def main():
    lists = load_lists()
    if lists is None:
        print("解析不了 contract.go 的公開範例清單", file=sys.stderr)
        return 2
    missing, checked = [], 0
    for path in FILES:
        rel = os.path.relpath(path, ROOT)
        try:
            text = open(path, encoding="utf-8").read()
        except OSError:
            print(f"讀不到 {rel}", file=sys.stderr)
            return 2
        for kind, rx, names in PATTERNS:
            for value in rx.findall(text):
                if value.startswith("$"):
                    continue  # 是變數引用，不是寫死的值
                checked += 1
                if not any(value in lists[n] for n in names):
                    missing.append(f"{rel}：{kind} {value!r} 不在 {'／'.join(names)}")
        for user, pw in DSN_URL.findall(text) + DSN_MYSQL.findall(text):
            if "$" in user or "$" in pw:
                continue
            checked += 1
            if f"{user}:{pw}" not in lists["PublicDBUserPasswords"] and pw not in lists["PublicDBPasswords"]:
                missing.append(f"{rel}：DSN 帳密 {user}:{pw} 不在 PublicDBUserPasswords／PublicDBPasswords")
    if missing:
        print("\n".join(missing))
        print(f"==> check-config-examples：檢查 {checked} 個範例值，{len(missing)} 個不在清單")
        return 1
    print(f"==> check-config-examples：檢查 {checked} 個範例值，全部在清單裡（{len(FILES)} 個檔案）")
    return 0


if __name__ == "__main__":
    sys.exit(main())
