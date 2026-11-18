/**
 * 旧版 Mile Knowledge Base（Electron 39 + React 19, v2.7.0）沙箱截图脚本。
 *
 * 用途：为 Go 重写版产出 UI 基线设计稿。只运行旧版 `out/` 构建产物，
 * 绝不修改旧版目录；appData/userData 指向 os.tmpdir() 下新建的临时沙箱，
 * 与真实 %APPDATA%\mile-knowledge-base 完全隔离。
 *
 * 用法（在新项目根执行）：
 *   "<旧版>/node_modules/electron/dist/electron.exe" design/baseline/capture.cjs --phase=<phase>
 *
 * phase：
 *   welcome     无已登记知识库：欢迎页 + 新手引导两步（系统目录对话框被桩掉）
 *   noprovider  有库、未配模型服务：对话不可用状态 + 设置-模型服务空状态
 *   workbench   有库 + 假 OpenAI 供应商：其余全部截图
 *
 * 输出：
 *   design/baseline/shots/<编号>-<名称>-<light|dark>-<WxH>.png
 *   design/baseline/manifest.json
 *
 * 退出：app.exit(0/1)。不用 app.quit / 关窗（旧版关窗会弹原生询问框）。
 * 每个截图都有 DOM 等待条件；单步失败记 failed 并继续，不中断整轮。
 */

const {
  existsSync,
  mkdirSync,
  mkdtempSync,
  readdirSync,
  realpathSync,
  rmSync,
  writeFileSync
} = require('node:fs')
const { tmpdir } = require('node:os')
const { join, resolve } = require('node:path')
const { createHash, randomBytes } = require('node:crypto')
const { createServer } = require('node:http')

// ---------------------------------------------------------------------------
// 参数与路径
// ---------------------------------------------------------------------------

const PHASE = (() => {
  const arg = process.argv.find((a) => a.startsWith('--phase='))
  return arg ? arg.slice('--phase='.length) : 'workbench'
})()

const NEW_ROOT = resolve(__dirname, '..', '..') // Mile-Knowledge-Base-Go
const OUT_DIR = join(__dirname, 'shots')
const MANIFEST = join(__dirname, 'manifest.json')
mkdirSync(OUT_DIR, { recursive: true })

// ---------------------------------------------------------------------------
// 沙箱（必须在 require electron app 之前建好路径，再 require 主包）
// ---------------------------------------------------------------------------

const { app, BrowserWindow, dialog } = require('electron')

const sandbox = mkdtempSync(join(tmpdir(), `mile-baseline-${PHASE}-`))
app.setPath('appData', sandbox)
app.setPath('userData', join(sandbox, 'mile-knowledge-base'))
mkdirSync(app.getPath('userData'), { recursive: true })

// ---------------------------------------------------------------------------
// 小工具
// ---------------------------------------------------------------------------

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
const canon = (p) => realpathSync.native(p)

// manifest 跨 phase 累积：启动时载入已有项，本次记录按 id 覆盖/追加。
let manifest = []
try {
  if (existsSync(MANIFEST)) {
    const prev = JSON.parse(require('node:fs').readFileSync(MANIFEST, 'utf8'))
    if (Array.isArray(prev)) manifest = prev
  }
} catch { manifest = [] }

function record(entry) {
  const i = manifest.findIndex((e) => e.id === entry.id)
  if (i >= 0) manifest[i] = entry
  else manifest.push(entry)
  manifest.sort((a, b) => String(a.id).localeCompare(String(b.id), 'en', { numeric: true }))
  writeFileSync(MANIFEST, `${JSON.stringify(manifest, null, 2)}\n`, 'utf8')
}

function shotFile(id, name, theme, w, h) {
  return `${String(id).padStart(2, '0')}-${name}-${theme}-${w}x${h}.png`
}

const js = async (win, source) => {
  try {
    return await win.webContents.executeJavaScript(source)
  } catch (error) {
    const detail = error && error.message ? error.message : String(error)
    throw new Error(`executeJavaScript 失败: ${detail}\n--- 源码 ---\n${source.slice(0, 300)}`)
  }
}

function waitFor(predicateSource, timeoutMs, win) {
  const started = Date.now()
  return new Promise((resolvePromise, reject) => {
    const tick = async () => {
      try {
        if (await js(win, predicateSource)) return resolvePromise()
      } catch {
        /* 页面未就绪 */
      }
      if (Date.now() - started > timeoutMs)
        return reject(new Error(`超时: ${predicateSource.slice(0, 140)}`))
      setTimeout(tick, 130)
    }
    void tick()
  })
}

const click = (win, selector) =>
  js(
    win,
    `(() => { const el = document.querySelector(${JSON.stringify(selector)}); if (!el) throw new Error('找不到 ${selector}'); el.click(); return true })()`
  )

const exists = (testid) => `!!document.querySelector('[data-testid="${testid}"]')`

/** 受控 input/textarea 赋值。 */
const setInput = (win, testid, value) =>
  js(
    win,
    `(() => {
      const el = document.querySelector('[data-testid="${testid}"]')
      if (!el) throw new Error('找不到 ${testid}')
      const proto = el.tagName === 'TEXTAREA' ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype
      Object.getOwnPropertyDescriptor(proto, 'value').set.call(el, ${JSON.stringify(value)})
      el.dispatchEvent(new Event('input', { bubbles: true }))
      el.dispatchEvent(new Event('change', { bubbles: true }))
      return true
    })()`
  )

/**
 * 截一张图。等待 wait 条件满足 -> 再静 settle ms -> capturePage 存 PNG。
 * 任何一步失败都把 status 标 failed 并写原因，继续后续截图。
 */
async function shot(win, { id, title, name, prdRefs = [], theme = 'light', size = '1440x900', wait = 'true', settle = 350, before = null, note = '' }) {
  const [w, h] = size.split('x').map(Number)
  const file = shotFile(id, name, theme, w, h)
  const full = join(OUT_DIR, file)
  const entry = { id: String(id).padStart(2, '0'), file: `shots/${file}`, title, prdRefs, phase: PHASE, theme, size, status: 'captured', note }
  try {
    if (typeof before === 'function') await before()
    if (wait !== 'true') await waitFor(wait, 20000, win)
    await sleep(settle)
    const image = await win.webContents.capturePage()
    writeFileSync(full, image.toPNG())
    entry.note = (note ? `${note} · ` : '') + '待像素校验'
    console.log(`  [shot] ${file}`)
  } catch (error) {
    entry.status = 'failed'
    entry.note = `${note ? `${note} · ` : ''}${error && error.message ? error.message : String(error)}`
    console.error(`  [FAIL] ${file}: ${entry.note}`)
  }
  record(entry)
}

/** 切主题并等 data-theme 生效。 */
async function setTheme(win, mode) {
  await js(win, `window.__mileSettingsStore.getState().setTheme('${mode}')`)
  await waitFor(`document.documentElement.dataset.theme === '${mode}'`, 6000, win)
  await sleep(350)
}

async function setSize(win, w, h) {
  await win.setContentSize(w, h)
  await sleep(320)
}

// ---------------------------------------------------------------------------
// 假 OpenAI 供应商（与 scripts/fixtures/fake-openai-provider.cjs 同口径，内联
// 一份精简版避免跨目录 require；触发词：STREAM_MD / PERMISSION_TEST / USAGE）
// ---------------------------------------------------------------------------

const FAKE_MODEL = 'fake-model-kb'

function startFakeProvider() {
  const requests = []
  const sse = (p) => `data: ${JSON.stringify(p)}\n\n`
  const delta = (d, finish = null) => ({ choices: [{ index: 0, delta: d, finish_reason: finish }] })
  const textOf = (m) => (typeof m?.content === 'string' ? m.content : '')
  const userText = (messages) => {
    for (let i = messages.length - 1; i >= 0; i -= 1)
      if (messages[i].role === 'user') return textOf(messages[i]).split('\n\n')[0]
    return ''
  }
  const lastTool = (messages) => {
    const last = messages[messages.length - 1]
    if (!last || last.role !== 'tool') return null
    return { output: textOf(last) }
  }
  const server = createServer((req, res) => {
    const chunks = []
    req.on('data', (c) => chunks.push(c))
    req.on('end', async () => {
      if (req.method === 'GET' && /\/models$/.test(req.url ?? '')) {
        res.writeHead(200, { 'content-type': 'application/json' })
        res.end(JSON.stringify({ data: [{ id: FAKE_MODEL }] }))
        return
      }
      if (req.method !== 'POST' || !/\/chat\/completions$/.test(req.url ?? '')) {
        res.writeHead(404); res.end(); return
      }
      let body = {}
      try { body = JSON.parse(Buffer.concat(chunks).toString('utf8')) } catch { /* 容忍 */ }
      requests.push(body)
      const messages = Array.isArray(body.messages) ? body.messages : []
      let closed = false
      res.on('close', () => { closed = true })
      const send = (p) => { if (!closed) res.write(sse(p)) }
      res.writeHead(200, { 'content-type': 'text/event-stream' })
      const text = userText(messages)
      const tool = lastTool(messages)
      if (tool) {
        send(delta({ content: '写入完成。' })); send(delta({}, 'stop'))
      } else if (text.includes('PERMISSION_TEST')) {
        send(delta({ tool_calls: [{ index: 0, id: 'call_1', type: 'function', function: { name: 'write_note', arguments: JSON.stringify({ path: 'raws/permission-test.md', content: '审批测试' }) } }] }))
        send(delta({}, 'tool_calls'))
      } else if (text.startsWith('USAGE')) {
        send(delta({ content: 'USAGE_SENT' })); send(delta({}, 'stop'))
        send({ choices: [], usage: { prompt_tokens: 1000, completion_tokens: 400, total_tokens: 1400, prompt_cache_hit_tokens: 800, prompt_cache_miss_tokens: 200, completion_tokens_details: { reasoning_tokens: 100 } } })
      } else if (text.startsWith('STREAM_MD')) {
        for (const piece of ['用户', '要求', '一段 Markdown', '，直接给出即可。']) send(delta({ reasoning_content: piece }))
        for (const piece of ['## 能', '力\n', '- 阅读', '代码\n', '- 修改代码\n', '- 检索', '知识\n\n', '我用 *', '*加粗*', '* 与 `', 'code', '`。\n\n| 列 A | 列 B |\n| --- | --- |\n| 1 | 2 |']) send(delta({ content: piece }))
        send(delta({}, 'stop'))
      } else {
        send(delta({ content: '好的，' }))
        await sleep(400)
        send(delta({ content: '收到。' }))
        send(delta({}, 'stop'))
      }
      if (!closed) res.end('data: [DONE]\n\n')
    })
  })
  return new Promise((resolvePromise) => {
    server.listen(0, '127.0.0.1', () => {
      resolvePromise({
        baseUrl: `http://127.0.0.1:${server.address().port}/v1`,
        requests,
        close: () => new Promise((done) => server.close(() => done()))
      })
    })
  })
}

function seedProvider(vaultRoot, baseUrl) {
  const dir = join(vaultRoot, '.mile-kb', 'settings')
  mkdirSync(dir, { recursive: true })
  writeFileSync(
    join(dir, 'providers.json'),
    `${JSON.stringify({ version: 1, providers: [{ name: 'fakeprovider', kind: 'openai', baseUrl, models: [FAKE_MODEL], defaultModel: FAKE_MODEL, apiKeyEnv: '', contextWindow: 64000 }] }, null, 2)}\n`,
    'utf8'
  )
}

// ---------------------------------------------------------------------------
// 样本知识库（workbench / noprovider 用）
// ---------------------------------------------------------------------------

const BIG_LINE = '这是一行用于填充体积的中文与 english mixed 内容，重复排列以制造大文件。\n'

function buildVault() {
  const vaultRoot = canon(mkdtempSync(join(tmpdir(), 'mile-baseline-vault-')))
  const w = (rel, text) => {
    const full = join(vaultRoot, ...rel.split('/'))
    mkdirSync(join(full, '..'), { recursive: true })
    writeFileSync(full, text, 'utf8')
  }

  // 功能目录
  for (const dir of [
    'skills/react', 'skills/web-vue', 'skills/nodejs', 'skills/harmonyos',
    'raws', 'Clippings', '_archives', 'projects/campus-hub', 'projects/mile-go',
    'tools', 'AI Knowledge base', 'dev-constraints', '.claude', '.obsidian', 'my-notes'
  ]) mkdirSync(join(vaultRoot, ...dir.split('/')), { recursive: true })

  // skills 内容
  w('skills/react/SKILL.md', '---\ntitle: React 技巧\ntags: [frontend, react]\n---\n\n# React\n\n受控组件与状态提升。参见 [[共享笔记]]。\n')
  w('skills/web-vue/SKILL.md', '# Vue\n\n组合式 API。\n')
  w('skills/nodejs/SKILL.md', '# Node\n\n事件循环。\n')
  w('skills/harmonyos/SKILL.md', '# 鸿蒙\n\nArkTS。\n')
  w('skills/react/deep/nested/note.md', '# 深层笔记\n\n嵌套目录内容。\n')

  // raws 富文本文档：表格 / 任务 / 代码 / 引用 / 图片 / wikilink 三态
  // 生成一张小 PNG（1x1 红色像素，合法 PNG）
  const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==', 'base64')
  mkdirSync(join(vaultRoot, 'raws', 'attachments'), { recursive: true })
  writeFileSync(join(vaultRoot, 'raws', 'attachments', 'img-red.png'), png)

  w('raws/富文本样例.md', [
    '---', 'title: 富文本样例', 'tags: demo, 排版', '---', '',
    '# 富文本样例', '',
    '一段包含 **加粗**、`code` 与 ~~删除线~~ 的正文。', '',
    '| 名称 | 数量 | 备注 |', '| --- | --- | --- |', '| 苹果 | 3 | 新鲜 |', '| 梨 | 5 | 一般 |', '',
    '- [x] 已完成任务', '- [ ] 待办任务', '',
    '```js', 'const x = 1', 'console.log(x)', '```', '',
    '> 这是一段引用文字。', '',
    '库内图片：', '![红](attachments/img-red.png)', '',
    '链接：[[共享笔记]]（已解析）、[[不存在的页面]]（未创建）、[[同名文档]]（多个同名）。',
    ''
  ].join('\n'))

  w('raws/共享笔记.md', '# 共享笔记\n\n被 [[富文本样例]] 与 [[首页]] 引用的目标。\n\n回链到 [[首页]]。\n')
  w('raws/首页.md', '# 首页\n\n引用 [[共享笔记]] 与 [[富文本样例]]。\n')
  w('raws/孤立文档.md', '# 孤立文档\n\n没有任何入链与出链。\n')
  w('raws/缺标题.md', '直接正文，没有标题行，标题回退为文件名。\n')
  // 同名不同目录
  w('raws/同名文档.md', '# 同名 A\n\nraws 下的同名文档。\n')
  w('my-notes/同名文档.md', '# 同名 B\n\nmy-notes 下的同名文档。\n')
  // 中文文件名 + 长文件名
  w('my-notes/中文文件名测试.md', '# 中文文件名\n\n用于检索与显示。\n')
  w('my-notes/a-very-long-file-name-that-should-not-break-the-grid-layout-at-all.md', '# 长文件名\n\n很长。\n')
  w('Clippings/剪藏一篇.md', '# 剪藏\n\n来自网页的剪藏内容，含「知识库」关键词。\n')
  w('AI Knowledge base/独立知识.md', '# 独立知识\n\nwiki 区文档。\n')
  w('dev-constraints/约束.md', '# 约束\n\n开发约束。\n')
  w('tools/工具说明.md', '# 工具\n\n工具说明。\n')
  w('projects/campus-hub/说明.md', '# 项目\n\n项目说明。\n')

  // 来源追踪：projects/ 下副本 + skills/ 下来源，baseHash 用第三份内容 → both-changed
  const copyPath = 'projects/mile-go/react-skill-copy.md'
  const sourcePath = 'skills/react/SKILL.md'
  w(copyPath, '# React（本地定制版）\n\n副本已改。\n')
  // sourcePath 已在上方写过（与 baseHash 不同即可）。记录 baseHash = 第三份内容的 hash。
  const baseHash = createHash('sha256').update('# React\n\n受控组件与状态提升。\n', 'utf8').digest('hex')
  mkdirSync(join(vaultRoot, '.mile-kb'), { recursive: true })
  writeFileSync(
    join(vaultRoot, '.mile-kb', 'provenance.json'),
    `${JSON.stringify({ version: 1, records: [{ copyPath, sourcePath, baseRevision: 1, baseHash, copiedAt: new Date().toISOString() }] }, null, 2)}\n`,
    'utf8'
  )

  // >1 MiB（阅读禁用，可源码）与 >2 MiB（只读）
  const oneMiB = BIG_LINE.repeat(Math.ceil((1.2 * 1024 * 1024) / Buffer.byteLength(BIG_LINE)))
  w('raws/big-1m.md', '# 大文件 1MiB+\n\n' + oneMiB)
  const twoMiB = BIG_LINE.repeat(Math.ceil((2.2 * 1024 * 1024) / Buffer.byteLength(BIG_LINE)))
  w('raws/huge-2m.md', '# 大文件 2MiB+\n\n' + twoMiB)

  return vaultRoot
}

function vaultIdOf(vaultRoot) {
  return createHash('sha256').update(vaultRoot.toLowerCase()).digest('hex').slice(0, 16)
}

function writeSettings(theme, layout, recentVaults, lastVaultId) {
  writeFileSync(
    join(app.getPath('userData'), 'settings.json'),
    JSON.stringify({ theme, layout, recentVaults, lastVaultId }),
    'utf8'
  )
}

const seedLayout = (overrides = {}) => ({
  sidebarWidth: 232, docWidth: 560, docCollapsed: false, sidebarCollapsed: false, ...overrides
})

// ---------------------------------------------------------------------------
// 驱动辅助
// ---------------------------------------------------------------------------

async function getWin() {
  // 主进程在 whenReady 之后才异步创建窗口，这里轮询等待而不是立即取。
  const deadline = Date.now() + 20000
  let win = null
  while (Date.now() < deadline) {
    win = BrowserWindow.getAllWindows()[0]
    if (win) break
    await sleep(250)
  }
  if (!win) throw new Error('主进程没有创建窗口')
  await new Promise((resolvePromise) => {
    if (!win.webContents.isLoading()) return resolvePromise()
    win.webContents.once('did-finish-load', () => resolvePromise())
  })
  return win
}

/** 通过统一输入框发消息。 */
async function sendMessage(win, text) {
  await js(
    win,
    `(() => {
      const composer = document.querySelector('[data-testid="unified-composer"]')
      const textarea = composer.querySelector('textarea[aria-label="发消息给 Agent"]')
      const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value').set
      setter.call(textarea, ${JSON.stringify(text)})
      textarea.dispatchEvent(new Event('change', { bubbles: true }))
      textarea.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))
      return true
    })()`
  )
}

/** 输入但不发送（用于路由预览截图）。 */
async function typeComposer(win, text) {
  await js(
    win,
    `(() => {
      const textarea = document.querySelector('[data-testid="composer-input"]')
      const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value').set
      setter.call(textarea, ${JSON.stringify(text)})
      textarea.dispatchEvent(new Event('change', { bubbles: true }))
      return true
    })()`
  )
}

async function openDoc(win, path) {
  await js(win, `window.__mileVaultStore.getState().openDocument(${JSON.stringify(path)})`)
  await waitFor(`window.__mileDocStore && window.__mileDocStore.getState().path === ${JSON.stringify(path)}`, 12000, win)
  await sleep(400)
}

async function expandDir(win, label) {
  await js(
    win,
    `(() => {
      const row = [...document.querySelectorAll('[data-testid="tree-row"], .tree-row')]
        .find((r) => (r.querySelector('.tree-label') || {}).textContent === ${JSON.stringify(label)})
      if (!row) throw new Error('找不到目录行 ${label}')
      row.click()
      return true
    })()`
  )
  await sleep(400)
}

// ===========================================================================
// PHASE: welcome —— 无任何已登记知识库
// ===========================================================================

async function phaseWelcome() {
  // 种一个可用库 + 一个指向不存在路径的「已断开」库；lastVaultId=null 不自动打开，
  // 于是落在欢迎页：主卡 + 最近库卡（一张正常、一张断开）。
  const okRoot = canon(mkdtempSync(join(tmpdir(), 'mile-baseline-okvault-')))
  writeFileSync(join(okRoot, 'note.md'), '# 可用库\n', 'utf8')
  const okId = vaultIdOf(okRoot)
  const goneRoot = join(tmpdir(), `mile-baseline-gone-${randomBytes(4).toString('hex')}`)
  const goneId = createHash('sha256').update(goneRoot.toLowerCase()).digest('hex').slice(0, 16)
  writeSettings('light', seedLayout(), [
    { id: okId, root: okRoot, displayName: '样本知识库', lastOpenedAt: new Date().toISOString() },
    { id: goneId, root: goneRoot, displayName: '已断开的库', lastOpenedAt: new Date(Date.now() - 86400000).toISOString() }
  ], null)

  // 系统目录对话框队列桩
  const nonEmptyDir = canon(mkdtempSync(join(tmpdir(), 'mile-baseline-nonempty-')))
  writeFileSync(join(nonEmptyDir, 'keep.txt'), 'keep\n', 'utf8')
  const dialogQueue = [nonEmptyDir]
  dialog.showOpenDialog = async () => {
    const next = dialogQueue.shift()
    return next ? { canceled: false, filePaths: [next] } : { canceled: true, filePaths: [] }
  }

  require(join(NEW_ROOT, '..', 'Mile-Knowledge-Base-master', 'out', 'main', 'index.js'))

  const win = await getWin()
  await setSize(win, 1440, 900)
  await sleep(1500)
  await waitFor(exists('welcome-view'), 15000, win)
  await sleep(600)

  // 欢迎页：主卡 + 已断开最近库卡（故障边框 + 重试/移除）
  await shot(win, { id: 0, name: 'welcome', title: '欢迎页（选择目录主卡 + 已断开最近知识库卡）', prdRefs: ['FR-VLT'], wait: exists('welcome-choose'), settle: 500, theme: 'light' })
  await shot(win, { id: 3, name: 'welcome', title: '欢迎页（深色）', prdRefs: ['FR-VLT', 'FR-THEME'], theme: 'dark', before: () => setTheme(win, 'dark'), wait: exists('welcome-choose'), settle: 400 })
  await setTheme(win, 'light')

  // 「新建知识库向导」→ 引导第 1 步（有已登记库时显示「返回」）
  await click(win, '[data-testid="welcome-guide-open"]').catch(async () => {
    await js(win, `window.__mileVaultStore.getState() && document.querySelector('[data-testid="welcome-hero"] button')?.click()`)
  })
  await waitFor(exists('onboarding-guide'), 8000, win).catch(async () => {
    await js(win, `(() => { const b = [...document.querySelectorAll('button')].find((x) => /新建知识库向导|向导/.test(x.textContent)); if (b) b.click(); return true })()`)
    return waitFor(exists('onboarding-guide'), 6000, win)
  })

  await shot(win, { id: 1, name: 'onboarding-step1', title: '新手引导 第1步（选择知识库目录）', prdRefs: ['FR-VLT'], wait: exists('onboarding-pick'), theme: 'light' })

  // 选非空目录 → 第 2 步（需勾选确认）
  await click(win, '[data-testid="onboarding-pick"]')
  await waitFor(exists('onboarding-confirm'), 8000, win)
  await shot(win, { id: 2, name: 'onboarding-step2-nonempty', title: '新手引导 第2步（非空目录需勾选确认）', prdRefs: ['FR-VLT'], wait: exists('onboarding-confirm-nonempty'), theme: 'light' })
}

// ===========================================================================
// PHASE: noprovider —— 有库、无模型服务
// ===========================================================================

async function phaseNoprovider() {
  const vaultRoot = buildVault()
  const vaultId = vaultIdOf(vaultRoot)
  writeSettings('light', seedLayout(), [{ id: vaultId, root: vaultRoot, displayName: 'baseline-vault', lastOpenedAt: new Date().toISOString() }], vaultId)

  require(join(NEW_ROOT, '..', 'Mile-Knowledge-Base-master', 'out', 'main', 'index.js'))

  const win = await getWin()
  await setSize(win, 1440, 900)
  await waitFor(exists('unified-composer'), 20000, win)
  await waitFor(`document.querySelectorAll('[data-testid="tree-row"], .tree-row').length > 0`, 15000, win)

  // 对话不可用状态（未配置供应商）
  await shot(win, { id: 40, name: 'chat-no-provider', title: '对话区：未配置模型服务的不可用状态', prdRefs: ['FR-AI-PROV', 'FR-AI-ENG'], wait: 'true', settle: 600, theme: 'light' })

  // 设置-模型服务空状态
  await js(win, `(() => { if (!document.querySelector('[data-testid="agnes-open-settings"]')) { document.querySelector('[data-testid="agnes-open-session"]')?.click() } return true })()`)
  await waitFor(exists('agnes-open-settings'), 6000, win)
  await click(win, '[data-testid="agnes-open-settings"]')
  await waitFor(exists('settings-view'), 8000, win)
  await click(win, '[data-testid="settings-tab-model"]')
  await waitFor(exists('settings-model'), 8000, win)
  await shot(win, { id: 41, name: 'settings-model-empty', title: '设置：模型服务（无供应商大空状态）', prdRefs: ['FR-SET', 'FR-AI-PROV'], wait: exists('provider-empty'), theme: 'light' })
}

// ===========================================================================
// PHASE: workbench —— 有库 + 假供应商，覆盖绝大部分截图
// ===========================================================================

async function phaseWorkbench() {
  const vaultRoot = buildVault()
  const vaultId = vaultIdOf(vaultRoot)
  const provider = await startFakeProvider()
  seedProvider(vaultRoot, provider.baseUrl)
  writeSettings('light', seedLayout(), [{ id: vaultId, root: vaultRoot, displayName: 'baseline-vault', lastOpenedAt: new Date().toISOString() }], vaultId)

  require(join(NEW_ROOT, '..', 'Mile-Knowledge-Base-master', 'out', 'main', 'index.js'))

  const win = await getWin()
  await setSize(win, 1440, 900)
  await waitFor(exists('unified-composer'), 20000, win)
  await waitFor(`window.__mileAgentStore.getState().sidecar.phase === 'ready'`, 25000, win)
  await waitFor(`document.querySelectorAll('[data-testid="tree-row"], .tree-row').length > 0`, 15000, win)

  const L = 'light'
  const D = 'dark'
  const S = '1440x900'

  // ---- 工作台总览（对话空状态）：文档面板默认展开 ----
  await shot(win, { id: 10, name: 'overview', title: '工作台总览（对话空状态）', prdRefs: ['FR-OVW', 'FR-WIN'], wait: exists('overview-grid'), theme: L })
  await shot(win, { id: 11, name: 'overview', title: '工作台总览（深色）', prdRefs: ['FR-OVW', 'FR-THEME'], theme: D, before: () => setTheme(win, 'dark'), wait: exists('overview-grid') })
  await setTheme(win, 'light')

  // 文档面板折叠（Ctrl+\ 或 store toggleDoc）
  await js(win, `window.__mileSettingsStore.getState().toggleDoc()`)
  await waitFor(`!!document.querySelector('aside[aria-label="文档面板入口"]')`, 6000, win)
  await shot(win, { id: 12, name: 'overview-doc-collapsed', title: '总览（文档面板折叠为竖轨）', prdRefs: ['FR-OVW', 'FR-DOC'], theme: L })
  await js(win, `window.__mileSettingsStore.getState().toggleDoc()`)
  await waitFor(`!!document.querySelector('[data-panel="editor"]')`, 6000, win)

  // ---- 文件树：展开多个功能区 ----
  for (const dir of ['skills', 'raws', 'my-notes', 'projects']) {
    try { await expandDir(win, dir) } catch (e) { console.error('展开目录失败', dir, e.message) }
  }
  await shot(win, { id: 13, name: 'tree-expanded', title: '文件树：展开多个功能区', prdRefs: ['FR-TREE', 'FR-AREA'], wait: 'true', settle: 500, theme: L })

  // 目录行悬停出现操作按钮（真实 mouseMove）
  try {
    const rect = await js(win, `(() => { const r = [...document.querySelectorAll('.tree-row')].find((x) => (x.querySelector('.tree-label')||{}).textContent === 'raws'); const b = r.getBoundingClientRect(); return { x: Math.round(b.x + b.width/2), y: Math.round(b.y + b.height/2) } })()`)
    win.webContents.sendInputEvent({ type: 'mouseMove', x: rect.x, y: rect.y })
    await sleep(350)
    await shot(win, { id: 14, name: 'tree-row-hover', title: '文件树：目录行悬停显示操作按钮', prdRefs: ['FR-TREE'], wait: 'true', settle: 200, theme: L })
  } catch (e) { record({ id: '14', file: '', title: '文件树：目录行悬停显示操作按钮', prdRefs: ['FR-TREE'], phase: PHASE, theme: L, size: S, status: 'failed', note: e.message }) }

  // 内联重命名：raws 已展开，直接用 tree-actions-<path> 容器里的「重命名」按钮。
  try {
    await js(win, `(() => {
      const box = document.querySelector('[data-testid="tree-actions-raws/共享笔记.md"]')
      if (!box) throw new Error('找不到 tree-actions-raws/共享笔记.md')
      const btn = [...box.querySelectorAll('button')].find((b) => (b.getAttribute('aria-label')||'').startsWith('重命名'))
      if (!btn) throw new Error('找不到重命名按钮')
      btn.click(); return true
    })()`)
    await waitFor(exists('inline-rename-input'), 5000, win)
    await shot(win, { id: 15, name: 'tree-inline-rename', title: '文件树：内联重命名', prdRefs: ['FR-TREE'], wait: exists('inline-rename-input'), settle: 200, theme: L })
    await js(win, `document.querySelector('[data-testid="inline-rename-input"]').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))`)
    await sleep(200)
  } catch (e) { record({ id: '15', file: '', title: '文件树：内联重命名', prdRefs: ['FR-TREE'], phase: PHASE, theme: L, size: S, status: 'failed', note: e.message }) }

  // 内联新建：侧栏「新建文件」按钮。
  try {
    await js(win, `(() => {
      const btn = [...document.querySelectorAll('button')].find((b) => /新建文件|新建/.test(b.getAttribute('aria-label')||'') || /新建文件/.test(b.textContent))
      if (!btn) throw new Error('找不到新建按钮')
      btn.click(); return true
    })()`)
    await waitFor(exists('inline-create-input'), 5000, win)
    await shot(win, { id: '15b', name: 'tree-inline-create', title: '文件树：内联新建', prdRefs: ['FR-TREE'], wait: exists('inline-create-input'), settle: 200, theme: L })
    await js(win, `document.querySelector('[data-testid="inline-create-input"]').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))`)
    await sleep(200)
  } catch (e) { record({ id: '15b', file: '', title: '文件树：内联新建', prdRefs: ['FR-TREE'], phase: PHASE, theme: L, size: S, status: 'failed', note: e.message }) }

  // ---- 文档：阅读模式（富文本样例，含三种 wikilink）----
  await openDoc(win, 'raws/富文本样例.md')
  await js(win, `window.__mileDocStore.getState().setMode('read')`)
  await waitFor(exists('markdown-reader'), 8000, win)
  await shot(win, { id: 16, name: 'doc-read', title: '文档阅读模式（表格/任务/代码/图片/wikilink三态）', prdRefs: ['FR-DOC', 'FR-LINK'], wait: exists('markdown-reader'), settle: 500, theme: L })
  await shot(win, { id: 17, name: 'doc-read', title: '文档阅读模式（深色）', prdRefs: ['FR-DOC', 'FR-THEME'], theme: D, before: () => setTheme(win, 'dark'), wait: exists('markdown-reader') })
  await setTheme(win, 'light')

  // 源码模式
  await js(win, `window.__mileDocStore.getState().setMode('source')`)
  await waitFor(exists('markdown-editor'), 8000, win)
  await shot(win, { id: 18, name: 'doc-source', title: '文档源码模式（等宽+行号）', prdRefs: ['FR-DOC'], wait: exists('markdown-editor'), settle: 400, theme: L })

  // 分栏模式
  await js(win, `window.__mileDocStore.getState().setMode('split')`)
  await waitFor(exists('split-preview'), 8000, win)
  await shot(win, { id: 19, name: 'doc-split', title: '文档分栏模式（左源码右阅读）', prdRefs: ['FR-DOC'], wait: exists('split-preview'), settle: 400, theme: L })
  await js(win, `window.__mileDocStore.getState().setMode('read')`)

  // ---- 反向链接 + 未解析链接面板（共享笔记：有入链有未解析）----
  await openDoc(win, 'raws/共享笔记.md')
  await shot(win, { id: 20, name: 'doc-backlinks', title: '反向链接与未解析链接面板', prdRefs: ['FR-LINK'], wait: exists('backlinks-panel'), settle: 500, theme: L })

  // ---- 来源追踪面板（both-changed）----
  await openDoc(win, 'projects/mile-go/react-skill-copy.md')
  await shot(win, { id: 21, name: 'doc-provenance', title: '来源追踪面板（两边都改了，无同步按钮）', prdRefs: ['FR-PROV'], wait: exists('provenance-panel'), settle: 500, theme: L })

  // ---- 多标签 + 脏标记 ----
  await openDoc(win, 'raws/缺标题.md')
  await js(win, `window.__mileDocStore.getState().setMode('source')`)
  await waitFor(exists('markdown-editor'), 8000, win)
  // edit() 是唯一设脏入口。
  await js(win, `(() => {
    const s = window.__mileDocStore.getState()
    s.edit(s.text + '\\n编辑了一点内容')
    return true
  })()`)
  await waitFor(`window.__mileDocStore.getState().dirty === true`, 5000, win)
  await sleep(300)
  await shot(win, { id: 22, name: 'doc-tabs-dirty', title: '多标签 + 未保存脏标记', prdRefs: ['FR-DOC'], wait: 'true', settle: 400, theme: L })

  // ---- 外部修改冲突卡：本地 dirty 时外部改磁盘 → diskPending；save() → 冲突卡 ----
  try {
    writeFileSync(join(vaultRoot, 'raws', '缺标题.md'), '外部程序改过的内容，与本地编辑冲突。\n', 'utf8')
    // 等 watcher 把外部改动推成 diskPending（本地 dirty，不会被 adopt）。
    await waitFor(`window.__mileDocStore.getState().diskPending !== null`, 8000, win)
    await js(win, `window.__mileDocStore.getState().save()`)
    await waitFor(exists('conflict-card'), 8000, win)
    await shot(win, { id: 23, name: 'doc-conflict-card', title: '外部修改冲突卡（保留当前/采用磁盘/合并）', prdRefs: ['FR-DOC'], wait: exists('conflict-card'), settle: 300, theme: L })
    // 保留当前（重新提交本地正文）关闭冲突卡。
    await js(win, `window.__mileDocStore.getState().resolveKeepMine()`).catch(() => {})
    await sleep(400)
  } catch (e) { record({ id: '23', file: '', title: '外部修改冲突卡', prdRefs: ['FR-DOC'], phase: PHASE, theme: L, size: S, status: 'failed', note: e.message }) }

  // ---- 大文件：>1 MiB 阅读禁用提示 ----
  await openDoc(win, 'raws/big-1m.md')
  await js(win, `window.__mileDocStore.getState().setMode('read')`).catch(() => {})
  await sleep(400)
  await shot(win, { id: 24, name: 'doc-big-1m', title: '大文件 >1 MiB（阅读降级提示）', prdRefs: ['FR-DOC'], wait: 'true', settle: 500, theme: L })

  // ---- 大文件：>2 MiB 只读 ----
  await openDoc(win, 'raws/huge-2m.md')
  await shot(win, { id: 25, name: 'doc-huge-2m', title: '大文件 >2 MiB（只读预览）', prdRefs: ['FR-DOC'], wait: 'true', settle: 600, theme: L })

  // ---- 搜索面板 ----
  await openDoc(win, 'raws/富文本样例.md')
  await js(win, `window.__mileDocStore.getState().setMode('read')`)
  await js(win, `document.querySelector('[data-testid="sidebar-search"]').click()`).catch(() =>
    js(win, `window.__mileVaultStore.getState() && (window.dispatchEvent(new KeyboardEvent('keydown', { key: 'p', ctrlKey: true, bubbles: true })), true)`))
  await waitFor(exists('search-panel'), 8000, win).catch(async () => {
    await js(win, `(window.__mileSearchStore ? window.__mileSearchStore.getState().openPanel() : null)`)
    return waitFor(exists('search-panel'), 6000, win)
  })
  // 有结果（中文查询）
  await setInput(win, 'search-input', '知识库')
  await waitFor(`document.querySelectorAll('[data-testid="search-hit"]').length > 0`, 12000, win)
  await shot(win, { id: 26, name: 'search-results', title: '搜索面板：中文查询有结果（高亮+匹配字段）', prdRefs: ['FR-SRCH'], wait: 'true', settle: 400, theme: L })
  // 无结果：用确定无命中词，等搜索结束（searching false 且 hits 空）再截空态。
  try {
    await setInput(win, 'search-input', 'zzqqxx987')
    await waitFor(`window.__mileSearchStore.getState().searching === false && window.__mileSearchStore.getState().hits.length === 0 && window.__mileSearchStore.getState().text === 'zzqqxx987'`, 12000, win)
    await waitFor(exists('search-empty'), 6000, win)
    await shot(win, { id: 27, name: 'search-empty', title: '搜索面板：无结果空状态', prdRefs: ['FR-SRCH'], wait: 'true', settle: 300, theme: L })
  } catch (e) {
    await shot(win, { id: 27, name: 'search-empty', title: '搜索面板：无结果空状态', prdRefs: ['FR-SRCH'], wait: 'true', settle: 300, theme: L, note: `空态等待失败兜底: ${e.message}` })
  }
  // 筛选已选：点一个扩展名快选 chip，再截。
  try {
    await setInput(win, 'search-input', '知识')
    await waitFor(`window.__mileSearchStore.getState().searching === false`, 10000, win).catch(() => {})
    await js(win, `(() => {
      const panel = document.querySelector('[data-testid="search-panel"]')
      const chips = [...panel.querySelectorAll('button')]
      const ext = chips.find((c) => /^\\.?md$|markdown/i.test(c.textContent.trim()))
      if (ext) ext.click()
      return true
    })()`).catch(() => {})
    await sleep(500)
    await shot(win, { id: 28, name: 'search-filtered', title: '搜索面板：已选筛选条件', prdRefs: ['FR-SRCH'], wait: 'true', settle: 300, theme: L })
  } catch (e) {
    await shot(win, { id: 28, name: 'search-filtered', title: '搜索面板：已选筛选条件', prdRefs: ['FR-SRCH'], wait: 'true', settle: 300, theme: L, note: `筛选驱动兜底: ${e.message}` })
  }
  // 关闭搜索
  await js(win, `window.__mileSearchStore.getState().closePanel()`).catch(() => {})
  await sleep(300)

  // ---- 变更计划卡（归档一个文件）----
  await js(win, `(() => {
    window.__mileMutationStore.getState().propose({
      vaultId: ${JSON.stringify(vaultId)},
      title: '归档 孤立文档.md',
      actions: [{ kind: 'archive', source: 'raws/孤立文档.md', reason: 'baseline 截图' }]
    })
    return true
  })()`)
  await waitFor(exists('mutation-plan-card'), 8000, win)
  await shot(win, { id: 29, name: 'mutation-plan-card', title: '变更计划卡（归档，含风险等级与恢复方式）', prdRefs: ['FR-MUT'], wait: exists('mutation-plan-card'), settle: 400, theme: L })
  // 确认执行 → 报告卡
  await js(win, `(() => { const c = document.querySelector('[data-testid="mutation-plan-card"]'); const arm = c.querySelector('[data-testid="plan-arm-destructive"]'); if (arm) arm.click(); return true })()`)
  await sleep(200)
  await click(win, '[data-testid="plan-confirm"]')
  await waitFor(exists('mutation-report'), 10000, win)
  await shot(win, { id: 30, name: 'mutation-report', title: '变更执行报告卡', prdRefs: ['FR-MUT'], wait: exists('mutation-report'), settle: 400, theme: L })
  await js(win, `window.__mileMutationStore.getState().dismissReport()`)
  await sleep(300)

  // ---- 永久删除计划卡（高风险 + 预确认）----
  await js(win, `(() => {
    window.__mileMutationStore.getState().propose({
      vaultId: ${JSON.stringify(vaultId)},
      title: '永久删除 孤立文档.md',
      actions: [{ kind: 'permanentDelete', source: 'raws/孤立文档.md', reason: 'baseline 截图' }],
      freshApproval: true
    })
    return true
  })()`)
  await waitFor(exists('mutation-plan-card'), 8000, win)
  await shot(win, { id: 31, name: 'mutation-plan-destructive', title: '永久删除计划卡（高风险+我了解风险预确认）', prdRefs: ['FR-MUT', 'FR-AI-PERM'], wait: exists('plan-arm-destructive'), settle: 400, theme: L })
  await js(win, `window.__mileMutationStore.getState().cancel()`)
  await sleep(300)

  // ---- 操作日志面板 ----
  await click(win, 'button[aria-label="查看操作日志"]')
  await waitFor(exists('audit-panel'), 8000, win)
  await waitFor(`document.querySelectorAll('[data-testid="audit-records"] li').length > 0`, 8000, win).catch(() => {})
  await shot(win, { id: 32, name: 'audit-panel', title: '操作日志面板', prdRefs: ['FR-MUT'], wait: exists('audit-panel'), settle: 400, theme: L })
  await js(win, `document.querySelector('button[aria-label="关闭操作日志"]')?.click()`)
  await sleep(300)

  // ---- 健康面板（总览磁贴进入）----
  await js(win, `window.__mileDocStore.getState().closeDocument ? window.__mileDocStore.getState().closeDocument() : null`)
  await sleep(300)
  // 回到总览（开新对话/清空消息）。总览在无消息时显示；先发的话会盖住。直接点磁贴。
  await js(win, `document.querySelector('[data-testid="overview-health"]')?.click()`)
  await waitFor(exists('health-panel'), 8000, win).catch(async () => {
    // 若总览没显示（已有对话），先新对话
    await js(win, `document.querySelector('[data-testid="agnes-new-conversation"]')?.click()`)
    await sleep(400)
    await js(win, `document.querySelector('[data-testid="overview-health"]')?.click()`)
    return waitFor(exists('health-panel'), 8000, win)
  })
  await shot(win, { id: 33, name: 'health-panel', title: '知识库健康面板（断链/孤立/同名/缺标题）', prdRefs: ['FR-HLTH'], wait: exists('health-panel'), settle: 600, theme: L })
  await js(win, `document.querySelector('[data-testid="health-panel"] button[aria-label*="关"], [data-testid="health-panel"] .btn-quiet')?.click()`).catch(() => {})
  await js(win, `document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))`).catch(() => {})
  await sleep(300)

  // ================= 对话区 =================
  // 回到总览空状态
  await js(win, `document.querySelector('[data-testid="agnes-new-conversation"]')?.click()`)
  await sleep(400)

  // 路由预览（输入但不发送）
  await typeComposer(win, '帮我整理归档这个目录')
  await sleep(400)
  await shot(win, { id: 34, name: 'chat-route-preview', title: '对话：路由预览「发给 整理归档」', prdRefs: ['FR-AI-ROUTE'], wait: exists('route-target'), settle: 300, theme: L })

  // 改派菜单展开
  await click(win, '[data-testid="route-override-toggle"]')
  await waitFor(exists('route-picker'), 5000, win)
  await shot(win, { id: 35, name: 'chat-route-picker', title: '对话：改派菜单展开（自动+4 Agent）', prdRefs: ['FR-AI-ROUTE'], wait: exists('route-picker'), settle: 300, theme: L })
  await js(win, `document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))`).catch(() => {})
  await js(win, `document.querySelector('[data-testid="route-override-toggle"]').click()`).catch(() => {})
  await sleep(250)

  // 运行参数条：折叠与展开
  await js(win, `(() => { const t = document.querySelector('[data-testid="runtime-controls-toggle"]'); if (t && t.getAttribute('aria-expanded') === 'false') t.click(); return true })()`).catch(() => {})
  await sleep(300)
  await shot(win, { id: 36, name: 'chat-runtime-controls', title: '对话：运行参数条展开（模型/强度/执行模式）', prdRefs: ['FR-AI-CHAT', 'FR-AI-PERM'], wait: exists('runtime-controls'), settle: 300, theme: L })

  // 发送 STREAM_MD → 流式进行中
  await typeComposer(win, '')
  await js(win, `(() => { const t = document.querySelector('[data-testid="composer-input"]'); const s = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set; s.call(t,''); t.dispatchEvent(new Event('change',{bubbles:true})); return true })()`)
  const sendPromise = sendMessage(win, 'STREAM_MD')
  await sleep(600) // 流式中途
  await shot(win, { id: 37, name: 'chat-streaming', title: '对话：流式输出进行中（思考+正文+呼吸动画）', prdRefs: ['FR-AI-CHAT'], wait: 'true', settle: 100, theme: L })
  await sendPromise.catch(() => {})
  // 等回合结束
  await waitFor(`document.body.innerText.includes('加粗')`, 15000, win).catch(() => {})
  await sleep(400)
  await shot(win, { id: 38, name: 'chat-answer', title: '对话：回答完成（思考+Markdown+表格）', prdRefs: ['FR-AI-CHAT'], wait: 'true', settle: 500, theme: L })
  await shot(win, { id: 39, name: 'chat-answer', title: '对话：回答完成（深色）', prdRefs: ['FR-AI-CHAT', 'FR-THEME'], theme: D, before: () => setTheme(win, 'dark'), wait: 'true', settle: 500 })
  await setTheme(win, 'light')

  // PERMISSION_TEST → 审批卡
  await sendMessage(win, 'PERMISSION_TEST')
  await waitFor(`window.__mileAgentStore.getState().agents.general.pendingPermissions.length > 0`, 15000, win).catch(async () => {
    // 目标可能不是 general：等任一 agent 出现 pendingPermissions
    return waitFor(`Object.values(window.__mileAgentStore.getState().agents).some((a) => (a.pendingPermissions||[]).length > 0)`, 10000, win)
  })
  await waitFor(exists('permission-card'), 8000, win).catch(() => {})
  await shot(win, { id: 42, name: 'chat-permission-card', title: '对话：写操作审批卡（允许/拒绝）', prdRefs: ['FR-AI-PERM'], wait: 'true', settle: 400, theme: L })
  // 拒绝，避免落盘
  await js(win, `(() => { const b = [...document.querySelectorAll('[role="group"][aria-label*="工具审批"] button')].find((x) => x.textContent.trim() === '拒绝'); if (b) b.click(); return true })()`).catch(() => {})
  await sleep(400)

  // USAGE → 用量收据
  await sendMessage(win, 'USAGE')
  await waitFor(`document.body.innerText.includes('USAGE_SENT')`, 15000, win).catch(() => {})
  await sleep(500)
  await shot(win, { id: 43, name: 'chat-usage', title: '对话：用量收据（token+缓存命中+费用）', prdRefs: ['FR-AI-USAGE'], wait: 'true', settle: 500, theme: L })

  // 会话信息浮层
  await js(win, `(() => { if (!document.querySelector('[data-testid="agnes-session-info"], [data-testid="agnes-open-settings"]')) { document.querySelector('[data-testid="agnes-open-session"]')?.click() } else { document.querySelector('[data-testid="agnes-open-session"]')?.click() } return true })()`)
  await waitFor(exists('agnes-open-settings'), 6000, win)
  await shot(win, { id: 44, name: 'chat-session-info', title: '会话信息浮层（引擎/权限/共享上下文/设置入口）', prdRefs: ['FR-AI-CTX', 'FR-SET'], wait: exists('agnes-open-settings'), settle: 300, theme: L })
  await js(win, `document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))`).catch(() => {})
  await sleep(250)

  // ---- 历史栏（再造 2 条对话以凑分组）----
  const convCount = () => js(win, `window.__mileConversationStore.getState().conversations.length`)
  await sendMessage(win, '历史对话第二条 关于整理')
  await waitFor(`window.__mileConversationStore.getState().conversations.length >= 2`, 15000, win).catch((e) => console.error('conv2 timeout', e.message))
  await js(win, `document.querySelector('[data-testid="agnes-new-conversation"]')?.click()`)
  await sleep(500)
  await sendMessage(win, '历史对话第三条 关于检索')
  await waitFor(`window.__mileConversationStore.getState().conversations.length >= 3`, 15000, win).catch((e) => console.error('conv3 timeout', e.message))
  console.log('  conversations =', await convCount())
  await sleep(500)
  // 置顶第一条
  if (!(await js(win, exists('history-panel')))) {
    await click(win, '[data-testid="agnes-open-history"]').catch(() => {})
  }
  await waitFor(exists('history-panel'), 6000, win)
  await js(win, `(() => { const p = document.querySelector('[data-testid="history-pin"]'); if (p) p.click(); return true })()`).catch(() => {})
  await sleep(400)
  await shot(win, { id: 45, name: 'history-panel', title: '对话历史栏（置顶/今天分组）', prdRefs: ['FR-AI-HIST'], wait: `document.querySelectorAll('[data-testid="history-row"]').length >= 2`, settle: 400, theme: L })
  // 历史搜索
  await setInput(win, 'history-search', '整理')
  await waitFor(`document.querySelectorAll('[data-testid="history-hit"]').length > 0`, 8000, win).catch(() => {})
  await shot(win, { id: 46, name: 'history-search', title: '对话历史搜索', prdRefs: ['FR-AI-HIST'], wait: 'true', settle: 400, theme: L })
  await setInput(win, 'history-search', '')
  await sleep(300)

  // ---- 设置页 5 个分区 ----
  await js(win, `(() => { if (!document.querySelector('[data-testid="agnes-open-settings"]')) { document.querySelector('[data-testid="agnes-open-session"]')?.click() } return true })()`)
  await waitFor(exists('agnes-open-settings'), 6000, win)
  await click(win, '[data-testid="agnes-open-settings"]')
  await waitFor(exists('settings-view'), 8000, win)

  await click(win, '[data-testid="settings-tab-appearance"]')
  await waitFor(exists('settings-appearance'), 8000, win)
  await shot(win, { id: 47, name: 'settings-appearance', title: '设置：常规（主题三档+关窗+面板宽度）', prdRefs: ['FR-SET', 'FR-THEME'], wait: exists('settings-appearance'), settle: 400, theme: L })
  await shot(win, { id: 48, name: 'settings-appearance', title: '设置：常规（深色）', prdRefs: ['FR-SET', 'FR-THEME'], theme: D, before: () => setTheme(win, 'dark'), wait: exists('settings-appearance'), settle: 400 })
  await setTheme(win, 'light')

  await click(win, '[data-testid="settings-tab-model"]')
  await waitFor(exists('settings-model'), 8000, win)
  await shot(win, { id: 49, name: 'settings-model', title: '设置：模型服务（已配置供应商）', prdRefs: ['FR-SET', 'FR-AI-PROV'], wait: exists('settings-model'), settle: 400, theme: L })
  await shot(win, { id: 50, name: 'settings-model', title: '设置：模型服务（深色）', prdRefs: ['FR-SET', 'FR-THEME'], theme: D, before: () => setTheme(win, 'dark'), wait: exists('settings-model'), settle: 400 })
  await setTheme(win, 'light')

  await click(win, '[data-testid="settings-tab-agents"]')
  await waitFor(exists('settings-agents'), 8000, win)
  await shot(win, { id: 51, name: 'settings-agents', title: '设置：Agents（4卡切换+模型+强度+执行模式+Steering）', prdRefs: ['FR-SET', 'FR-AI-AGT', 'FR-AI-STEER'], wait: exists('settings-agents'), settle: 400, theme: L })

  await click(win, '[data-testid="settings-tab-categories"]')
  await waitFor(exists('settings-categories'), 8000, win)
  await shot(win, { id: 52, name: 'settings-categories', title: '设置：分类与检索（真实一级目录表格）', prdRefs: ['FR-SET', 'FR-AREA', 'FR-SRCH'], wait: exists('settings-categories'), settle: 400, theme: L })

  await click(win, '[data-testid="settings-tab-about"]')
  await waitFor(exists('settings-about'), 8000, win)
  await shot(win, { id: 53, name: 'settings-about', title: '设置：关于与诊断', prdRefs: ['FR-SET'], wait: exists('settings-about'), settle: 400, theme: L })

  // 未保存草稿的保存条：在 agents 改 steering
  await click(win, '[data-testid="settings-tab-agents"]')
  await waitFor(exists('settings-agents'), 8000, win)
  await js(win, `(() => {
    const box = document.querySelector('textarea[data-testid^="agent-steering-"]')
    if (!box) throw new Error('找不到 steering textarea')
    const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value').set
    setter.call(box, (box.value || '') + '\\n baseline draft')
    box.dispatchEvent(new Event('change', { bubbles: true }))
    return true
  })()`).catch((e) => console.error('steering draft fail', e.message))
  await waitFor(exists('settings-savebar'), 6000, win).catch(() => {})
  await shot(win, { id: 54, name: 'settings-savebar', title: '设置：未保存草稿的保存条', prdRefs: ['FR-SET', 'FR-AI-STEER'], wait: exists('settings-savebar'), settle: 300, theme: L })
  await js(win, `document.querySelector('[data-testid="settings-savebar-discard"]')?.click()`).catch(() => {})
  await sleep(250)
  await js(win, `document.querySelector('[data-testid="settings-close"]')?.click()`)
  await waitFor(`!document.querySelector('[data-testid="settings-view"]')`, 6000, win).catch(() => {})

  // ---- 技能导入向导 第1步 ----
  await click(win, 'button[aria-label="导入技能"]')
  await waitFor(exists('skill-import-wizard'), 8000, win)
  await shot(win, { id: 55, name: 'skill-import-step1', title: '技能导入向导 第1步（选来源文件夹）', prdRefs: ['FR-SKILL'], wait: exists('skill-import-pick'), settle: 300, theme: L })
  await js(win, `document.querySelector('button[aria-label="关闭导入向导"]')?.click()`)
  await sleep(300)

  // ---- 最小尺寸 744x640 ----
  await setSize(win, 744, 640)
  await js(win, `document.querySelector('[data-testid="agnes-new-conversation"]')?.click()`).catch(() => {})
  await sleep(400)
  await shot(win, { id: 56, name: 'overview-min', title: '总览（最小尺寸 744x640）', prdRefs: ['FR-OVW', 'FR-WIN'], wait: 'true', settle: 400, theme: L, size: '744x640' })
  await openDoc(win, 'raws/富文本样例.md').catch(() => {})
  await js(win, `window.__mileDocStore.getState().setMode('read')`).catch(() => {})
  await sleep(400)
  await shot(win, { id: 57, name: 'doc-read-min', title: '阅读模式（最小尺寸 744x640）', prdRefs: ['FR-DOC', 'FR-WIN'], wait: 'true', settle: 400, theme: L, size: '744x640' })
  await setSize(win, 1440, 900)

  // ---- 原生无法截取项 ----
  record({ id: '58', file: '', title: '关窗询问（挂到托盘/退出）', prdRefs: ['FR-WIN'], phase: PHASE, theme: L, size: S, status: 'native', note: '原生对话框，无法截取' })
  record({ id: '59', file: '', title: '系统选择目录对话框', prdRefs: ['FR-VLT'], phase: PHASE, theme: L, size: S, status: 'native', note: '原生系统对话框，无法截取' })
  record({ id: '60', file: '', title: '关闭脏标签确认（原生）', prdRefs: ['FR-DOC'], phase: PHASE, theme: L, size: S, status: 'native', note: '若为原生对话框则无法截取' })

  await provider.close().catch(() => {})
}

// ===========================================================================
// 入口
// ===========================================================================

async function main() {
  try {
    if (PHASE === 'welcome') await phaseWelcome()
    else if (PHASE === 'noprovider') await phaseNoprovider()
    else await phaseWorkbench()
    console.log(`\n[done] phase=${PHASE} shots=${manifest.length}`)
    return 0
  } catch (error) {
    console.error('capture 异常:', error && error.stack ? error.stack : error)
    return 1
  }
}

// 总超时看门狗（12 分钟），防挂住。
const watchdog = setTimeout(() => {
  console.error('总超时，强制退出')
  app.exit(2)
}, 12 * 60 * 1000)

app.whenReady().then(async () => {
  await sleep(1000)
  const code = await main()
  clearTimeout(watchdog)
  app.exit(code)
})

// 清理沙箱（只删自己 mkdtemp 建的目录）
process.on('exit', () => {
  try { rmSync(sandbox, { recursive: true, force: true }) } catch { /* 忽略 */ }
})
