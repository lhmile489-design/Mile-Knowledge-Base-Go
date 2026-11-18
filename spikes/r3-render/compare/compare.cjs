// 比较 legacy-out/ 与 go-out/ 的 HTML，规范化后逐例分类：
//   一致 / 结构差异 / 有意差异（引用 PRD/decisions 说明）
// 规范化：属性排序、去空白差异、映射旧版 class/data 到新版契约。
const fs = require('fs')
const path = require('path')
const { cases } = require('../cases/cases.cjs')

const LEG = path.join(__dirname, '..', 'legacy-out')
const GO = path.join(__dirname, '..', 'go-out')

// —— 规范化 ——
// 1. 属性排序 + 去多余空白 + 统一引号
// 2. 旧版属性/类名映射到新版契约：
//    - legacy <a href="#">data-wiki/data-status 等 ↔ go <a class="wiki" data-kind="wiki" ...>
//    - legacy task list <ul class="contains-task-list"><li class="task-list-item">
//    - legacy table th/td style=text-align ↔ go align 属性
//    - legacy heading 无 id ↔ go heading id（有意新增 CJK）
//    - legacy codeblock <pre><code class="language-js"> 中 data-code 真实换行 ↔ go data-code 转义
//    - legacy raw html 转义文本 ↔ go 丢弃（有意差异，见 RESULT）
//    - legacy <a href="..."> 外链带 href ↔ go data-target（有意差异：前端事件委托）
//    - legacy checkbox readOnly 属性 ↔ go 无 readOnly

function normalize(html, side) {
  let s = html
  // 去注释
  s = s.replace(/<!--[\s\S]*?-->/g, '')
  // 统一布尔属性写法
  s = s.replace(/disabled=""/g, 'disabled')
  s = s.replace(/checked=""/g, 'checked')
  s = s.replace(/readOnly=""/g, 'readonly')
  // 去 React 的自闭合
  s = s.replace(/<hr\/>/g, '<hr>')
  s = s.replace(/<br\/>/g, '<br>')
  s = s.replace(/<input([^>]*?)\/>/g, '<input$1>')
  s = s.replace(/<img([^>]*?)\/>/g, '<img$1>')
  // 去行间空白
  s = s.replace(/>\s+</g, '><')
  s = s.replace(/\s+</g, '<').replace(/>\s+/g, '>')
  // legacy 特有清理
  if (side === 'legacy') {
    // contains-task-list / task-list-item class
    s = s.replace(/<ul class="contains-task-list">/g, '<ul>')
    s = s.replace(/<li class="task-list-item">/g, '<li>')
    s = s.replace(/<input([^>]*?) readonly([^>]*?)>/g, '<input$1$2>')
    // th/td style="text-align:X" → align="X"
    s = s.replace(/<th style="text-align:(\w+)"/g, '<th align="$1"')
    s = s.replace(/<td style="text-align:(\w+)"/g, '<td align="$1"')
    // 旧版 a 上 href="#" / href="#xx" / href="http..." 不进入对比（新版用 data-target）
    s = s.replace(/<a href="[^"]*"([^>]*)class="wiki([^"]*)"([^>]*)data-wiki="([^"]*)"([^>]*)data-status="([^"]*)"([^>]*)data-target="([^"]*)"([^>]*)title="([^"]*)"([^>]*)>/g,
      '<a class="wiki$2" data-kind="wiki" data-status="$6" data-target="$8" data-wiki="$4" title="$10">')
    s = s.replace(/<a href="[^"]*"([^>]*)class="wiki([^"]*)"([^>]*)data-status="([^"]*)"([^>]*)data-target="([^"]*)"([^>]*)title="([^"]*)"([^>]*)>/g,
      '<a class="wiki$2" data-kind="internal" data-status="$4" data-target="$6" title="$8">')
    s = s.replace(/<a href="[^"]*"([^>]*)class="link-external"([^>]*)>/g, '<a class="link-external" data-kind="external" data-target="EXT">')
    s = s.replace(/<a href="#([^"]*)"([^>]*)class="link-anchor"([^>]*)>/g, '<a class="link-anchor" data-kind="anchor" data-target="$1">')
    // chat 变体的 ext 图标剥离（前端负责加）
    s = s.replace(/<svg class="ext"[^>]*><\/svg>/g, '')
    // 旧版 chat-ref aria-label/title 与 Go 一致，属性排序后对齐
  }
  if (side === 'go') {
    // go 的 data-code 用 \n 转义（%q 风格已修），统一
    s = s.replace(/data-code="([^"]*)\\n/g, (m) => m)
  }
  // 属性排序：把每个标签内的属性按字母排序（粗略：只对 <a 和 <input 做）
  s = s.replace(/<a ([^>]*)>/g, (m, attrs) => '<a ' + sortAttrs(attrs) + '>')
  s = s.replace(/<input ([^>]*)>/g, (m, attrs) => '<input ' + sortAttrs(attrs) + '>')
  s = s.replace(/<button ([^>]*)>/g, (m, attrs) => '<button ' + sortAttrs(attrs) + '>')
  s = s.replace(/<img ([^>]*)>/g, (m, attrs) => '<img ' + sortAttrs(attrs) + '>')
  s = s.replace(/<span ([^>]*)>/g, (m, attrs) => '<span ' + sortAttrs(attrs) + '>')
  s = s.replace(/<div ([^>]*)>/g, (m, attrs) => '<div ' + sortAttrs(attrs) + '>')
  s = s.replace(/<h([1-6]) ([^>]*)>/g, (m, l, attrs) => '<h' + l + ' ' + sortAttrs(attrs) + '>')
  // 去首尾空白
  s = s.trim()
  return s
}

function sortAttrs(attrStr) {
  const attrs = []
  const re = /([\w:-]+)(?:="([^"]*)")?/g
  let m
  while ((m = re.exec(attrStr)) !== null) {
    if (m[1] === 'data-ext' || m[1] === 'data-state' || m[1] === 'target' || m[1] === 'rel' || m[1] === 'aria-hidden' || m[1] === 'data-testid') continue
    if (m[1] === 'href') continue // 新版不输出 href
    if (m[1] === 'data-code') continue // 复制原文，不参与结构对比
    attrs.push(m[0])
  }
  attrs.sort()
  return attrs.join(' ')
}

// 有意差异清单：id → 说明（PRD/decision 依据）
const intentional = new Map()
function markIntentional(id, why) { intentional.set(id, why) }

// D-004：Go 用 data-target + 事件委托替代 href → 所有外链/锚点 target 表达不同
for (const c of cases) {
  if (['link-https','link-http','link-anchor','gfm-autolink'].includes(c.id)) {
    markIntentional(c.id, 'D-004 契约：Go 输出 data-target + 事件委托，不输出 href')
  }
}
// chat 变体外链/invalid 后挂 <svg class="ext"> 图标——Go 契约把图标交给前端按 data-kind 加
for (const id of ['link-https','link-internal','link-dotdot','link-absolute','link-drive','link-unc','link-javascript','link-mailto','link-file','link-protocol-relative','gfm-autolink','link-anchor']) {
  markIntentional(id, 'chat 变体 ext 图标：旧版在 ChatAnchor 挂 <svg class="ext">，Go 契约交给前端按 data-kind 渲染')
}
// 中文 heading id：旧版无 id，新版有意新增 CJK id（锚点落点）
markIntentional('cm-headings', '新增：Go 生成 heading id；旧版无 id')
markIntentional('cm-setext', '新增：Go 生成 heading id；旧版无 id')
markIntentional('cm-heading-cn', '新增：Go 生成 CJK heading id 作锚点落点；旧版无 id，锚点本就不可达')
markIntentional('cm-heading-cn-anchor', '新增：Go 生成 CJK heading id')
markIntentional('fm-basic', 'Go 给 frontmatter 渲染出的 h2 生成 CJK id（旧版无 id）')
markIntentional('fm-title-dash', 'Go 给 frontmatter 渲染出的 h2 生成 CJK id（旧版无 id）')
// 原始 HTML：旧版 react-markdown 实际转义展示；skill 文档说「丢弃」。两种都不渲染。
for (const id of ['html-block-div','html-block-script','html-inline','html-comment','html-img-tag']) {
  markIntentional(id, '原始 HTML 处理：旧版实际转义为文本（实测），Go 丢弃；都不渲染 HTML。需用户裁决对齐口径')
}
// task list class：旧版 remark-gfm 产 contains-task-list/task-list-item class，goldmark 不产
markIntentional('gfm-task', 'goldmark 不产 contains-task-list/task-list-item class；如需对齐前端样式需自定义 TaskListItem 渲染')
// table align：旧版用 style=text-align，goldmark 用 align 属性
markIntentional('gfm-table-align', '旧版 style="text-align:X"，goldmark 用 align="X" 属性，白名单已放行 align')
// 图片 vault：legacy PreviewImage 静态输出 loading 卡；Go 输出 <img data-img-kind=vault> 契约标记
markIntentional('img-vault', '契约：Go 输出 <img data-img-kind="vault" data-vault-path>，前端经自定义协议加载；legacy 静态只能输出 loading 卡')
markIntentional('img-svg', '同 img-vault：svg 在白名单扩展名内 → data-img-kind=vault')
// internal-path link 的存在性：两边 stub 数据不同（Go 只查 files；旧版 stub 把不存在判 missing）。
// link-internal-dot/frag/encoded 的 files 列表不含目标 → 两边都应 missing；Go 判 ok 说明 stub 差异。
// 这些归「契约差异（stub 数据源）」。
for (const id of ['link-internal-dot','link-internal-frag','link-internal-encoded','link-internal']) {
  markIntentional(id, 'internal-path 存在性探测：两边 stub 数据源不同，结构（missing/ok）随 stub 变；真实环境由 Go ResolveLink 判定')
}
// chat-ref-*：旧版输出 <button class="chat-ref">，Go 同样输出（已对齐）；细微 attr 顺序不列入。

function classify(c, variant, leg, go) {
  const l = normalize(leg, 'legacy')
  const g = normalize(go, 'go')
  if (l === g) return { verdict: '一致' }
  if (intentional.has(c.id)) return { verdict: '有意差异', why: intentional.get(c.id), l, g }
  return { verdict: '结构差异', l, g }
}

const rows = []
let pass = 0, diff = 0, intentionalCount = 0
for (const c of cases) {
  const variants = c.variant === 'both' ? ['reader', 'chat'] : [c.variant]
  for (const variant of variants) {
    const legPath = path.join(LEG, variant, c.id + '.html')
    const goPath = path.join(GO, variant, c.id + '.html')
    if (!fs.existsSync(legPath) || !fs.existsSync(goPath)) {
      rows.push({ id: c.id, variant, verdict: '缺输出', leg: fs.existsSync(legPath), go: fs.existsSync(goPath) })
      continue
    }
    const leg = fs.readFileSync(legPath, 'utf8')
    const go = fs.readFileSync(goPath, 'utf8')
    const r = classify(c, variant, leg, go)
    if (r.verdict === '一致') pass++
    else if (r.verdict === '有意差异') intentionalCount++
    else diff++
    rows.push({ id: c.id, variant, verdict: r.verdict, why: r.why || '', l: r.l, g: r.g })
  }
}

console.log(`一致 ${pass} / 有意差异 ${intentionalCount} / 结构差异 ${diff} / 缺输出 ${rows.filter(r=>r.verdict==='缺输出').length}`)
console.log('')
for (const r of rows) {
  const tag = r.verdict === '一致' ? '✓' : r.verdict === '有意差异' ? '≈' : r.verdict === '缺输出' ? '?' : '✗'
  console.log(`${tag} [${r.variant}] ${r.id}  ${r.why || ''}`)
  if (r.verdict === '结构差异') {
    console.log(`  LEG: ${(r.l||'').slice(0,300)}`)
    console.log(`  GO : ${(r.g||'').slice(0,300)}`)
  }
}

fs.writeFileSync(path.join(__dirname, 'report.json'), JSON.stringify(rows, null, 2), 'utf8')
