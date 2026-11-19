/**
 * 旧版性能基线（M3 T12）。只读运行旧版 out/main，appData/userData 指向
 * os.tmpdir() 下新建的沙箱；不修改旧版仓库，不碰真实 %APPDATA%。
 *
 * 由 scripts/perf-legacy/bench.sh 调用：
 *   electron.exe bench.cjs
 * 环境变量：PERF_APPDATA（沙箱根）、PERF_OUT（结果 JSON）。
 */

const { mkdtempSync, mkdirSync, writeFileSync } = require('node:fs')
const { tmpdir } = require('node:os')
const { join, resolve } = require('node:path')

const t0 = Date.now()
const OUT = process.env.PERF_OUT
const sandbox = process.env.PERF_APPDATA || mkdtempSync(join(tmpdir(), 'mile-perf-legacy-'))

const { app } = require('electron')
app.setPath('appData', sandbox)
app.setPath('userData', join(sandbox, 'mile-knowledge-base'))
mkdirSync(app.getPath('userData'), { recursive: true })

const LEGACY = resolve('E:/WebApp/Mile-Project/Mile-Knowledge-Base-master/out/main/index.js')
require(LEGACY)

const QUERIES = ['知识', '知识库', 'React', '状态', '项目', '工具', '约束', '剪藏', '首页', '共享', '文档', '测试']

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

async function mainWindow() {
  for (let i = 0; i < 200; i++) {
    const wins = require('electron').BrowserWindow.getAllWindows().filter((w) => !w.isDestroyed())
    if (wins.length) return wins[0]
    await sleep(100)
  }
  throw new Error('no window')
}

async function waitFor(win, script, timeoutMs) {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    const ok = await win.webContents.executeJavaScript(`(() => { try { return !!(${script}) } catch { return false } })()`)
    if (ok) return
    await sleep(50)
  }
  throw new Error('wait timed out: ' + script)
}

async function search(win, query) {
  return win.webContents.executeJavaScript(
    `new Promise((resolve) => {
      const s = window.__mileSearchStore
      if (!s) return resolve({ ms: -1, hits: 0 })
      const q = ${JSON.stringify(query)}
      const t = performance.now()
      s.getState().setText(q)
      const iv = setInterval(() => {
        const st = s.getState()
        if (!st.searching && st.text === q) {
          clearInterval(iv)
          resolve({ ms: performance.now() - t, hits: st.hits.length })
        }
      }, 10)
      setTimeout(() => { clearInterval(iv); resolve({ ms: -1, hits: 0 }) }, 10000)
    })`
  )
}

app.whenReady().then(async () => {
  const win = await mainWindow()
  await waitFor(win, "document.querySelector('.tree-row, [role=treeitem]')", 30000)
  const readyMs = Date.now() - t0
  await waitFor(win, 'window.__mileSearchStore', 15000)
  await win.webContents.executeJavaScript("window.__mileSearchStore.getState().openPanel()")
  await sleep(500)
  // 预热一次，避免把首次索引等待算进 p95。
  await search(win, '知识')
  const searches = []
  for (const query of QUERIES) {
    searches.push({ query, ...(await search(win, query)) })
    await sleep(120)
  }
  const ok = searches.filter((item) => item.ms >= 0).map((item) => item.ms).sort((a, b) => a - b)
  const p95 = ok.length ? ok[Math.min(ok.length - 1, Math.floor(ok.length * 0.95))] : -1
  writeFileSync(
    OUT,
    `${JSON.stringify({ coldStartMs: readyMs, samples: ok.length, searchP95Ms: p95, searches }, null, 2)}\n`,
    'utf8'
  )
  app.exit(0)
}).catch((error) => {
  if (OUT) writeFileSync(OUT, `${JSON.stringify({ error: String(error) })}\n`, 'utf8')
  app.exit(1)
})
