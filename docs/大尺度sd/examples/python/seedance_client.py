#!/usr/bin/env python3
"""Customer-gateway example. Default action is read-only; --run creates a paid task."""

import argparse
import json
import os
import re
import sys
import time
import uuid
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


def api(base, key, method, path, payload=None, idem=None):
    headers = {"Authorization": f"Bearer {key}"}
    data = None
    if payload is not None:
        headers["Content-Type"] = "application/json"
        data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
    if idem:
        headers["Idempotency-Key"] = idem
    request = Request(base + path, data=data, headers=headers, method=method)
    try:
        with urlopen(request, timeout=210) as response:
            return json.load(response)
    except HTTPError as exc:
        raw = exc.read().decode("utf-8", "replace")
        try:
            detail = json.loads(raw).get("error", {})
        except json.JSONDecodeError:
            detail = {}
        raise RuntimeError(f"HTTP {exc.code}: {detail.get('code', 'unknown_error')}: {detail.get('message', raw[:300])}") from exc
    except URLError as exc:
        raise RuntimeError(f"Network error; request outcome may be unknown: {exc.reason}") from exc


def poll(base, key, path, terminal, failed, interval, timeout):
    deadline = time.monotonic() + timeout
    while True:
        item = api(base, key, "POST" if isinstance(path, tuple) else "GET",
                   path[0] if isinstance(path, tuple) else path,
                   path[1] if isinstance(path, tuple) else None)
        status = item.get("status")
        print(f"{item.get('id', '?')}: {status}", flush=True)
        if status == terminal:
            return item
        if status in failed:
            raise RuntimeError(f"Task ended as {status}: {item.get('error') or item}")
        if time.monotonic() >= deadline:
            raise TimeoutError(f"Polling timed out; keep task ID {item.get('id')} for later query")
        time.sleep(interval)


def download(base, key, path, destination):
    if not (path.startswith("/v1/videos/") or path.startswith("/v1/tasks/")) or "://" in path:
        raise ValueError("Unexpected download path in task response")
    request = Request(base + path, headers={"Authorization": f"Bearer {key}"})
    with urlopen(request, timeout=120) as response, destination.open("wb") as output:
        while chunk := response.read(1024 * 1024):
            output.write(chunk)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run", action="store_true", help="Import reference if provided and create a billable video")
    parser.add_argument("--model", default="seedance-2.0-720p")
    parser.add_argument("--prompt", default="古代街道，镜头缓慢向前推进")
    parser.add_argument("--seconds", type=int, default=5)
    parser.add_argument("--reference-url", help="Public HTTP(S) image/video/audio file URL")
    parser.add_argument("--asset-uri", help="Previously imported asset://task_... for an exact-request retry")
    parser.add_argument("--reference-type", choices=("image", "video", "audio"), default="video")
    parser.add_argument("--idempotency-key", help="Reuse this exact key and request body for retry")
    parser.add_argument("--output", type=Path, default=Path("seedance-output.mp4"))
    args = parser.parse_args()

    base = os.environ.get("SEEDANCE_BASE_URL", "").rstrip("/")
    key = os.environ.get("SEEDANCE_API_KEY", "")
    if not base.startswith(("http://", "https://")) or not key:
        parser.error("Set SEEDANCE_BASE_URL and SEEDANCE_API_KEY in the server environment")

    models = api(base, key, "GET", "/v1/models")
    ids = {item["id"] for item in models.get("data", [])}
    print("Visible models:", ", ".join(sorted(ids)))
    if not args.run:
        print("Read-only check complete. Pass --run to create a billable task.")
        return
    if args.model not in ids:
        parser.error(f"Model {args.model} is not visible to this API Key")

    if args.reference_url and args.asset_uri:
        parser.error("Choose --reference-url or --asset-uri, not both")
    asset_uri = args.asset_uri
    if asset_uri and not re.fullmatch(r"asset://task_[A-Za-z0-9_-]+", asset_uri):
        parser.error("--asset-uri must start with asset://task_")
    if (args.reference_url or asset_uri) and "media-service" not in ids:
        parser.error("media-service is required for reference material")
    if args.reference_url:
        if not args.reference_url.startswith(("http://", "https://")):
            parser.error("--reference-url must be a raw HTTP(S) URL")
        imported = api(base, key, "POST", "/seedance/media", {
            "action": "import", "url": args.reference_url,
            "media_type": args.reference_type, "title": "API example reference",
        })
        print("Asset ID:", imported["id"])
        asset_uri = imported["asset_uri"]
    if asset_uri:
        print("Asset URI:", asset_uri, flush=True)
        asset = poll(base, key, ("/seedance/media", {"action": "get", "id": asset_uri[8:]}),
                     "succeeded", {"failed", "deleted"}, 2, 300)
        preflight = api(base, key, "POST", "/seedance/media/preflight", {
            "asset_id": asset["id"], "model": args.model,
            "media_type": args.reference_type, "role": "reference_" + args.reference_type,
        })
        print("Preflight:", preflight.get("compatibility"))
        if preflight.get("compatibility") == "unsupported":
            raise RuntimeError("Known incompatible reference; no billable task submitted")

    if asset_uri and args.reference_type in ("video", "audio"):
        kind = args.reference_type + "_url"
        payload = {"model": args.model, "duration": args.seconds, "ratio": "16:9", "n": 1,
                   "content": [{"type": "text", "text": args.prompt},
                               {"type": kind, kind: {"url": asset_uri}, "role": "reference_" + args.reference_type}]}
    else:
        payload = {"model": args.model, "prompt": args.prompt, "seconds": args.seconds}
        if asset_uri:
            payload["input_reference"] = asset_uri

    idem = args.idempotency_key or "example-" + uuid.uuid4().hex
    print("Idempotency-Key:", idem, flush=True)
    print("Submitting one billable video request.", flush=True)
    created = api(base, key, "POST", "/v1/videos", payload, idem)
    task_id = created["id"]
    print("Video task ID:", task_id, flush=True)
    video = poll(base, key, "/v1/videos/" + task_id, "completed", {"failed"}, 10, 1200)
    path = video.get("url") or (video.get("urls") or [None])[0]
    if not path:
        raise RuntimeError("Completed task has no downloadable URL")
    download(base, key, path, args.output)
    print("Saved:", args.output.resolve())


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, TimeoutError, HTTPError, URLError, KeyError, ValueError) as exc:
        print(f"Error: {exc}", file=sys.stderr)
        sys.exit(1)
