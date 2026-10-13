#!/usr/bin/env python3
"""
Td 上游 mock 服务，用于本地端到端验证（不消耗真实额度）。

模拟土豆（ai-tudou.net）的视频接口行为：
  POST /v1/videos/generations   提交任务，返回 {code,message,data:{id,...}}
  GET  /v1/tasks/{task_id}      查询任务，首次 processing，之后 completed

并记录收到的请求体到 MOCK_LOG（默认 mock_upstream_requests.jsonl），
供核对 new-api 实际发往上游的参数是否正确。

用法：
    python3 scripts/mock_td_upstream.py [PORT]
默认端口 9799。
"""
import json
import os
import sys
import threading
import time
from http.server import BaseHTTPRequestHandler, HTTPServer

PORT = int(sys.argv[1]) if len(sys.argv) > 1 else 9799
LOG_PATH = os.environ.get("MOCK_LOG", "mock_upstream_requests.jsonl")

_lock = threading.Lock()
_seq = 0
_tasks = {}  # task_id -> {"created": ts, "polls": n}


def _log(entry):
    with _lock:
        with open(LOG_PATH, "a", encoding="utf-8") as f:
            f.write(json.dumps(entry, ensure_ascii=False) + "\n")


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, fmt, *args):
        pass  # 静默，避免污染测试输出

    def _send(self, code, payload):
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_POST(self):
        global _seq
        length = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(length) if length else b""
        try:
            body = json.loads(raw.decode("utf-8")) if raw else {}
        except Exception:
            body = {"_raw": raw.decode("utf-8", "replace")}

        _log({
            "ts": time.time(),
            "method": "POST",
            "path": self.path,
            "auth": self.headers.get("Authorization", ""),
            "body": body,
        })

        if self.path.rstrip("/") == "/v1/videos/generations":
            with _lock:
                _seq += 1
                tid = "task_mock_%04d" % _seq
            _tasks[tid] = {"created": time.time(), "polls": 0}
            # 对齐上游文档：code/message/data.id + task_id + status + estimated_time
            self._send(200, {
                "code": 200,
                "message": "ok",
                "data": {
                    "id": tid,
                    "task_id": tid,
                    "status": "submitted",
                    "created": int(time.time()),
                    "estimated_time": 5,
                },
            })
            return

        # 图片异步提交
        if self.path.rstrip("/") == "/v1/images/generations/async":
            with _lock:
                _seq += 1
                tid = "task_img_%04d" % _seq
            _tasks[tid] = {"created": time.time(), "polls": 0, "image": True}
            self._send(200, {
                "code": 200,
                "message": "ok",
                "data": {"id": tid, "status": "submitted", "created": int(time.time())},
            })
            return

        self._send(404, {"code": 404, "message": "not found"})

    def do_GET(self):
        path = self.path.rstrip("/")
        _log({"ts": time.time(), "method": "GET", "path": self.path,
              "auth": self.headers.get("Authorization", "")})

        if path.startswith("/v1/tasks/"):
            tid = path.rsplit("/", 1)[-1]
            st = _tasks.get(tid)
            if st is None:
                self._send(200, {"code": 404, "message": "task not found"})
                return
            st["polls"] += 1
            is_img = st.get("image", False)
            # 首次返回 processing（视频）/ queued（图片首轮也走 processing），
            # 第二次起返回 completed，确保轮询逻辑被真实走到。
            if st["polls"] < 2:
                status = "processing"
                self._send(200, {
                    "code": 200, "message": "ok",
                    "data": {"id": tid, "status": status, "progress": 45},
                })
                return
            if is_img:
                self._send(200, {
                    "code": 200, "message": "ok",
                    "data": {
                        "id": tid, "status": "completed", "progress": 100,
                        "result": {"images": [
                            {"url": ["https://mock.cdn/img_%s_0.png" % tid],
                             "expires_at": int(time.time()) + 86400}
                        ]},
                    },
                })
                return
            self._send(200, {
                "code": 200, "message": "ok",
                "data": {
                    "id": tid, "status": "completed", "progress": 100,
                    "result": {
                        "video_url": "https://mock.cdn/%s_0.mp4" % tid,
                        "videos": [{
                            "url": ["https://mock.cdn/%s_0.mp4" % tid],
                            "expires_at": int(time.time()) + 86400,
                        }],
                    },
                },
            })
            return

        self._send(404, {"code": 404, "message": "not found"})


if __name__ == "__main__":
    if os.path.exists(LOG_PATH):
        os.remove(LOG_PATH)
    srv = HTTPServer(("127.0.0.1", PORT), Handler)
    print("mock td upstream listening on http://127.0.0.1:%d" % PORT, flush=True)
    print("request log: %s" % LOG_PATH, flush=True)
    srv.serve_forever()
