// R4 spike page: 40px title bar, 50 history messages, streaming answer
// rendered incrementally (only the last block changes) vs a full-repaint
// counterexample, plus input-latency and longtask measurement.
//
// The page reports everything to Go via SpikeService.Result, which lands
// in %TEMP%/r4-spike/spike-log.jsonl.
import { call, Channel } from "mygo-runtime";
import { SpikeService, type StreamFrame } from "./mygo";

const $ = <T extends HTMLElement>(sel: string) => document.querySelector<T>(sel)!;
const results: Record<string, unknown> = {};
const report = (key: string, value: unknown) => {
  results[key] = value;
  void SpikeService.result({ key, value });
};

declare global {
  interface Window {
    __spike: typeof results & {
      runStream?: (mode: string, render: string) => Promise<unknown>;
      pressKeys?: (n: number) => Promise<unknown>;
      snapshot?: () => unknown;
    };
    __bootT0: number;
  }
}

// ---- measurement helpers -------------------------------------------------

interface PerfStats {
  frames: number;
  dropped: number; // frame gaps > 25ms
  p50: number;
  p95: number;
  max: number;
  longtasks: number;
  longestTask: number;
  inputP50: number;
  inputP90: number;
  inputMax: number;
  msgs: number;
  ms: number;
}

class Perf {
  gaps: number[] = [];
  longtasks: number[] = [];
  inputs: number[] = [];
  private rafPrev = 0;
  private running = false;
  private po?: PerformanceObserver;

  start() {
    this.gaps = [];
    this.longtasks = [];
    this.inputs = [];
    this.running = true;
    this.rafPrev = 0;
    const loop = (t: number) => {
      if (!this.running) return;
      if (this.rafPrev) this.gaps.push(t - this.rafPrev);
      this.rafPrev = t;
      requestAnimationFrame(loop);
    };
    requestAnimationFrame(loop);
    try {
      this.po = new PerformanceObserver((list) => {
        for (const e of list.getEntries()) this.longtasks.push(e.duration);
      });
      this.po.observe({ type: "longtask", buffered: true });
    } catch {
      /* longtask unsupported */
    }
  }

  recordInput(ms: number) {
    if (this.running) this.inputs.push(ms);
  }

  stop(msgs: number, ms: number): PerfStats {
    this.running = false;
    this.po?.disconnect();
    const pct = (a: number[], p: number) =>
      a.length ? a.sort((x, y) => x - y)[Math.min(a.length - 1, Math.floor((p / 100) * a.length))]! : 0;
    return {
      frames: this.gaps.length,
      dropped: this.gaps.filter((g) => g > 25).length,
      p50: +pct(this.gaps.slice(), 50).toFixed(2),
      p95: +pct(this.gaps.slice(), 95).toFixed(2),
      max: +Math.max(0, ...this.gaps).toFixed(2),
      longtasks: this.longtasks.filter((t) => t > 50).length,
      longestTask: +Math.max(0, ...this.longtasks).toFixed(2),
      inputP50: +pct(this.inputs.slice(), 50).toFixed(2),
      inputP90: +pct(this.inputs.slice(), 90).toFixed(2),
      inputMax: +Math.max(0, ...this.inputs).toFixed(2),
      msgs,
      ms: +ms.toFixed(0),
    };
  }
}

const perf = new Perf();

// ---- DOM -----------------------------------------------------------------

document.querySelector("#app")!.innerHTML = `
  <div id="titlebar">
    <span class="app-name">R4 Spike</span>
    <span class="vault-tag">VAULT</span>
    <span class="spacer"></span>
    <button id="btn-min" title="Minimize">&#8211;</button>
    <button id="btn-max" title="Maximize">&#9633;</button>
    <button id="btn-close" title="Close">&#10005;</button>
  </div>
  <div id="statusbar">
    <span id="sb-boot">boot…</span>
    <span id="sb-win"></span>
    <button id="bt-run-frame16">stream frame16 (incr)</button>
    <button id="bt-run-token">stream per-token (incr)</button>
    <button id="bt-run-full">stream frame16 (full repaint)</button>
    <button id="bt-keys">press 60 keys</button>
    <button id="bt-dialog">dialog probe</button>
    <button id="bt-hide">hide to tray</button>
  </div>
  <div id="chat"></div>
  <div id="composer"><input id="inp" placeholder="type during streaming" autocomplete="off" /></div>
  <div id="metrics"></div>
`;

const chat = $("#chat") as HTMLDivElement;
const inp = $("#inp") as HTMLInputElement;

// history: 50 messages, ~2000 chars of HTML each
const para = (seed: number, i: number) =>
  `<p>${"history paragraph ".repeat(3)}${seed}-${i} ${"lorem ipsum dolor sit amet ".repeat(9)}</p>`;
for (let m = 0; m < 50; m++) {
  const el = document.createElement("div");
  el.className = m % 2 ? "msg agent" : "msg user";
  let html = `<div class="meta">msg ${m + 1} · ${m % 2 ? "agent" : "user"}</div><div class="body">`;
  for (let p = 0; p < 4; p++) html += para(m, p);
  el.innerHTML = html + "</div>";
  chat.appendChild(el);
}
chat.scrollTop = chat.scrollHeight;

// live streaming message: sealed blocks + one open block. A block seals at
// a paragraph boundary; sealed nodes are never touched again.
let openP: HTMLElement | null = null;
let liveBody: HTMLElement | null = null;
let liveEl: HTMLElement | null = null;
let fullText = ""; // for the full-repaint counterexample

function newLiveMessage(): HTMLElement {
  const el = document.createElement("div");
  el.className = "msg agent";
  el.innerHTML = `<div class="meta">live · agent · streaming</div><div class="body"></div>`;
  liveBody = el.querySelector(".body");
  openP = document.createElement("p");
  liveBody!.appendChild(openP);
  chat.appendChild(el);
  chat.scrollTop = chat.scrollHeight;
  liveEl = el;
  return el;
}

// apply a frame incrementally: the frame's text is appended; when it
// contains a paragraph break the sealed part becomes its own node.
function applyIncr(html: string) {
  const parts = html.split("</p><p>");
  parts.forEach((part, i) => {
    const last = i === parts.length - 1;
    const text = last ? part : part + "</p>";
    if (openP) {
      // extend the open paragraph's text; innerHTML only on the open node
      openP.innerHTML += text.replace(/^<p>/, "");
    }
    if (!last) {
      // seal: the just-closed node is never touched again
      openP = document.createElement("p");
      liveBody!.appendChild(openP);
    }
  });
  chat.scrollTop = chat.scrollHeight;
}

function applyFull(html: string) {
  fullText += html;
  // counterexample: re-render the entire message every frame (P-3)
  const fixed = fullText.replaceAll("</p><p>", "</p><p>");
  liveBody!.innerHTML = `<p>${fixed.replace(/^<p>|<\/p>$/g, "")}</p>`;
  chat.scrollTop = chat.scrollHeight;
}

// ---- input latency: dispatch a key, measure to paint ---------------------

async function pressKeys(n: number) {
  const lat: number[] = [];
  for (let i = 0; i < n; i++) {
    const t0 = performance.now();
    inp.value += "x";
    inp.dispatchEvent(new InputEvent("input", { bubbles: true, inputType: "insertText", data: "x" }));
    await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
    const ms = performance.now() - t0;
    lat.push(ms);
    perf.recordInput(ms);
    await new Promise((r) => setTimeout(r, 16));
  }
  lat.sort((a, b) => a - b);
  return {
    n,
    p50: +lat[Math.floor(lat.length * 0.5)]!.toFixed(2),
    p90: +lat[Math.floor(lat.length * 0.9)]!.toFixed(2),
    max: +lat[lat.length - 1]!.toFixed(2),
  };
}

// ---- streaming runs --------------------------------------------------------

let streaming = false;

async function runStream(mode: string, render: string) {
  if (streaming) return { skipped: true };
  streaming = true;
  newLiveMessage();
  fullText = "";
  perf.start();
  const t0 = performance.now();
  let frames = 0;
  const ch = new Channel<StreamFrame>((f) => {
    frames++;
    if (render === "full") applyFull(f.text);
    else applyIncr(f.text);
  });
  const callP = SpikeService.stream(
    { mode, interval: 0, chunk: 24, total: 10000 },
    ch,
  ).catch((e) => ({ error: String(e) }));
  // consume via onmessage; wait for the call to finish
  await callP;
  const ms = performance.now() - t0;
  const stats = perf.stop(frames, ms);
  streaming = false;
  return { mode, render, frames, stats };
}

async function runAndReport(mode: string, render: string) {
  const r = (await runStream(mode, render)) as Record<string, unknown>;
  report(`stream.${mode}.${render}`, r);
  const m = $("#metrics");
  const s = r["stats"] as PerfStats;
  m.textContent =
    `${mode}/${render}\n` +
    `frames ${s.frames} longtasks ${s.longtasks} longest ${s.longestTask}ms\n` +
    `gap p50 ${s.p50} p95 ${s.p95} max ${s.max}ms\n` +
    `input p50 ${s.inputP50} p90 ${s.inputP90} max ${s.inputMax}ms`;
  return r;
}

// ---- wiring ----------------------------------------------------------------

$("#btn-min").addEventListener("click", () => void SpikeService.winOp("minimize"));
$("#btn-max").addEventListener("click", () => void SpikeService.winOp("toggleMaximize"));
$("#btn-close").addEventListener("click", () => void SpikeService.winOp("close"));
$("#bt-run-frame16").addEventListener("click", () => void runAndReport("frame16", "incr"));
$("#bt-run-token").addEventListener("click", () => void runAndReport("token", "incr"));
$("#bt-run-full").addEventListener("click", () => void runAndReport("frame16", "full"));
$("#bt-keys").addEventListener("click", async () => {
  const r = await pressKeys(60);
  report("keys", r);
});
$("#bt-dialog").addEventListener("click", async () => {
  const r = await SpikeService.dialogProbe();
  report("dialog-probe", r);
});
$("#bt-hide").addEventListener("click", () => void SpikeService.winOp("hideToTray"));

// input latency while user actually types: time from keydown to next paint
inp.addEventListener("keydown", () => {
  const t0 = performance.now();
  requestAnimationFrame(() => requestAnimationFrame(() => perf.recordInput(performance.now() - t0)));
});

// ---- boot ------------------------------------------------------------------

async function refreshWin() {
  const s = await SpikeService.winState();
  $("#sb-win").textContent =
    `win ${s.boundsW}x${s.boundsH} @${s.boundsX},${s.boundsY} ` +
    `${s.maximized ? "max" : ""}${s.minimized ? "min" : ""}`;
  return s;
}

const boot = await SpikeService.ready();
const dclMs = +(performance.now()).toFixed(0);
$("#sb-boot").textContent =
  `boot ${boot.bootMs}ms · page-ready ${dclMs}ms · tray ${boot.trayOk ? "ok" : "FAIL"}`;
report("boot", {
  goBootMs: boot.bootMs,
  pageReadyMs: +dclMs,
  dclRelativeToScript: +(performance.now() - window.__bootT0).toFixed(0),
  trayOk: boot.trayOk,
});
await refreshWin();

// surface for automation / manual eval
window.__spike = Object.assign(results, {
  runStream,
  pressKeys,
  refreshWin,
  snapshot: () => results,
});
void refreshWin();
setInterval(() => void refreshWin(), 1000);
