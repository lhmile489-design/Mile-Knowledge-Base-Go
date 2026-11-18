// 用旧版 node_modules 的 react-markdown + remark-gfm 渲染对照集。
// 行为对齐旧版 MarkdownCore / ChatMarkdown：
//   PLUGINS = [remarkGfm, remarkWikilinks]（readerLinks.ts splitWikilinks 逻辑在此内联）
//   chat 追加 remarkChatReferences（findReferences 逻辑内联）
//   BASE: a→CoreAnchor 三分类（classifyHref 内联）、input→checkbox disabled
//   READER: img→presentImage 分类（classifyImageSource 内联；vault 模拟 loading→blocked 输出结构按契约映射）
//   CHAT: img→img-label、table→md-table-wrap、pre→codeblock 工具条、code→chat-ref
//   流式：chatStreaming.trimStreamingText 逻辑内联
// 输出：legacy-out/<variant>/<id>.html
const path = require('path')
const fs = require('fs')

const NM = 'E:/WebApp/Mile-Project/Mile-Knowledge-Base-master/node_modules'
const React = require(path.join(NM, 'react'))
const { renderToStaticMarkup } = require(path.join(NM, 'react-dom/server'))
const { createElement: h } = React

const ReactMarkdown = require(path.join(NM, 'react-markdown')).default
const remarkGfm = require(path.join(NM, 'remark-gfm')).default
const { cases } = require('../cases/cases.cjs')

// ---------- 内联旧版逻辑（来源：src/shared/readerLinks.ts、imageSource.ts、chatStreaming.ts、chatReferences.ts） ----------

const SCHEME_RE = /^[a-z][a-z0-9+.-]*:/i
const HTTP_RE = /^https?:\/\//i
const DRIVE_RE = /^[a-z]:[\\/]/i

function safeDecode(v) { try { return decodeURIComponent(v) } catch { return v } }

// readerLinks.ts:36 classifyHref
function classifyHref(raw) {
  const href = raw.trim()
  if (href === '') return { kind: 'invalid', target: '' }
  if (href.startsWith('#')) return { kind: 'anchor', target: href.slice(1) }
  if (HTTP_RE.test(href)) return { kind: 'external', target: href }
  if (SCHEME_RE.test(href)) return { kind: 'invalid', target: href }
  const withoutFragment = href.split('#')[0].split('?')[0]
  const decoded = safeDecode(withoutFragment).replace(/\\/g, '/')
  if (decoded === '') return { kind: 'anchor', target: href }
  if (decoded.startsWith('/') || DRIVE_RE.test(decoded)) return { kind: 'invalid', target: href }
  if (decoded.split('/').some((s) => s === '..')) return { kind: 'invalid', target: href }
  const target = decoded.startsWith('./') ? decoded.slice(2) : decoded
  return { kind: 'internal-path', target }
}

const WIKI_RE = /\[\[([^\]\n|]+)(?:\|([^\]\n]+))?\]\]/g
function splitWikilinks(text) {
  const pieces = []
  let cursor = 0
  let match
  const re = new RegExp(WIKI_RE.source, WIKI_RE.flags)
  while ((match = re.exec(text)) !== null) {
    const target = match[1].trim()
    if (target === '') continue
    if (match.index > cursor) pieces.push({ text: text.slice(cursor, match.index) })
    pieces.push({ wiki: { target, label: (match[2] ?? target).trim() } })
    cursor = match.index + match[0].length
  }
  if (cursor < text.length) pieces.push({ text: text.slice(cursor) })
  return pieces
}

// remarkWikilinks.ts：text 节点切 wikilink → link 节点带 data-wiki
function remarkWikilinks() {
  return (tree) => transformWiki(tree, false)
}
function transformWiki(node, insideLink) {
  if (!Array.isArray(node.children)) return
  const next = []
  for (const child of node.children) {
    const isLink = child.type === 'link' || insideLink
    if (child.type === 'text' && typeof child.value === 'string' && !isLink) {
      const pieces = splitWikilinks(child.value)
      if (pieces.length === 1 && 'text' in pieces[0]) { next.push(child); continue }
      for (const piece of pieces) {
        if ('text' in piece) { next.push({ type: 'text', value: piece.text }); continue }
        next.push({
          type: 'link', url: '#', title: null,
          children: [{ type: 'text', value: piece.wiki.label }],
          data: { hProperties: { className: ['wiki'], 'data-wiki': piece.wiki.target } }
        })
      }
      continue
    }
    if (Array.isArray(child.children)) transformWiki(child, isLink)
    next.push(child)
  }
  node.children = next
}

// recentDocuments.ts:35 isPlausibleRelativePath
function isPlausibleRelativePath(value) {
  if (typeof value !== 'string') return false
  if (value.length === 0 || value.length > 1024) return false
  if (/^[\\/]/.test(value) || /^[A-Za-z]:/.test(value)) return false
  return value.split(/[\\/]+/).every((s) => s !== '.' && s !== '..')
}

// chatReferences.ts：REF_EXTENSIONS + PATH_TOKEN + findReferences
const REF_EXTENSIONS = ['md', 'markdown', 'txt']
const PATH_TOKEN = new RegExp(
  String.raw`(?<![\w/.\\:@-])((?:[\w一-鿿.-]+/)*[\w一-鿿.-]+\.(?:${REF_EXTENSIONS.join('|')}))(?![\w/-])`,
  'gi'
)
const WIKI_TOKEN = /\[\[([^[\]|\n]+)(?:\|([^[\]\n]+))?\]\]/g

function asPathReference(text) {
  const candidate = text.trim()
  if (candidate === '' || candidate !== text) return null
  const re = new RegExp(`^${PATH_TOKEN.source}$`, 'i')
  if (!re.test(candidate)) return null
  return isPlausibleRelativePath(candidate) ? candidate : null
}

function findReferences(text) {
  const found = []
  for (const match of text.matchAll(new RegExp(WIKI_TOKEN.source, 'g'))) {
    const target = match[1].trim()
    if (target === '') continue
    const start = match.index ?? 0
    found.push({ start, end: start + match[0].length, refKind: 'wiki', target, label: (match[2] ?? target).trim() || target })
  }
  for (const match of text.matchAll(new RegExp(PATH_TOKEN.source, 'gi'))) {
    const start = match.index ?? 0
    const end = start + match[0].length
    if (found.some((h2) => start < h2.end && end > h2.start)) continue
    if (!isPlausibleRelativePath(match[1])) continue
    found.push({ start, end, refKind: 'path', target: match[1], label: match[1] })
  }
  return found.sort((a, b) => a.start - b.start)
}

// remarkChatReferences.ts
function remarkChatReferences() {
  return (tree) => transformRefs(tree, false)
}
function transformRefs(node, insideLink) {
  if (!Array.isArray(node.children)) return
  const next = []
  for (const child of node.children) {
    const isLink = child.type === 'link' || insideLink
    if (child.type === 'text' && typeof child.value === 'string' && !isLink) {
      const refs = findReferences(child.value)
      if (refs.length === 0) { next.push(child); continue }
      let cursor = 0
      for (const ref of refs) {
        if (ref.start > cursor) next.push({ type: 'text', value: child.value.slice(cursor, ref.start) })
        next.push({
          type: 'link', url: '#', title: null,
          children: [{ type: 'text', value: ref.label }],
          data: { hProperties: { className: ['chat-ref-link'], 'data-ref-kind': ref.refKind, 'data-ref-target': ref.target } }
        })
        cursor = ref.end
      }
      if (cursor < child.value.length) next.push({ type: 'text', value: child.value.slice(cursor) })
      continue
    }
    if (Array.isArray(child.children)) transformRefs(child, isLink)
    next.push(child)
  }
  node.children = next
}

// imageSource.ts classifyImageSource（简化：足够用于判定 kind）
const IMAGE_EXTENSIONS = ['png','jpg','jpeg','gif','webp','svg','avif','bmp','ico']
const MIME_BY_EXT = { png:'image/png', jpg:'image/jpeg', jpeg:'image/jpeg', gif:'image/gif', webp:'image/webp', svg:'image/svg+xml', avif:'image/avif', bmp:'image/bmp', ico:'image/x-icon' }
const ALLOWED_DATA_MIMES = new Set(Object.values(MIME_BY_EXT))
const MAX_SRC_LENGTH = 12 * 1024 * 1024

function stripTitle(src) {
  const m = /^(\S*)\s+(?:"[^"]*"|'[^']*'|\([^)]*\))$/.exec(src)
  return m ? m[1] : src
}
function decodePathSegments(value) {
  try {
    return value.split(/([\\/]+)/).map((p) => (/^[\\/]+$/.test(p) ? p : decodeURIComponent(p))).join('')
  } catch { return null }
}
function extOf(b) {
  const d = b.lastIndexOf('.')
  if (d <= 0 || d === b.length - 1) return null
  const e = b.slice(d + 1).toLowerCase()
  return IMAGE_EXTENSIONS.includes(e) ? e : null
}
function classifyImageSource(raw) {
  if (typeof raw !== 'string') return { kind: 'invalid', reason: 'empty' }
  if (raw.length > MAX_SRC_LENGTH) return { kind: 'invalid', reason: 'too-long' }
  const src = stripTitle(raw.trim().replace(/^<(.*)>$/, '$1')).trim()
  if (src === '') return { kind: 'invalid', reason: 'empty' }
  if (src.includes('\0')) return { kind: 'invalid', reason: 'path-escape' }
  if (/^data:/i.test(src)) {
    const header = /^data:([^;,]*)[;,]/i.exec(src)
    if (!header) return { kind: 'invalid', reason: 'not-an-image' }
    const mime = header[1].trim().toLowerCase()
    if (!ALLOWED_DATA_MIMES.has(mime)) return { kind: 'invalid', reason: 'not-an-image' }
    return { kind: 'data', src }
  }
  const scheme = /^([A-Za-z][A-Za-z0-9+.-]*):/.exec(src)
  if (scheme) {
    const protocol = scheme[1].toLowerCase()
    if (protocol !== 'http' && protocol !== 'https') return { kind: 'invalid', reason: 'unsupported-scheme' }
    let host = ''
    try { host = new URL(src).host } catch { host = '' }
    if (host === '') return { kind: 'invalid', reason: 'unsupported-scheme' }
    return { kind: 'remote', src, host, secure: protocol === 'https' }
  }
  if (src.startsWith('//')) {
    let host = ''
    try { host = new URL(`https:${src}`).host } catch { host = '' }
    if (host === '') return { kind: 'invalid', reason: 'unsupported-scheme' }
    return { kind: 'remote', src: `https:${src}`, host, secure: true }
  }
  const clean = src.split(/[?#]/)[0]
  if (clean === '') return { kind: 'invalid', reason: 'empty' }
  if (/^[\\/]{2}/.test(clean)) return { kind: 'invalid', reason: 'path-escape' }
  if (/^[\\/]/.test(clean)) return { kind: 'invalid', reason: 'path-escape' }
  if (/^[A-Za-z]:/.test(clean)) return { kind: 'invalid', reason: 'path-escape' }
  const decoded = decodePathSegments(clean)
  if (decoded === null) return { kind: 'invalid', reason: 'path-escape' }
  const segments = decoded.split(/[\\/]+/).filter((s) => s.length > 0)
  if (segments.length === 0) return { kind: 'invalid', reason: 'empty' }
  for (const s of segments) if (s === '.' || s === '..') return { kind: 'invalid', reason: 'path-escape' }
  const extension = extOf(segments[segments.length - 1])
  if (extension === null) return { kind: 'invalid', reason: 'unsupported-extension' }
  return { kind: 'vault', path: segments.join('/'), extension }
}

// chatStreaming.ts trimStreamingText
function lastOpenFence(lines, beforeIndex) {
  let open = -1
  for (let i = 0; i < beforeIndex; i += 1) {
    const t = lines[i].trim()
    if (!/^(```|~~~)/.test(t)) continue
    if (open === -1) { open = i } else {
      const prev = lines[open].trim().slice(0, 3)
      const cur = t.slice(0, 3)
      if (prev === cur) open = -1
    }
  }
  return open
}
function trimStreamingText(text, streaming) {
  if (!streaming || text === '') return { text, pending: false, pendingKind: null }
  const lines = text.split('\n')
  let tableEnd = -1, tableStart = -1
  for (let i = lines.length - 1; i >= 0; i -= 1) {
    const t = lines[i].trim()
    if (t.startsWith('|')) { if (tableEnd === -1) tableEnd = i; tableStart = i; continue }
    if (t === '') continue
    break
  }
  if (tableEnd !== -1) {
    if (lastOpenFence(lines, tableStart) !== -1) return { text, pending: false, pendingKind: null }
    const block = lines.slice(tableStart, tableEnd + 1)
    const sepIndex = block.findIndex((l) => /^\s*\|[\s\-:|]+\|\s*$/.test(l))
    if (sepIndex === -1) {
      const kept = lines.slice(0, tableStart).join('\n').replace(/\n+$/, '')
      return { text: kept, pending: true, pendingKind: 'table' }
    }
    const lastLine = lines[tableEnd].trim()
    if (!lastLine.endsWith('|')) {
      const kept = lines.slice(0, tableEnd).join('\n').replace(/\n+$/, '')
      return { text: kept, pending: true, pendingKind: 'table' }
    }
  }
  const tail = text.slice(-80)
  const openLink = tail.match(/\[([^\]]+)\]\([^)]*$/)
  if (openLink) {
    const idx = text.lastIndexOf(openLink[0])
    const before = text.slice(0, idx)
    const lineNo = before.split('\n').length - 1
    if (lastOpenFence(text.split('\n'), lineNo) === -1) {
      const kept = text.slice(0, idx) + openLink[1]
      return { text: kept, pending: true, pendingKind: 'link' }
    }
  }
  return { text, pending: false, pendingKind: null }
}
function streamPendingLabel(kind) {
  if (kind === 'table') return '正在生成表格…'
  if (kind === 'link') return '正在生成链接…'
  return ''
}

// ---------- React 组件：与旧版结构对齐 ----------

// vault 解析 stub：case.vault.resolve 决定 resolved；ambiguous 列表决定多同名
function makeVault(caseVault) {
  const v = caseVault || {}
  const files = new Set(v.files || [])
  const resolveMap = v.resolve || {}
  const ambiguousSet = new Set(v.ambiguous || [])
  return {
    resolveWiki(target) {
      if (ambiguousSet.has(target)) return { status: 'ambiguous' }
      const t = resolveMap[target] ?? target
      const cand = t.endsWith('.md') || t.endsWith('.txt') || t.endsWith('.markdown') ? t : `${t}.md`
      if (resolveMap[target] || files.has(cand) || files.has(t)) return { status: 'resolved', path: cand }
      return { status: 'missing' }
    },
    exists(p) { return files.has(p) }
  }
}

// CoreAnchor（MarkdownCore.tsx:44-96）：wikilink 三态 + 链接三分类
function CoreAnchor({ href, wiki, children, vault }) {
  const link = wiki ? { kind: 'internal-path', target: wiki } : classifyHref(href ?? '')
  if (link.kind === 'external') return h('a', { href: link.target, target: '_blank', rel: 'noreferrer', className: 'link-external' }, children)
  if (link.kind === 'anchor') return h('a', { href: `#${link.target}`, className: 'link-anchor' }, children)
  if (link.kind === 'invalid') return h('span', { className: 'link-plain' }, children)
  // internal-path / wiki
  let status = 'ok'
  let resolvedTarget = link.target
  if (wiki) {
    const r = vault.resolveWiki(wiki)
    if (r.status === 'ambiguous') status = 'ambiguous'
    else if (r.status === 'missing') status = 'missing'
    else resolvedTarget = r.path
  } else {
    if (!vault.exists(link.target)) status = 'missing'
  }
  const ambiguous = status === 'ambiguous'
  const missing = status === 'missing' || ambiguous
  return h('a', {
    href: '#',
    className: missing ? 'wiki wiki-missing' : 'wiki',
    'data-wiki': wiki ?? undefined,
    'data-status': ambiguous ? 'ambiguous' : missing ? 'missing' : 'ok',
    'data-target': resolvedTarget,
    title: ambiguous ? '有多个同名文件' : wiki ? `[[${wiki}]]` : link.target
  }, children, missing ? h('span', { className: 'wiki-note' }, ambiguous ? '（有多个同名）' : '（未创建）') : null)
}

// ChatReference（ChatReference.tsx）：button.chat-ref
function ChatReference({ refKind, target, label, status = 'ok' }) {
  const note = status === 'missing' ? '找不到' : status === 'ambiguous' ? '有多个同名' : null
  return h('button', {
    type: 'button', className: 'chat-ref', 'aria-label': `打开 ${target}`,
    title: refKind === 'path' ? target : `[[${target}]]`,
    'data-ref-kind': refKind, 'data-ref-target': target, 'data-status': status
  }, label, note ? h('span', { className: 'chat-ref-note' }, note) : null)
}

// reader img → PreviewImage 结构（PreviewImage.tsx），vault 图片静态无法取字节 → 按 loading 卡
function ReaderImage({ src, alt }) {
  const source = classifyImageSource(src)
  const caption = alt.trim() === '' ? '未命名图片' : alt.trim()
  if (source.kind === 'data') return h('img', { className: 'mile-preview-image', 'data-state': 'show', 'data-img-kind': 'data', alt, src: source.src })
  if (source.kind === 'vault') return h('span', { className: 'mile-preview-image-card', 'data-state': 'loading', 'data-img-kind': 'vault', 'data-vault-path': source.path },
    h('strong', null, caption), h('span', null, '正在读取图片'))
  if (source.kind === 'remote' && !source.secure) return h('span', { className: 'mile-preview-image-card', 'data-state': 'blocked', 'data-img-kind': 'remote', 'data-secure': 'false' },
    h('strong', null, caption), h('span', null, '不支持明文 HTTP 图片（可能被中间人替换），请改用 https 链接'))
  if (source.kind === 'remote') return h('span', { className: 'mile-preview-image-card', 'data-state': 'remote-gate', 'data-img-kind': 'remote', 'data-src': source.src, 'data-host': source.host, 'data-secure': 'true' },
    h('strong', null, caption), h('span', null, '默认不加载：请求会让对方服务器知道你此刻读了这篇笔记'),
    h('code', null, source.host), h('button', { type: 'button', className: 'btn-quiet' }, '加载这张图片'))
  const REASONS = { empty: '图片地址为空', 'too-long': '图片地址过长', 'unsupported-scheme': '不支持这种地址协议', 'not-an-image': '内联数据不是图片', 'path-escape': '路径超出知识库范围', 'unsupported-extension': '不是支持的图片格式' }
  return h('span', { className: 'mile-preview-image-card', 'data-state': 'blocked', 'data-img-kind': 'invalid', 'data-reason': REASONS[source.reason] || source.reason },
    h('strong', null, caption), h('span', null, REASONS[source.reason] || ''))
}

// chat img → ImageLabel（ImageLabel.tsx）
function ChatImage({ src, alt }) {
  const caption = alt.trim() === '' ? '未命名图片' : alt.trim()
  let vault = null
  if (!/^[A-Za-z][A-Za-z0-9+.-]*:/.test(src) && !src.startsWith('//') && isPlausibleRelativePath(src)) vault = src
  if (vault === null) return h('span', { className: 'img-label' }, `图片：${caption}`, h('span', { className: 'img-note' }, '（远程图片不加载）'))
  return h('button', { type: 'button', className: 'img-label vault', 'data-vault': vault, title: vault, 'aria-label': `打开 ${vault}` },
    `图片：${caption}`, h('span', { className: 'img-open' }, `打开 ${vault}`))
}

// chat pre → CodeBlock 结构（CodeBlock.tsx）
function ChatPre({ children }) {
  let lang = '', codeText = ''
  const kids = React.Children.toArray(children)
  const el = kids.find((k) => React.isValidElement(k))
  if (el && el.props) {
    const cls = typeof el.props.className === 'string' ? el.props.className : ''
    const m = cls.match(/language-([A-Za-z0-9_+-]+)/)
    if (m) lang = m[1]
    const inner = el.props.children
    codeText = typeof inner === 'string' ? inner : Array.isArray(inner) ? inner.filter((c) => typeof c === 'string').join('') : ''
  }
  return h('div', { className: 'codeblock', 'data-lang': lang },
    h('div', { className: 'tools' },
      lang !== '' ? h('span', { className: 'lang' }, lang) : null,
      h('button', { type: 'button', className: 'copy', 'aria-label': '复制代码', 'data-code': codeText }, '复制')),
    h('pre', null, children))
}

// markdownUrlTransform.ts
const { defaultUrlTransform } = require(path.join(NM, 'react-markdown'))
function markdownUrlTransform(url) {
  if (/^data:image\//i.test(url)) return url
  return defaultUrlTransform(url)
}
function chatUrlTransform(url) {
  if (/^data:/i.test(url)) return ''
  return defaultUrlTransform(url)
}

// ---------- components 表 ----------

function baseComponents(vault) {
  return {
    a({ href, children, node, ...rest }) {
      const wiki = rest['data-wiki'] ?? rest['dataWiki']
      return h(CoreAnchor, { href, wiki: typeof wiki === 'string' ? wiki : undefined, vault }, children)
    },
    input({ type, checked, node, ...rest }) {
      if (type !== 'checkbox') return h('input', { type, ...rest })
      return h('input', { type: 'checkbox', checked: Boolean(checked), disabled: true, readOnly: true })
    }
  }
}

function readerComponents(vault) {
  return {
    ...baseComponents(vault),
    img({ src, alt, node }) {
      return h(ReaderImage, { src: typeof src === 'string' ? src : '', alt: alt ?? '' })
    }
  }
}

function chatComponents(vault) {
  return {
    ...baseComponents(vault),
    a({ href, children, node, ...rest }) {
      const wiki = rest['data-wiki']
      const refKind = rest['data-ref-kind']
      const refTarget = rest['data-ref-target']
      if ((refKind === 'path' || refKind === 'wiki') && typeof refTarget === 'string') {
        const label = typeof children === 'string' ? children : refTarget
        const status = vault.exists(refTarget) || vault.resolveWiki(refTarget).status === 'resolved' ? 'ok' : 'missing'
        return h(ChatReference, { refKind, target: refTarget, label, status })
      }
      if (typeof wiki === 'string' && wiki !== '') {
        const label = typeof children === 'string' ? children : wiki
        const r = vault.resolveWiki(wiki)
        return h(ChatReference, { refKind: 'wiki', target: wiki, label, status: r.status === 'resolved' ? 'ok' : r.status })
      }
      // 外链：CoreAnchor + 外链图标（用 data-ext 标记）
      const inner = React.Children.toArray(children)
      return h(CoreAnchor, { href, vault },
        ...inner,
        h('svg', { className: 'ext', 'data-ext': 'true', 'aria-hidden': 'true', width: 11, height: 11 }))
    },
    code({ children, className, node, ...rest }) {
      const text = typeof children === 'string' ? children : ''
      if (className === undefined && !/\n/.test(text)) {
        const asPath = asPathReference(text)
        if (asPath !== null) {
          return h(ChatReference, { refKind: 'path', target: asPath, label: asPath, status: vault.exists(asPath) ? 'ok' : 'missing' })
        }
        return h('code', { className: 'chat-code' }, children)
      }
      return h('code', { className, ...rest }, children)
    },
    img({ src, alt, node }) {
      return h(ChatImage, { src: typeof src === 'string' ? src : '', alt: alt ?? '' })
    },
    table({ children, node }) {
      return h('div', { className: 'md-table-wrap' }, h('table', null, children))
    },
    pre({ children, node }) {
      return h(ChatPre, null, children)
    }
  }
}

// ---------- 主流程 ----------

async function main() {
  const outDir = path.join(__dirname, '..', 'legacy-out')
  const manifest = []
  for (const variant of ['reader', 'chat']) {
    const dir = path.join(outDir, variant)
    fs.mkdirSync(dir, { recursive: true })
  }
  for (const c of cases) {
    const variants = c.variant === 'both' ? ['reader', 'chat'] : [c.variant]
    for (const variant of variants) {
      const vault = makeVault(c.vault)
      let text = c.md
      let pending = false, pendingKind = null
      if (variant === 'chat' && c.streaming) {
        const r = trimStreamingText(c.md, true)
        text = r.text; pending = r.pending; pendingKind = r.pendingKind
      }
      const plugins = variant === 'chat' ? [remarkGfm, remarkWikilinks, remarkChatReferences] : [remarkGfm, remarkWikilinks]
      const components = variant === 'chat' ? chatComponents(vault) : readerComponents(vault)
      const urlTransform = variant === 'chat' ? chatUrlTransform : markdownUrlTransform
      let body = ''
      if (text.trim() !== '') {
        body = renderToStaticMarkup(
          h(ReactMarkdown, { remarkPlugins: plugins, components, urlTransform }, text)
        )
      }
      if (pending) {
        body += renderToStaticMarkup(
          h('div', { className: 'stream-pending', 'data-kind': pendingKind, role: 'status' }, streamPendingLabel(pendingKind))
        )
      }
      const file = path.join(outDir, variant, `${c.id}.html`)
      fs.writeFileSync(file, body, 'utf8')
      manifest.push({ id: c.id, variant, streaming: Boolean(c.streaming), pendingKind, bytes: body.length })
    }
  }
  fs.writeFileSync(path.join(outDir, 'manifest.json'), JSON.stringify(manifest, null, 2), 'utf8')
  console.log(`rendered ${manifest.length} outputs`)
}

main().catch((e) => { console.error(e); process.exit(1) })
