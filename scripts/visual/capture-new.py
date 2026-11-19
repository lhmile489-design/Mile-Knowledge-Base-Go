#!/usr/bin/env python3
"""Capture the new app's UI states for the visual comparison page (M3 T10).

Runs the e2e build against a temporary sample vault and a temporary APPDATA
(the real app data is never touched), drives states through the automation
socket, and writes PNGs to design/compare/new/. A local compare.html pairs
each shot with the legacy baseline. Only read-only access to the legacy repo.

Usage (from the repo root):
  python -I scripts/visual/capture-new.py [--port 18431] [--provider-port 18432]
"""

import base64
import hashlib
import json
import os
import re
import shutil
import socket
import subprocess
import sys
import tempfile
import time
import traceback
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
APP = ROOT / "build" / "windows-amd64" / "Mile Knowledge Base.exe"
OUT = ROOT / "design" / "compare" / "new"
BASELINE = ROOT / "design" / "baseline"
MANIFEST = BASELINE / "manifest.json"

PORT = 18431
PROVIDER_PORT = 18432
FAKE_MODEL = "fake-model"


def arg(name, default):
    if name in sys.argv:
        return sys.argv[sys.argv.index(name) + 1]
    return default


PORT = int(arg("--port", PORT))
PROVIDER_PORT = int(arg("--provider-port", PROVIDER_PORT))


def log(message):
    print(message, flush=True)


def canon(path):
    return os.path.realpath(path)


def vault_id_of(root):
    return hashlib.sha256(root.lower().encode("utf-8")).hexdigest()[:16]


# ---------------------------------------------------------------------------
# Sample vault (mirrors design/baseline/capture.cjs buildVault)
# ---------------------------------------------------------------------------

PNG_RED = base64.b64decode(
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
)
BIG_LINE = "这是一行用于填充体积的中文与 english mixed 内容，重复排列以制造大文件。\n"


def build_vault():
    root = canon(tempfile.mkdtemp(prefix="mkb-visual-vault-"))

    def write(rel, text):
        full = Path(root) / rel
        full.parent.mkdir(parents=True, exist_ok=True)
        full.write_text(text, encoding="utf-8", newline="\n")

    for directory in [
        "skills/react", "skills/web-vue", "skills/nodejs", "skills/harmonyos",
        "raws", "Clippings", "_archives", "projects/campus-hub", "projects/mile-go",
        "tools", "AI Knowledge base", "dev-constraints", ".claude", ".obsidian", "my-notes",
    ]:
        (Path(root) / directory).mkdir(parents=True, exist_ok=True)

    write("skills/react/SKILL.md", "---\ntitle: React 技巧\ntags: [frontend, react]\n---\n\n# React\n\n受控组件与状态提升。参见 [[共享笔记]]。\n")
    write("skills/web-vue/SKILL.md", "# Vue\n\n组合式 API。\n")
    write("skills/nodejs/SKILL.md", "# Node\n\n事件循环。\n")
    write("skills/harmonyos/SKILL.md", "# 鸿蒙\n\nArkTS。\n")
    write("skills/react/deep/nested/note.md", "# 深层笔记\n\n嵌套目录内容。\n")

    (Path(root) / "raws" / "attachments").mkdir(parents=True, exist_ok=True)
    (Path(root) / "raws" / "attachments" / "img-red.png").write_bytes(PNG_RED)

    write("raws/富文本样例.md", "\n".join([
        "---", "title: 富文本样例", "tags: demo, 排版", "---", "",
        "# 富文本样例", "",
        "一段包含 **加粗**、`code` 与 ~~删除线~~ 的正文。", "",
        "| 名称 | 数量 | 备注 |", "| --- | --- | --- |", "| 苹果 | 3 | 新鲜 |", "| 梨 | 5 | 一般 |", "",
        "- [x] 已完成任务", "- [ ] 待办任务", "",
        "```js", "const x = 1", "console.log(x)", "```", "",
        "> 这是一段引用文字。", "",
        "库内图片：", "![红](attachments/img-red.png)", "",
        "链接：[[共享笔记]]（已解析）、[[不存在的页面]]（未创建）、[[同名文档]]（多个同名）。", "",
    ]))
    write("raws/共享笔记.md", "# 共享笔记\n\n被 [[富文本样例]] 与 [[首页]] 引用的目标。\n\n回链到 [[首页]]。\n")
    write("raws/首页.md", "# 首页\n\n引用 [[共享笔记]] 与 [[富文本样例]]。\n")
    write("raws/孤立文档.md", "# 孤立文档\n\n没有任何入链与出链。\n")
    write("raws/缺标题.md", "直接正文，没有标题行，标题回退为文件名。\n")
    write("raws/同名文档.md", "# 同名 A\n\nraws 下的同名文档。\n")
    write("my-notes/同名文档.md", "# 同名 B\n\nmy-notes 下的同名文档。\n")
    write("my-notes/中文文件名测试.md", "# 中文文件名\n\n用于检索与显示。\n")
    write("my-notes/a-very-long-file-name-that-should-not-break-the-grid-layout-at-all.md", "# 长文件名\n\n很长。\n")
    write("Clippings/剪藏一篇.md", "# 剪藏\n\n来自网页的剪藏内容，含「知识库」关键词。\n")
    write("AI Knowledge base/独立知识.md", "# 独立知识\n\nwiki 区文档。\n")
    write("dev-constraints/约束.md", "# 约束\n\n开发约束。\n")
    write("tools/工具说明.md", "# 工具\n\n工具说明。\n")
    write("projects/campus-hub/说明.md", "# 项目\n\n项目说明。\n")

    base = hashlib.sha256("# React\n\n受控组件与状态提升。\n".encode("utf-8")).hexdigest()
    write("projects/mile-go/react-skill-copy.md", "# React（本地定制版）\n\n副本已改。\n")
    write(".mile-kb/provenance.json", json.dumps({
        "version": 1,
        "records": [{
            "copyPath": "projects/mile-go/react-skill-copy.md",
            "sourcePath": "skills/react/SKILL.md",
            "baseRevision": 1,
            "baseHash": base,
            "copiedAt": "2026-11-19T00:00:00.000Z",
        }],
    }, ensure_ascii=False, indent=2) + "\n")

    write("raws/big-1m.md", "# 大文件 1MiB+\n\n" + BIG_LINE * (int(1.2 * 1024 * 1024 / len(BIG_LINE.encode())) + 1))
    write("raws/huge-2m.md", "# 大文件 2MiB+\n\n" + BIG_LINE * (int(2.2 * 1024 * 1024 / len(BIG_LINE.encode())) + 1))
    return root


def write_provider(root, base_url):
    settings = Path(root) / ".mile-kb" / "settings"
    settings.mkdir(parents=True, exist_ok=True)
    (settings / "providers.json").write_text(json.dumps({
        "version": 1,
        "providers": [{
            "name": "fakeprovider", "kind": "openai", "baseUrl": base_url,
            "models": [FAKE_MODEL], "defaultModel": FAKE_MODEL,
            "apiKeyEnv": "", "contextWindow": 64000,
        }],
    }, ensure_ascii=False, indent=2) + "\n", encoding="utf-8", newline="\n")


def write_settings(appdata, theme, vault_root, vault_id, layout=None):
    layout = layout or {
        "sidebarWidth": 232, "docWidth": 560, "docCollapsed": False,
        "sidebarCollapsed": False, "agnesControlsCollapsed": False,
        "agnesHistoryOpen": True, "agnesComposerHeight": 96,
    }
    payload = {
        "version": 1,
        "theme": theme,
        "closeBehavior": "ask",
        "layout": layout,
        "recentVaults": [{
            "id": vault_id, "root": vault_root, "displayName": "baseline-vault",
            "createdAt": "2026-11-19T00:00:00Z", "lastOpenedAt": "2026-11-19T00:00:00Z",
        }],
        "lastVaultId": vault_id,
    }
    # mygo names the user-data directory after the app identifier.
    data_dir = Path(appdata) / "Mile Knowledge Base"
    data_dir.mkdir(parents=True, exist_ok=True)
    (data_dir / "settings.json").write_text(
        json.dumps(payload, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8", newline="\n",
    )


# ---------------------------------------------------------------------------
# Automation client
# ---------------------------------------------------------------------------

class Driver:
    def __init__(self, port, timeout=120):
        self.sock = None
        self.reader = None
        self.seq = 0
        self.timeout = timeout
        self.port = port

    def connect(self, wait=30):
        deadline = time.time() + wait
        while time.time() < deadline:
            try:
                self.sock = socket.create_connection(("127.0.0.1", self.port), timeout=5)
                self.reader = self.sock.makefile("rb")
                return
            except OSError:
                time.sleep(0.3)
        raise RuntimeError("automation port never opened")

    def request(self, method, params=None):
        self.seq += 1
        payload = {"id": self.seq, "method": method}
        if params is not None:
            payload["params"] = params
        self.sock.settimeout(self.timeout)
        self.sock.sendall((json.dumps(payload) + "\n").encode("utf-8"))
        try:
            line = self.reader.readline()
        except OSError as error:
            # 一次超时后这条连接就不可再用：重连，把超时交给调用方处理。
            try:
                self.sock.close()
            except OSError:
                pass
            self.connect(10)
            raise RuntimeError(f"automation timeout: {error}")
        if not line:
            raise RuntimeError("automation socket closed")
        message = json.loads(line)
        if "error" in message:
            raise RuntimeError(message["error"].get("message") or message["error"].get("code"))
        return message.get("result")

    def js(self, script):
        result = self.request("eval", {"window": 1, "script": script})
        return result.get("value") if isinstance(result, dict) else result

    def wait(self, script, timeout=15, settle=0.0):
        deadline = time.time() + timeout
        last = None
        while time.time() < deadline:
            try:
                value = self.js(script)
                last = value
                if value:
                    if settle:
                        time.sleep(settle)
                    return value
            except RuntimeError:
                pass
            time.sleep(0.15)
        raise RuntimeError(f"wait timed out: {script[:120]} (last={last!r})")

    def capture(self):
        result = self.request("capture", {"window": 1})
        return base64.b64decode(result["png"])


DRIVER: Driver | None = None
ENTRIES = {}
LEGACY = {}


def shot(entry_id, name, note="", status="captured", theme=None):
    """Capture one state. A failure is recorded, never fatal."""
    entry = {
        "id": str(entry_id),
        "name": name,
        "file": "",
        "note": note,
        "status": status,
    }
    if LEGACY.get(str(entry_id)):
        entry["title"] = LEGACY[str(entry_id)].get("title", "")
        entry["prdRefs"] = LEGACY[str(entry_id)].get("prdRefs", [])
        entry["theme"] = theme or LEGACY[str(entry_id)].get("theme", "light")
        entry["size"] = LEGACY[str(entry_id)].get("size", "1440x900")
    try:
        if status == "captured":
            png = DRIVER.capture()
            file = f"{entry_id}-{name}.png"
            OUT.mkdir(parents=True, exist_ok=True)
            (OUT / file).write_bytes(png)
            entry["file"] = f"new/{file}"
    except Exception as error:  # noqa: BLE001 - a failed shot must not stop the run
        entry["status"] = "failed"
        entry["note"] = (entry["note"] + " " if entry["note"] else "") + str(error)
    ENTRIES[entry["id"]] = entry
    log(f"  [{entry['status']}] {entry['id']} {name} {entry['note']}")
    return entry


def step(label, fn, entry_id, name, **kwargs):
    try:
        fn()
        shot(entry_id, name, **kwargs)
    except Exception as error:  # noqa: BLE001
        shot(entry_id, name, note=str(error), status="failed", **kwargs)


def js_click(selector):
    return (
        "(() => { const el = document.querySelector(%s);"
        " if (!el) throw new Error('missing %s'); el.click(); return true; })()"
        % (json.dumps(selector), selector)
    )


def js_set_value(selector, value, kind="textarea"):
    proto = "HTMLTextAreaElement" if kind == "textarea" else "HTMLInputElement"
    return (
        "(() => { const el = document.querySelector(%s); if (!el) throw new Error('missing %s');"
        " const set = Object.getOwnPropertyDescriptor(window.%s.prototype, 'value').set;"
        " set.call(el, %s); el.dispatchEvent(new Event('input', { bubbles: true })); return true; })()"
        % (json.dumps(selector), selector, proto, json.dumps(value))
    )


def click_nav(section):
    return js_click(f'[data-testid="settings-nav-{section}"]')


def open_settings(section):
    DRIVER.js("window.__mkbApp.getState().openSettings(); true")
    DRIVER.wait("!!document.querySelector('[data-panel=\"settings-nav\"]')")
    deadline = time.time() + 8
    while time.time() < deadline:
        try:
            # 上一次设置的草稿会触发离开确认；这里直接放弃草稿，截图不依赖它。
            if DRIVER.js("!!document.querySelector('[data-testid=\"settings-leave-dialog\"]')"):
                DRIVER.js(js_click('[data-testid="settings-leave-discard"]'))
                time.sleep(0.3)
            DRIVER.js(click_nav(section))
            if DRIVER.js(
                f'!!document.querySelector(\'[data-testid="settings-nav-{section}"][data-active="true"]\')'
            ):
                return
        except RuntimeError:
            pass
        time.sleep(0.3)
    raise RuntimeError(f"settings section never active: {section}")


def open_doc(path):
    DRIVER.js(
        "(() => { const app = window.__mkbApp.getState();"
        f" window.__mkbDocs.getState().open(app.activeVaultId, {json.dumps(path)});"
        " app.setLayout({ docCollapsed: false }); return true; })()"
    )
    DRIVER.wait("!!document.querySelector('[data-panel=\"editor\"]')")
    DRIVER.wait(f"window.__mkbDocs.getState().activePath === {json.dumps(path)}")
    time.sleep(0.4)


def set_mode(label):
    DRIVER.js(
        "(() => { const tabs = [...document.querySelectorAll('[aria-label=\"文档模式\"] [role=tab]')];"
        f" const tab = tabs.find((t) => t.textContent.trim() === {json.dumps(label)});"
        " if (!tab) throw new Error('missing mode tab'); tab.click(); return true; })()"
    )
    time.sleep(0.5)


def main():
    global DRIVER
    if not APP.exists():
        log(f"missing e2e build: {APP}\nrun: GOFLAGS=-tags=e2e MYGO_INSPECTOR=1 bun run build")
        return 2
    try:
        LEGACY.update({str(item["id"]): item for item in json.loads(MANIFEST.read_text(encoding="utf-8"))})
    except Exception:  # noqa: BLE001
        pass

    appdata = canon(tempfile.mkdtemp(prefix="mkb-visual-appdata-"))
    vault = build_vault()
    vault_id = vault_id_of(vault)
    write_provider(vault, f"http://127.0.0.1:{PROVIDER_PORT}")
    write_settings(appdata, "light", vault, vault_id)

    provider = subprocess.Popen(
        [sys.executable, "-I", str(ROOT / "scripts" / "e2e" / "fake-openai.py"), "--port", str(PROVIDER_PORT)],
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
    )
    app = subprocess.Popen(
        [str(APP)],
        env={**os.environ, "APPDATA": appdata, "MYGO_AUTOMATION": f"tcp:127.0.0.1:{PORT}"},
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
    )
    try:
        DRIVER = Driver(PORT)
        DRIVER.connect(40)
        DRIVER.wait("!!document.querySelector('[data-testid=\"overview-grid\"], .welcome-shell')", 40)

        log("== workbench states")
        # 旧版截图在索引就绪后拍；这里等「未索引」徽标消失，避免拍到扫描中状态。
        try:
            DRIVER.wait("!document.body.innerText.includes('未索引')", 30)
        except RuntimeError:
            log("  index still scanning; continuing")
        if "--probe" in sys.argv:
            log(str(DRIVER.js(
                "JSON.stringify({inner: window.innerWidth,"
                " shell: document.querySelector('.overview-shell') ? document.querySelector('.overview-shell').clientWidth : -1,"
                " cols: getComputedStyle(document.querySelector('.overview-grid')).gridTemplateColumns,"
                " container: getComputedStyle(document.querySelector('.overview-shell')).containerType, dirs: (window.__mkbTree.getState().nodes['']||[]).filter((e)=>e.kind==='directory').map((e)=>e.name)})"
            )))
            return 0
        DRIVER.wait("!!document.querySelector('.tree-scroll [role=treeitem], .tree-scroll .tree-row')", 30)
        step("10 overview light", lambda: DRIVER.wait("!!document.querySelector('[data-testid=\"overview-grid\"]')", 20), "10", "overview", theme="light")
        step("11 overview dark", lambda: (DRIVER.js("window.__mkbApp.getState().setTheme('dark')"), time.sleep(0.8)), "11", "overview-dark", theme="dark")
        DRIVER.js("window.__mkbApp.getState().setTheme('light')")
        time.sleep(0.5)
        step(
            "12 doc collapsed",
            lambda: (DRIVER.js("window.__mkbApp.getState().setLayout({ docCollapsed: true })"), time.sleep(0.6)),
            "12", "overview-doc-collapsed",
        )
        DRIVER.js("window.__mkbApp.getState().setLayout({ docCollapsed: false })")
        time.sleep(0.4)
        step(
            "13 tree expanded",
            lambda: (DRIVER.js(
                "(() => { const tree = window.__mkbTree.getState();"
                " ['skills','raws','projects','my-notes'].forEach((p) => { tree.reveal(p + '/'); }); return true; })()"
            ), time.sleep(1.2)),
            "13", "tree-expanded",
        )
        step("16 doc read", lambda: (open_doc("raws/富文本样例.md"), set_mode("阅读")), "16", "doc-read")
        step("17 doc read dark", lambda: (DRIVER.js("window.__mkbApp.getState().setTheme('dark')"), time.sleep(0.8)), "17", "doc-read-dark", theme="dark")
        DRIVER.js("window.__mkbApp.getState().setTheme('light')")
        time.sleep(0.5)
        step("18 doc source", lambda: set_mode("源码"), "18", "doc-source")
        step("19 doc split", lambda: set_mode("分栏"), "19", "doc-split")
        set_mode("阅读")
        step("20 backlinks", lambda: open_doc("raws/共享笔记.md"), "20", "doc-backlinks")
        step("21 provenance", lambda: open_doc("projects/mile-go/react-skill-copy.md"), "21", "doc-provenance")
        step(
            "22 tabs dirty",
            lambda: (
                open_doc("raws/缺标题.md"),
                set_mode("源码"),
                DRIVER.js(
                    "(() => { const s = window.__mkbDocs.getState();"
                    " const path = s.activePath; s.setText(path, (s.docs[path] ? s.docs[path].text : '') + '\\n编辑了一点内容');"
                    " return true; })()"
                ),
                time.sleep(0.5),
            ),
            "22", "doc-tabs-dirty",
        )
        step(
            "23 conflict card",
            lambda: (
                (Path(vault) / "raws" / "缺标题.md").write_text("外部程序改过的内容，与本地编辑冲突。\n", encoding="utf-8"),
                DRIVER.wait("(() => { const d = window.__mkbDocs.getState(); const doc = d.docs[d.activePath]; return !!(doc && doc.externalRevision); })()", 12),
                DRIVER.js("window.__mkbDocs.getState().save(window.__mkbApp.getState().activeVaultId, window.__mkbDocs.getState().activePath)"),
                DRIVER.wait("!!document.querySelector('[aria-label=\"关闭未保存的标签\"], .conflict-card, [data-testid=\"conflict-card\"]')", 10),
            ),
            "23", "doc-conflict-card",
        )
        DRIVER.js("(() => { try { window.__mkbDocs.getState().resolveConflictKeepMine(window.__mkbApp.getState().activeVaultId, window.__mkbDocs.getState().activePath); } catch (e) {} return true; })()")
        time.sleep(0.5)
        step("24 big 1m", lambda: (open_doc("raws/big-1m.md"), time.sleep(0.6)), "24", "doc-big-1m")
        step("25 huge 2m", lambda: (open_doc("raws/huge-2m.md"), time.sleep(0.6)), "25", "doc-huge-2m")
        step(
            "26 search results",
            lambda: (
                open_doc("raws/富文本样例.md"),
                DRIVER.js("window.__mkbSearch.getState().openPanel()"),
                DRIVER.wait("!!document.querySelector('.search-panel')"),
                DRIVER.js(js_set_value('[aria-label="搜索关键词"]', "知识库", "input")),
                DRIVER.wait("document.querySelectorAll('[data-testid=\"search-hit\"], .search-hit').length > 0", 15),
            ),
            "26", "search-results",
        )
        step(
            "27 search empty",
            lambda: (
                DRIVER.js(js_set_value('[aria-label="搜索关键词"]', "zzqqxx987", "input")),
                DRIVER.wait(
                    "(() => { const s = window.__mkbSearch.getState(); return !s.searching && s.text === 'zzqqxx987' && s.hits.length === 0; })()",
                    15,
                ),
                time.sleep(0.3),
            ),
            "27", "search-empty",
        )
        step(
            "28 search filtered",
            lambda: (
                DRIVER.js(js_set_value('[aria-label="搜索关键词"]', "知识", "input")),
                time.sleep(1.2),
                DRIVER.js(
                    "(() => { const chips = [...document.querySelectorAll('.search-chips button')];"
                    " const md = chips.find((c) => /^\\.?md$/i.test(c.textContent.trim()));"
                    " if (md) md.click(); return true; })()"
                ),
                time.sleep(0.6),
            ),
            "28", "search-filtered",
        )
        DRIVER.js("window.__mkbSearch.getState().closePanel()")
        time.sleep(0.3)

        log("== settings states")
        step("47 settings general", lambda: open_settings("general"), "47", "settings-general")
        step("48 settings general dark", lambda: (DRIVER.js("window.__mkbApp.getState().setTheme('dark')"), time.sleep(0.8)), "48", "settings-general-dark", theme="dark")
        DRIVER.js("window.__mkbApp.getState().setTheme('light')")
        time.sleep(0.4)
        step("49 settings model", lambda: open_settings("model"), "49", "settings-model")
        step("51 settings agents", lambda: open_settings("agents"), "51", "settings-agents")
        step("52 settings categories", lambda: open_settings("categories"), "52", "settings-categories")
        step("53 settings about", lambda: open_settings("about"), "53", "settings-about")
        step(
            "54 save bar",
            lambda: (
                open_settings("agents"),
                DRIVER.js(
                    "(() => { const ta = document.querySelector('[data-testid=\"steering-input\"]');"
                    " if (!ta) throw new Error('no steering input');"
                    " const set = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value').set;"
                    " set.call(ta, ta.value + '\\n更简洁一点。'); ta.dispatchEvent(new Event('input', { bubbles: true })); return true; })()"
                ),
                DRIVER.wait("!!document.querySelector('[data-testid=\"settings-savebar\"]')", 8),
            ),
            "54", "settings-savebar",
        )
        DRIVER.js("window.__mkbApp.getState().closeSettings(); true")
        time.sleep(0.4)
        step(
            "55 skill wizard",
            lambda: (
                DRIVER.js("window.__mkbApp.getState().openSkillWizard(); true"),
                DRIVER.wait("!!document.querySelector('[data-testid=\"skill-wizard\"]')", 8),
            ),
            "55", "skill-wizard-step1",
        )
        DRIVER.js("window.__mkbApp.getState().closeSkillWizard(); true")
        time.sleep(0.3)

        log("== chat states")
        step(
            "34 route preview",
            lambda: (
                DRIVER.js("window.__mkbComposer.getState().setText('帮我整理归档这个目录'); true"),
                DRIVER.wait("!!document.querySelector('[data-testid=\"route-target\"]')", 8),
                time.sleep(0.6),
            ),
            "34", "chat-route",
        )
        step(
            "35 assign menu",
            lambda: (
                DRIVER.js(js_click('[data-testid="route-target"]')),
                DRIVER.wait("!!document.querySelector('[role=\"menu\"], .agent-picker')", 6),
                time.sleep(0.4),
            ),
            "35", "chat-assign",
        )
        DRIVER.js("document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); true")
        time.sleep(0.3)
        step(
            "37 streaming",
            lambda: (
                DRIVER.js(
                    "(() => { const c = window.__mkbComposer.getState();"
                    " c.setText('请给我一段长回答，用于截图');"
                    " void c.send(window.__mkbApp.getState().activeVaultId); return true; })()"
                ),
                DRIVER.wait("window.__mkbAgent.getState().buckets[window.__mkbAgent.getState().agent].busy", 20),
                DRIVER.wait("document.querySelectorAll('.chat-answer, [data-testid=\"chat-answer\"]').length > 0", 20),
                time.sleep(0.8),
            ),
            "37", "chat-streaming",
        )
        DRIVER.wait("!window.__mkbAgent.getState().buckets[window.__mkbAgent.getState().agent].busy", 90)
        time.sleep(0.8)
        step("38 chat finished", lambda: time.sleep(0.3), "38", "chat-finished")
        step("39 chat finished dark", lambda: (DRIVER.js("window.__mkbApp.getState().setTheme('dark')"), time.sleep(0.8)), "39", "chat-finished-dark", theme="dark")
        DRIVER.js("window.__mkbApp.getState().setTheme('light')")
        time.sleep(0.4)
        step("43 usage receipt", lambda: time.sleep(0.2), "43", "chat-usage")
        step(
            "44 session info",
            lambda: (
                DRIVER.js(js_click('button[aria-label="会话信息"]')),
                DRIVER.wait("!!document.querySelector('[data-testid=\"session-info\"]')", 8),
                time.sleep(0.4),
            ),
            "44", "chat-session-info",
        )
        DRIVER.js("document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); true")
        time.sleep(0.3)
        step(
            "45 history rail",
            lambda: (
                DRIVER.wait("!!document.querySelector('[data-testid=\"history-rail\"]')", 6),
                time.sleep(0.5),
            ),
            "45", "chat-history",
        )
        step(
            "46 history search",
            lambda: (
                DRIVER.js(js_set_value('[data-testid=\"history-search\"]', "长回答", "input")),
                time.sleep(1.0),
            ),
            "46", "chat-history-search",
        )
        step(
            "42 permission card",
            lambda: (
                DRIVER.js(
                    "(() => { const c = window.__mkbComposer.getState();"
                    " c.setText('请帮我写入一段内容');"
                    " void c.send(window.__mkbApp.getState().activeVaultId); return true; })()"
                ),
                DRIVER.wait(
                    "window.__mkbAgent.getState().buckets[window.__mkbAgent.getState().agent].pendingPermissions.length > 0",
                    30,
                ),
                time.sleep(0.6),
            ),
            "42", "chat-permission",
        )
        DRIVER.js(
            "(() => { const st = window.__mkbAgent.getState(); const b = st.buckets[st.agent];"
            " const card = b.pendingPermissions[0]; if (card) st.reject(card.requestId); return true; })()"
        )
        time.sleep(0.8)

        log("== mutation / audit / health")
        def sidebar_action(path, label):
            DRIVER.js(f"window.__mkbTree.getState().reveal({json.dumps(path)}); true")
            time.sleep(0.8)
            DRIVER.js(
                "(() => { const row = [...document.querySelectorAll('.tree-scroll [role=treeitem]')]"
                f" .find((r) => r.getAttribute('title') === {json.dumps(path)});"
                " if (!row) throw new Error('row not found');"
                f" const btn = [...row.querySelectorAll('button')].find((b) => b.getAttribute('aria-label') === {json.dumps(label)});"
                " if (!btn) throw new Error('action not found'); btn.click(); return true; })()"
            )

        step(
            "29 mutation plan",
            lambda: (
                open_doc("raws/孤立文档.md"),
                sidebar_action("raws/孤立文档.md", "归档"),
                DRIVER.wait("!!document.querySelector('.plan-card')", 10),
                time.sleep(0.4),
            ),
            "29", "mutation-plan",
        )
        step(
            "30 mutation report",
            lambda: (
                DRIVER.js(
                    "(() => { const b = [...document.querySelectorAll('.plan-card button')]"
                    " .find((x) => x.textContent.trim() === '执行');"
                    " if (!b) throw new Error('execute missing'); b.click(); return true; })()"
                ),
                time.sleep(2.0),
                log("  mutation dom: " + str(DRIVER.js(
                    "JSON.stringify({report: !!document.querySelector('.report-card'),"
                    " text: document.body.innerText.includes('操作已完成') || document.body.innerText.includes('操作部分完成') || document.body.innerText.includes('操作失败')})"
                ))),
            ),
            "30", "mutation-report",
        )
        DRIVER.js("(() => { const b = [...document.querySelectorAll('.report-card button')].find((x) => x.textContent.trim() === '知道了'); if (b) b.click(); return true; })()")
        time.sleep(0.3)
        step(
            "31 permanent delete plan",
            lambda: (
                sidebar_action("my-notes/中文文件名测试.md", "删除"),
                DRIVER.wait("!!document.querySelector('.plan-card')", 10),
                time.sleep(0.4),
            ),
            "31", "mutation-delete-plan",
        )
        DRIVER.js("(() => { const b = [...document.querySelectorAll('.plan-card button')].find((x) => x.textContent.trim() === '取消'); if (b) b.click(); return true; })()")
        time.sleep(0.4)
        step("32 audit panel", lambda: (DRIVER.js("window.__mkbApp.getState().toggleAudit(); true"), time.sleep(0.6)), "32", "audit-panel")
        DRIVER.js("window.__mkbApp.getState().toggleAudit(); true")
        time.sleep(0.3)
        step("33 health panel", lambda: (DRIVER.js("window.__mkbApp.getState().openHealth('unresolvedLinks'); true"), time.sleep(0.8)), "33", "health-panel")
        DRIVER.js("window.__mkbApp.getState().closeHealth(); true")
        time.sleep(0.3)

        log("== remaining workbench states")
        step(
            "50 settings model dark",
            lambda: (
                open_settings("model"),
                DRIVER.js("window.__mkbApp.getState().setTheme('dark')"),
                time.sleep(0.8),
            ),
            "50", "settings-model-dark", theme="dark",
        )
        DRIVER.js("window.__mkbApp.getState().setTheme('light')")
        DRIVER.js("window.__mkbApp.getState().closeSettings(); true")
        time.sleep(0.4)
        step(
            "36 run params",
            lambda: (
                DRIVER.js("(() => { const t = document.querySelector('[data-testid=\"runtime-controls-toggle\"]'); if (!t) throw new Error('no toggle'); if (t.getAttribute('aria-expanded') === 'false' || !t.closest('[data-collapsed=false]')) t.click(); return true; })()"),
                time.sleep(0.6),
            ),
            "36", "chat-run-params",
        )
        step(
            "15 inline rename",
            lambda: (
                DRIVER.js("window.__mkbTree.getState().reveal('raws/共享笔记.md'); true"),
                time.sleep(0.8),
                DRIVER.js(
                    "(() => { const row = [...document.querySelectorAll('.tree-scroll [role=treeitem]')]"
                    " .find((r) => r.getAttribute('title') === 'raws/共享笔记.md');"
                    " if (!row) throw new Error('row not found');"
                    " const btn = [...row.querySelectorAll('button')].find((b) => b.getAttribute('aria-label') === '重命名');"
                    " if (!btn) throw new Error('rename missing'); btn.click(); return true; })()"
                ),
                DRIVER.wait("!!document.querySelector('.tree-scroll input')", 6),
                time.sleep(0.3),
            ),
            "15", "tree-inline-rename",
        )
        DRIVER.js("document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); true")
        time.sleep(0.3)
        step(
            "15b inline create",
            lambda: (
                DRIVER.js(js_click('[aria-label="新建文件"]')),
                DRIVER.wait("!!document.querySelector('.tree-row-editing')", 6),
                time.sleep(0.3),
            ),
            "15b", "tree-inline-create",
        )
        DRIVER.js("document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); true")
        time.sleep(0.3)
        step(
            "60 close dirty tab",
            lambda: (
                open_doc("raws/首页.md"),
                set_mode("源码"),
                DRIVER.js(
                    "(() => { const s = window.__mkbDocs.getState(); const p = s.activePath;"
                    " s.setText(p, (s.docs[p] ? s.docs[p].text : '') + String.fromCharCode(10) + '确认卡截图'); return true; })()"
                ),
                time.sleep(0.4),
                DRIVER.js(
                    "(() => { const b = [...document.querySelectorAll('button')]"
                    " .find((x) => (x.getAttribute('aria-label') || '').startsWith('关闭 '));"
                    " if (!b) throw new Error('close tab missing'); b.click(); return true; })()"
                ),
                DRIVER.wait("!!document.querySelector('[aria-label=\"关闭未保存的标签\"]')", 8),
                time.sleep(0.3),
            ),
            "60", "doc-close-confirm",
        )
        DRIVER.js(
            "(() => { const b = [...document.querySelectorAll('button')].find((x) => x.textContent.trim() === '继续编辑'); if (b) b.click(); return true; })()"
        )
        time.sleep(0.3)

        log("== welcome phase")
        app.terminate()
        try:
            app.wait(timeout=8)
        except subprocess.TimeoutExpired:
            pass
        time.sleep(1.0)
        subprocess.run(["taskkill", "/F", "/IM", "Mile Knowledge Base.exe"], capture_output=True)
        time.sleep(0.8)
        welcome_appdata = canon(tempfile.mkdtemp(prefix="mkb-visual-welcome-"))
        app = subprocess.Popen(
            [str(APP)],
            env={**os.environ, "APPDATA": welcome_appdata, "MYGO_AUTOMATION": f"tcp:127.0.0.1:{PORT}"},
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
        )
        DRIVER = Driver(PORT)
        DRIVER.connect(40)
        DRIVER.wait("!!document.querySelector('.welcome-shell')", 40)
        step("00 welcome light", lambda: time.sleep(0.8), "00", "welcome")
        step(
            "03 welcome dark",
            lambda: (DRIVER.js("window.__mkbApp.getState().setTheme('dark')"), time.sleep(0.8)),
            "03", "welcome-dark", theme="dark",
        )
        for entry_id, name, note in [
            ("01", "onboarding-step1", "欢迎页与引导在新版是同一屏（旧版为两步：欢迎页 + 引导覆盖层）"),
            ("02", "onboarding-step2", "非空目录确认需要系统选目录对话框，自动化不可达（T13 e2e 覆盖流程）"),
        ]:
            shot(entry_id, name, note=note, status="skipped")

        log("== no-provider phase")
        app.terminate()
        try:
            app.wait(timeout=8)
        except subprocess.TimeoutExpired:
            pass
        time.sleep(1.0)
        subprocess.run(["taskkill", "/F", "/IM", "Mile Knowledge Base.exe"], capture_output=True)
        time.sleep(0.8)
        provider_file = Path(vault) / ".mile-kb" / "settings" / "providers.json"
        if provider_file.exists():
            provider_file.unlink()
        app = subprocess.Popen(
            [str(APP)],
            env={**os.environ, "APPDATA": appdata, "MYGO_AUTOMATION": f"tcp:127.0.0.1:{PORT}"},
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
        )
        DRIVER = Driver(PORT)
        DRIVER.connect(40)
        time.sleep(2.0)
        # 前面的阶段留下了对话历史，先开新对话，才能拍到空状态的总览。
        DRIVER.js("void window.__mkbAgent.getState().newConversation(); true")
        DRIVER.wait("!!document.querySelector('[data-testid=\"overview-grid\"]')", 40)
        step("40 chat no provider", lambda: time.sleep(0.8), "40", "chat-no-provider")
        step(
            "41 settings model empty",
            lambda: (
                open_settings("model"),
                DRIVER.wait("!!document.querySelector('[data-testid=\"model-service-section\"]')", 10),
                time.sleep(0.6),
            ),
            "41", "settings-model-empty",
        )
        for entry_id, name, note in [
            ("14", "tree-row-hover", "悬停态需要真实指针；automation 只有点击没有移动事件，按 CSS 实现"),
            ("56", "min-size-overview", "automation 协议没有窗口缩放方法，最小尺寸按 DESIGN 和 T13 人工确认"),
            ("57", "min-size-read", "同上：最小尺寸下的阅读模式需人工确认"),
            ("58", "close-prompt", "原生询问框无法截取（旧版同样未截）"),
            ("59", "folder-dialog", "系统选目录对话框无法截取（旧版同样未截）"),
        ]:
            shot(entry_id, name, note=note, status="skipped")

        log("== write outputs")
        manifest = [ENTRIES[key] for key in sorted(ENTRIES.keys(), key=lambda k: (len(k), k))]
        OUT.mkdir(parents=True, exist_ok=True)
        (OUT / "manifest.json").write_text(
            json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
        )
        write_compare_page(manifest)
        captured = sum(1 for item in manifest if item["status"] == "captured")
        failed = sum(1 for item in manifest if item["status"] == "failed")
        log(f"done: {captured} captured, {failed} failed")
        return 0
    finally:
        try:
            app.terminate()
        except Exception:  # noqa: BLE001
            pass
        provider.terminate()
        subprocess.run(["taskkill", "/F", "/IM", "Mile Knowledge Base.exe"], capture_output=True)


def write_compare_page(manifest):
    rows = []
    for item in sorted(manifest, key=lambda e: (len(str(e["id"])), str(e["id"]))):
        legacy = LEGACY.get(item["id"], {})
        legacy_file = legacy.get("file", "")
        old = f"../../baseline/{legacy_file}" if legacy_file else ""
        new = f"{item['file']}" if item.get("file") else ""
        rows.append(
            "<tr><td>{id}</td><td>{title}</td>"
            '<td>{old}</td><td>{new}</td><td>{note}</td></tr>'.format(
                id=item["id"],
                title=(item.get("title") or "").replace("<", "&lt;"),
                old=f'<img src="{old}">' if old else "—",
                new=f'<img src="{new}">' if new else "—",
                note=(item.get("note") or "").replace("<", "&lt;"),
            )
        )
    html = """<!doctype html>
<html lang="zh-CN"><meta charset="utf-8">
<title>M3 视觉对照</title>
<style>
body{font:13px/1.5 system-ui,"Microsoft YaHei",sans-serif;margin:16px;background:#f6f7f9;color:#111}
table{border-collapse:collapse;width:100%%}
td,th{border:1px solid #d5d8de;padding:6px;vertical-align:top;background:#fff}
img{max-width:100%%;display:block}
td:nth-child(3),td:nth-child(4){width:38%%}
</style>
<h1>M3 视觉对照（左：旧版基线，右：新版）</h1>
<p>只作本地对照，不上传。生成脚本：scripts/visual/capture-new.py。</p>
<table>
<tr><th>编号</th><th>标题</th><th>旧版</th><th>新版</th><th>备注</th></tr>
%s
</table>
</html>
""" % "\n".join(rows)
    (OUT.parent / "index.html").write_text(html, encoding="utf-8", newline="\n")


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception:  # noqa: BLE001
        traceback.print_exc()
        sys.exit(1)
