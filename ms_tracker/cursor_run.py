#!/usr/bin/env python3
"""Один прогон Cursor SDK локально в клоне репозитория (как myvibelab)."""

from __future__ import annotations

import json
import os
import sys

from cursor_sdk import Agent, AgentOptions, CursorAgentError, LocalAgentOptions


def _text(result: object) -> str:
    raw = getattr(result, "result", None)
    if raw is None:
        raw = getattr(result, "text", None)
    if callable(raw):
        try:
            raw = raw()
        except TypeError:
            raw = None
    return str(raw or "").strip()


def main() -> int:
    try:
        payload = json.load(sys.stdin)
    except json.JSONDecodeError as exc:
        print(json.dumps({"ok": False, "error": f"stdin json: {exc}"}), flush=True)
        return 1

    cwd = str(payload.get("cwd") or "").strip()
    prompt = str(payload.get("prompt") or "").strip()
    model = str(payload.get("model") or "composer-2.5").strip() or "composer-2.5"
    api_key = (os.environ.get("CURSOR_API_KEY") or "").strip()
    if not api_key:
        print(json.dumps({"ok": False, "error": "нет CURSOR_API_KEY"}), flush=True)
        return 1
    if not cwd or not os.path.isdir(cwd):
        print(json.dumps({"ok": False, "error": "нет каталога репозитория"}), flush=True)
        return 1
    if not prompt:
        print(json.dumps({"ok": False, "error": "пустой промпт"}), flush=True)
        return 1

    try:
        result = Agent.prompt(
            prompt,
            AgentOptions(
                api_key=api_key,
                model=model,
                local=LocalAgentOptions(cwd=cwd, setting_sources=[]),
            ),
        )
    except CursorAgentError as err:
        print(
            json.dumps(
                {
                    "ok": False,
                    "error": err.message,
                    "retryable": bool(getattr(err, "is_retryable", False)),
                },
                ensure_ascii=False,
            ),
            flush=True,
        )
        return 1
    except Exception as exc:  # noqa: BLE001
        print(json.dumps({"ok": False, "error": str(exc)}, ensure_ascii=False), flush=True)
        return 1

    status = str(getattr(result, "status", "") or "").lower()
    text = _text(result)
    if status == "error":
        print(
            json.dumps(
                {"ok": False, "error": text or "cursor run error", "status": status},
                ensure_ascii=False,
            ),
            flush=True,
        )
        return 2
    print(
        json.dumps({"ok": True, "status": status or "finished", "result": text}, ensure_ascii=False),
        flush=True,
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
