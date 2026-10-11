#!/usr/bin/env python3
"""Seedance 接入包静态扫描 + 网关探测工具（只读为主）。

两种模式：
  1. scan   —— 静态扫描整个目录，寻找泄漏的 API Key / secret / 高熵字符串。
  2. probe  —— 对网关发起只读探测（/v1/models、/seedance/capabilities 等），
               验证 Key 是否有效并枚举可见目录。

默认只跑 scan；probe 需要显式指定，且只使用不计费接口。

用法：
  python3 tools/seedance_key_hunt.py scan
  python3 tools/seedance_key_hunt.py probe                       # 无 Key，验证 401 行为
  SEEDANCE_API_KEY='xxx' python3 tools/seedance_key_hunt.py probe # 验证真实 Key
  python3 tools/seedance_key_hunt.py all --json report.json
"""

from __future__ import annotations

import argparse
import json
import math
import os
import re
import sys
import urllib.error
import urllib.request
from collections import Counter
from pathlib import Path

# ---------------------------------------------------------------------------
# 静态扫描规则
# ---------------------------------------------------------------------------

# 明确的密钥前缀 / 赋值模式
HARD_PATTERNS: list[tuple[str, str]] = [
    ("bearer_literal", r"(?i)bearer\s+([A-Za-z0-9._\-]{16,})"),
    ("openai_style", r"\bsk-[A-Za-z0-9_\-]{16,}\b"),
    ("anthropic_style", r"\bsk-ant-[A-Za-z0-9_\-]{16,}\b"),
    ("aws_akid", r"\b(?:AKIA|ASIA)[0-9A-Z]{16}\b"),
    ("google_api", r"\bAIza[0-9A-Za-z_\-]{35}\b"),
    ("github_pat", r"\bgh[pousr]_[A-Za-z0-9]{30,}\b"),
    ("slack_token", r"\bxox[baprs]-[A-Za-z0-9\-]{10,}\b"),
    ("jwt", r"\beyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\b"),
    ("private_key", r"-----BEGIN [A-Z ]*PRIVATE KEY-----"),
    # 赋值形式：api_key = "xxx" / "api_key": "xxx"
    (
        "key_assignment",
        r"""(?i)\b(?:api[_-]?key|secret|token|password|passwd|access[_-]?key)\b
            \s*[:=]\s*["']([^"'\n]{12,})["']""",
    ),
]

# 允许出现的占位符 / 模板值，命中这些不算泄漏
PLACEHOLDER_RE = re.compile(
    r"""(?ix)^(?:
        <[^>]*>                              # <自己的 API Key>
      | \{\{[^}]*\}\}                        # {{api_key}}
      | \$\{?[A-Za-z_][A-Za-z0-9_]*\}?       # $API_KEY / ${API_KEY}
      | REPLACE[_-]?WITH.*                   # REPLACE_WITH_YOUR_...
      | replace-with.*
      | your[_-].* | xxx+ | \*+ | -+ | \.{3}
      | example.* | demo.* | test.* | sample.*
      | placeholder.* | changeme.* | todo.*
      | none | null | true | false | ""
    )$"""
)

TEXT_SUFFIXES = {
    ".md", ".json", ".py", ".mjs", ".js", ".ts", ".txt", ".sh",
    ".yml", ".yaml", ".env", ".toml", ".ini", ".cfg", ".html",
}

# 令牌格式约束：字母数字加 - _ . : /，长度区间见下
TOKEN_RE = re.compile(r"\b[A-Za-z0-9][A-Za-z0-9_\-.:/]{15,127}\b")
IDEM_PREFIX_RE = re.compile(r"(?i)^(example|replace|demo|test|sample|idem)")
# 已经是 URI / 路径 / 版本号形状的串不当作密钥
URI_SHAPE_RE = re.compile(r"(?i)^(?:https?|asset|file)://|^/|^[\w.\-]+/[\w.\-/]+$|^\d+(?:\.\d+)+$")
MARKDOWN_LINK_RE = re.compile(r"\]\(|https?://\S+")
# 文档/代码里常见但绝非密钥的标识符形状
NOT_SECRET_RE = re.compile(
    r"""(?ix)^(?:
        (?:seedance|video|image|audio|asset|media|task|content|callback|storage)[-_].*  # 模型/字段/路径名
      | [a-z]+_[a-z_]+                                        # snake_case 字段名 max_bytes
      | [a-z]+(?:[A-Z][a-z]+)+                                # camelCase 标识符
      | [A-Za-z]+-[A-Za-z0-9]+(?:-[A-Za-z0-9]+)*              # kebab-case 标识符
      | X-[A-Za-z-]+                                          # HTTP 头名
      | (?:GET|POST|PUT|DELETE|HEAD|PATCH)$
    )$"""
)


def shannon_entropy(value: str) -> float:
    """计算字符串的香农熵（bit/char）。"""
    if not value:
        return 0.0
    counts = Counter(value)
    length = len(value)
    return -sum((n / length) * math.log2(n / length) for n in counts.values())


def is_likely_secret(value: str) -> tuple[bool, str]:
    """判断高熵候选是否像真实密钥。"""
    if PLACEHOLDER_RE.match(value):
        return False, "placeholder/template"
    if IDEM_PREFIX_RE.match(value):
        return False, "idempotency-key 样例"
    if URI_SHAPE_RE.match(value):
        return False, "URL/路径/版本号"
    if NOT_SECRET_RE.match(value):
        return False, "文档标识符/字段名"
    # 必须有足够字符种类，纯字母单词（如 generations）不算
    classes = sum(
        bool(re.search(p, value)) for p in (r"[a-z]", r"[A-Z]", r"[0-9]", r"[_\-.:/]")
    )
    if classes < 2:
        return False, "字符种类不足"
    # 真实 API Key 通常同时含数字与字母；缺少数字的候选降级为低置信
    if not re.search(r"[0-9]", value):
        return False, "不含数字"
    if shannon_entropy(value) < 3.5:
        return False, "熵过低"
    return True, ""


def redact(value: str) -> str:
    """脱敏：只保留首尾少量字符。"""
    if len(value) <= 8:
        return "*" * len(value)
    return f"{value[:4]}{'*' * 6}{value[-4:]}"


def iter_files(root: Path):
    for path in sorted(root.rglob("*")):
        if path.is_file() and not path.name.startswith("."):
            yield path


def scan_file(path: Path, root: Path) -> list[dict]:
    findings: list[dict] = []
    try:
        raw = path.read_bytes()
    except OSError as exc:
        return [{"type": "read_error", "file": str(path), "detail": str(exc)}]
    text = raw.decode("utf-8", "replace")
    rel = str(path.relative_to(root))

    for lineno, line in enumerate(text.splitlines(), 1):
        for name, pattern in HARD_PATTERNS:
            for match in re.finditer(pattern, line):
                value = match.group(1) if match.groups() else match.group(0)
                if PLACEHOLDER_RE.match(value):
                    continue
                findings.append(
                    {
                        "type": name,
                        "severity": "high",
                        "file": rel,
                        "line": lineno,
                        "match": redact(value),
                        "context": line.strip()[:160],
                    }
                )

    # 高熵扫描：只对文本类文件、按“疑似 key 的 token”逐个评估
    if path.suffix.lower() in TEXT_SUFFIXES:
        for lineno, line in enumerate(text.splitlines(), 1):
            if MARKDOWN_LINK_RE.search(line) and "curl" not in line:
                continue
            for match in TOKEN_RE.finditer(line):
                value = match.group(0)
                ok, reason = is_likely_secret(value)
                if not ok:
                    continue
                findings.append(
                    {
                        "type": "high_entropy",
                        "severity": "medium",
                        "file": rel,
                        "line": lineno,
                        "match": redact(value),
                        "entropy": round(shannon_entropy(value), 2),
                        "context": line.strip()[:160],
                    }
                )
    return findings


def run_scan(root: Path) -> dict:
    findings: list[dict] = []
    files = list(iter_files(root))
    for path in files:
        findings.extend(scan_file(path, root))

    # 汇总硬编码端点与占位符，便于人工复核
    hosts: Counter[str] = Counter()
    for path in files:
        if path.suffix.lower() not in TEXT_SUFFIXES:
            continue
        text = path.read_text("utf-8", "replace")
        for host in re.findall(r"https?://([A-Za-z0-9.\-]+)", text):
            hosts[host] += 1

    return {
        "scanned_files": len(files),
        "findings": findings,
        "hosts": dict(hosts.most_common()),
    }


# ---------------------------------------------------------------------------
# 网关探测（只读、不计费）
# ---------------------------------------------------------------------------

READ_ONLY_ENDPOINTS = [
    ("GET", "/v1/models"),
    ("GET", "/seedance/capabilities"),
    ("GET", "/v1/videos?page=1&page_size=5"),
    ("GET", "/seedance/media?page=1&page_size=5"),
]


def request(base: str, key: str, method: str, path: str, timeout: int = 25):
    url = base + path
    headers = {"Accept": "application/json"}
    if key:
        headers["Authorization"] = f"Bearer {key}"
    req = urllib.request.Request(url, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            body = resp.read(200_000).decode("utf-8", "replace")
            return resp.status, body
    except urllib.error.HTTPError as exc:
        return exc.code, exc.read(200_000).decode("utf-8", "replace")
    except urllib.error.URLError as exc:
        return None, f"network error: {exc.reason}"


def run_probe(base: str, key: str) -> dict:
    base = base.rstrip("/")
    results = []
    for method, path in READ_ONLY_ENDPOINTS:
        status, body = request(base, key, method, path)
        entry = {"method": method, "path": path, "status": status, "body": body[:2000]}
        if status == 200:
            try:
                payload = json.loads(body)
                if path == "/v1/models":
                    ids = [m.get("id") for m in payload.get("data", []) if isinstance(m, dict)]
                    entry["models"] = ids
                entry["ok"] = True
            except json.JSONDecodeError:
                entry["ok"] = False
        else:
            entry["ok"] = False
            try:
                entry["error_code"] = json.loads(body).get("error", {}).get("code")
            except json.JSONDecodeError:
                pass
        results.append(entry)

    key_state = "absent"
    if key:
        models = next((r for r in results if r["path"] == "/v1/models"), {})
        if models.get("status") == 200:
            key_state = "valid"
        elif models.get("status") in (401, 403):
            key_state = "invalid"
        else:
            key_state = "unknown"
    return {"base_url": base, "key_state": key_state, "results": results}


# ---------------------------------------------------------------------------

def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__,
                                     formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("mode", choices=("scan", "probe", "all"))
    parser.add_argument("--root", type=Path,
                        default=Path(__file__).resolve().parent.parent,
                        help="扫描根目录（默认接入包根目录）")
    parser.add_argument("--base-url",
                        default=os.environ.get("SEEDANCE_BASE_URL",
                                               "http://106.54.45.168/seedance-gateway"))
    parser.add_argument("--api-key", default=os.environ.get("SEEDANCE_API_KEY", ""))
    parser.add_argument("--json", type=Path, help="把完整结果写入 JSON 文件")
    args = parser.parse_args()

    report: dict = {}
    exit_code = 0

    if args.mode in ("scan", "all"):
        scan = run_scan(args.root.resolve())
        report["scan"] = scan
        hard = [f for f in scan["findings"] if f["severity"] == "high"]
        print(f"[scan] 扫描文件 {scan['scanned_files']} 个")
        print(f"[scan] 硬编码密钥命中 {len(hard)} 处，高熵可疑 {len(scan['findings']) - len(hard)} 处")
        for f in scan["findings"]:
            print(f"  - [{f['severity']}] {f['type']} {f['file']}:{f['line']} -> {f['match']}")
        if not scan["findings"]:
            print("  (无命中：目录内未发现泄漏的 Key)")
        print("[scan] 出现的域名:")
        for host, n in scan["hosts"].items():
            print(f"  - {host} x{n}")
        if hard:
            exit_code = 2

    if args.mode in ("probe", "all"):
        probe = run_probe(args.base_url, args.api_key)
        report["probe"] = probe
        print(f"\n[probe] base={probe['base_url']} key_state={probe['key_state']}")
        for r in probe["results"]:
            print(f"  - {r['method']} {r['path']} -> HTTP {r['status']}")
            if r.get("models"):
                print(f"      models: {', '.join(str(m) for m in r['models'])}")
            elif not r["ok"]:
                print(f"      {r['body'][:200]}")
        if args.api_key and probe["key_state"] != "valid":
            exit_code = max(exit_code, 1)

    if args.json:
        args.json.write_text(json.dumps(report, ensure_ascii=False, indent=2), "utf-8")
        print(f"\n报告已写入 {args.json}")

    return exit_code


if __name__ == "__main__":
    sys.exit(main())
