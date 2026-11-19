#!/usr/bin/env python3
"""Fake OpenAI-compatible provider for scripts/e2e/ac-m2.sh (T19).

Serves POST /chat/completions over SSE on 127.0.0.1 only; it never calls out
and requires no key. The answer depends on the conversation:

- a message that asks to write hands out one write_note tool call, then a
  final answer once the tool result is in;
- anything else streams a short Chinese answer.

Every answer is sent in chunks so the window really streams, and the usage
block is DeepSeek shaped with no cost field, so the receipt must render
"— 未提供" rather than $0.00.
"""

import argparse
import json
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

USAGE = {
    "prompt_tokens": 12,
    "completion_tokens": 8,
    "total_tokens": 20,
    "prompt_cache_hit_tokens": 4,
    "prompt_cache_miss_tokens": 8,
}

WRITE_PATH = "raws/ac.md"
WRITE_CONTENT = "# AC-M2\n\n由假供应商写入。\n"

# The perf script asks for a long answer: 10k Chinese chars in 100 chunks with
# a 40ms gap, so the window really streams for about four seconds.
LONG_ANSWER_BASE = (
    "知识库的整理原则是保留来源与上下文，先确认目录结构，再决定写入位置。"
    "每条结论都应当能回溯到具体的文件和行号，无法回溯的内容要明确标注为推断。"
    "批量修改之前先给出清单，等待确认后再执行；归档优先于删除，删除优先于永久删除。"
)
LONG_CHUNK_CHARS = 100
LONG_CHUNK_GAP = 0.04


def last_user_text(messages):
    for message in reversed(messages):
        if message.get("role") == "user":
            return str(message.get("content") or "")
    return ""


def plan(messages):
    text = last_user_text(messages)
    has_tool_result = any(message.get("role") == "tool" for message in messages)
    if "长回答" in text:
        return {"kind": "long"}
    if "写入" in text or "写文件" in text:
        if has_tool_result:
            return {"kind": "text", "text": "已写入 raws/ac.md，本次改动已记录。"}
        return {"kind": "tool"}
    if "整理归档" in text:
        return {"kind": "text", "text": "我会按主题把这个目录整理归档，先列出候选文件。"}
    return {"kind": "text", "text": "你好，我是 Agnes。已收到你的消息。"}


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        pass

    def do_GET(self):
        if self.path == "/health":
            self.send_response(200)
            self.send_header("Content-Type", "text/plain")
            self.end_headers()
            self.wfile.write(b"ok")
            return
        self.send_error(404)

    def do_POST(self):
        if self.path.split("?")[0] != "/chat/completions":
            self.send_error(404)
            return
        length = int(self.headers.get("Content-Length") or 0)
        try:
            body = json.loads(self.rfile.read(length) or b"{}")
        except json.JSONDecodeError:
            self.send_error(400)
            return
        messages = body.get("messages") or []
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.end_headers()
        self.stream(plan(messages))

    def emit(self, payload):
        data = json.dumps(payload, ensure_ascii=False)
        self.wfile.write(("data: " + data + "\n\n").encode("utf-8"))
        self.wfile.flush()

    def stream(self, answer):
        if answer["kind"] == "long":
            text = (LONG_ANSWER_BASE * 200)[:10_000]
            for start in range(0, len(text), LONG_CHUNK_CHARS):
                self.emit({"choices": [{"delta": {"content": text[start:start + LONG_CHUNK_CHARS]}}]})
                time.sleep(LONG_CHUNK_GAP)
            self.emit({
                "choices": [{"delta": {}, "finish_reason": "stop"}],
                "usage": USAGE,
            })
        elif answer["kind"] == "tool":
            arguments = json.dumps(
                {"path": WRITE_PATH, "content": WRITE_CONTENT}, ensure_ascii=False
            )
            self.emit({
                "choices": [{"delta": {"tool_calls": [{
                    "index": 0,
                    "id": "call_fake_1",
                    "type": "function",
                    "function": {"name": "write_note", "arguments": arguments},
                }]}}],
            })
            self.emit({
                "choices": [{"delta": {}, "finish_reason": "tool_calls"}],
                "usage": USAGE,
            })
        else:
            text = answer["text"]
            middle = max(1, len(text) // 2)
            for piece in (text[:middle], text[middle:]):
                if piece:
                    self.emit({"choices": [{"delta": {"content": piece}}]})
            self.emit({
                "choices": [{"delta": {}, "finish_reason": "stop"}],
                "usage": USAGE,
            })
        self.wfile.write(b"data: [DONE]\n\n")
        self.wfile.flush()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, default=18402)
    args = parser.parse_args()
    server = ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    print(f"fake provider on 127.0.0.1:{args.port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
