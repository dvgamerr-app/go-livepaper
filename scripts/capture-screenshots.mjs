// Captures the README / GitHub Pages screenshots of the desktop UI.
//
// The real app runs inside Wails, so this script serves the Astro frontend,
// replaces /wails/runtime.js with an in-memory mock of AppService, and drives
// the page with Playwright using the locally installed Chrome (no browser
// download, no network). Wallpaper thumbnails are generated SVGs.
//
//   bun scripts/capture-screenshots.mjs [outDir]
//
// Output defaults to site/screenshots.

import { spawn } from 'node:child_process'
import { mkdir } from 'node:fs/promises'
import { resolve } from 'node:path'
import { chromium } from 'playwright-core'

const PORT = 9245
const BASE = `http://localhost:${PORT}`
const OUT = resolve(process.argv[2] || 'site/screenshots')
const VIEWPORT = { width: 1180, height: 740 }

// ── Generated wallpapers ──────────────────────────────────────────────────────

const WALLPAPERS = {
  'aurora.jpg': { a: '#0b1026', b: '#3b82f6', c: '#34d399', kind: 'mountains' },
  'sunset.jpg': { a: '#2b1055', b: '#f97316', c: '#fde68a', kind: 'sun' },
  'neon-city.mp4': { a: '#0f0c29', b: '#ec4899', c: '#22d3ee', kind: 'city' },
  'ocean.mp4': { a: '#022c43', b: '#0ea5e9', c: '#a5f3fc', kind: 'waves' },
  'forest.jpg': { a: '#052e16', b: '#22c55e', c: '#bef264', kind: 'mountains' },
  'rain.gif': { a: '#111827', b: '#6366f1', c: '#c7d2fe', kind: 'waves' },
  'desert.jpg': { a: '#451a03', b: '#f59e0b', c: '#fef3c7', kind: 'sun' },
  'galaxy.mp4': { a: '#020617', b: '#7c3aed', c: '#f0abfc', kind: 'stars' },
}

function art(name, w = 640, h = 360) {
  const p = WALLPAPERS[name]
  const stars = Array.from({ length: 60 }, (_, i) => {
    const x = (i * 97) % w
    const y = (i * 53) % (h * 0.7)
    return `<circle cx="${x}" cy="${y}" r="${(i % 3) * 0.5 + 0.6}" fill="#fff" opacity="${0.35 + (i % 4) * 0.15}"/>`
  }).join('')
  const shapes = {
    mountains: `<path d="M0 ${h} L0 ${h * 0.7} L${w * 0.2} ${h * 0.45} L${w * 0.38} ${h * 0.68} L${w * 0.58} ${h * 0.38} L${w * 0.8} ${h * 0.66} L${w} ${h * 0.5} L${w} ${h}Z" fill="${p.a}" opacity=".85"/>
      <path d="M0 ${h} L0 ${h * 0.82} L${w * 0.3} ${h * 0.6} L${w * 0.55} ${h * 0.8} L${w * 0.78} ${h * 0.62} L${w} ${h * 0.8} L${w} ${h}Z" fill="#000" opacity=".5"/>`,
    sun: `<circle cx="${w * 0.5}" cy="${h * 0.62}" r="${h * 0.26}" fill="${p.c}" opacity=".95"/>
      <rect y="${h * 0.7}" width="${w}" height="${h * 0.3}" fill="${p.a}" opacity=".8"/>`,
    city: Array.from({ length: 14 }, (_, i) => {
      const bw = w / 14
      const bh = h * (0.25 + ((i * 37) % 50) / 100)
      return `<rect x="${i * bw + 2}" y="${h - bh}" width="${bw - 4}" height="${bh}" fill="${i % 2 ? p.a : '#000'}" opacity=".9"/>`
    }).join(''),
    waves: [0.55, 0.68, 0.8]
      .map(
        (f, i) =>
          `<path d="M0 ${h * f} Q${w * 0.25} ${h * (f - 0.08)} ${w * 0.5} ${h * f} T${w} ${h * f} V${h} H0Z" fill="${p.c}" opacity="${0.18 + i * 0.12}"/>`
      )
      .join(''),
    stars: `${stars}<circle cx="${w * 0.7}" cy="${h * 0.4}" r="${h * 0.18}" fill="${p.c}" opacity=".25"/>`,
  }
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${w} ${h}" preserveAspectRatio="xMidYMid slice">
    <defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="${p.a}"/><stop offset=".55" stop-color="${p.b}"/><stop offset="1" stop-color="${p.c}"/></linearGradient></defs>
    <rect width="${w}" height="${h}" fill="url(#g)"/>${p.kind !== 'stars' ? '' : ''}${shapes[p.kind]}</svg>`
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`
}

// ── Wails mock ────────────────────────────────────────────────────────────────

const MONITORS = [
  { index: 0, width: 2560, height: 1440, x: 0, y: 0, primary: true },
  { index: 1, width: 1920, height: 1080, x: 2560, y: 360, primary: false },
]

const SETTINGS = {
  gpuAcceleration: true,
  vramCapMB: 256,
  gpuAdapter: '',
  pauseOnGame: true,
  pauseOnBattery: true,
  reduceMotionOnFocus: false,
  windowTheme: 'mica',
  launchAtLogin: true,
  startMinimized: true,
  restoreLastPlaylist: true,
  hotkeys: {
    next: 'Ctrl + Shift + >',
    prev: 'Ctrl + Shift + <',
    playpause: 'Ctrl + Shift + P',
    open: 'Ctrl + Shift + L',
  },
}

const RUNTIME_MOCK = `
const WALLPAPERS = ${JSON.stringify(Object.keys(WALLPAPERS))}
const ART = ${JSON.stringify(Object.fromEntries(Object.keys(WALLPAPERS).map((n) => [n, art(n)])))}
const MONITORS = ${JSON.stringify(MONITORS)}
const SETTINGS = ${JSON.stringify(SETTINGS)}
const nameOf = (p) => String(p).replace(/\\\\/g, '/').split('/').pop()

export const Events = { On() {}, Emit() {}, Off() {} }
export const Call = {
  async ByName(name, ...a) {
    switch (name.split('.').pop()) {
      case 'GetMonitorLayout': {
        const [aw, ah, label] = a
        const minX = Math.min(...MONITORS.map((m) => m.x))
        const minY = Math.min(...MONITORS.map((m) => m.y))
        const cw = Math.max(...MONITORS.map((m) => m.x + m.width)) - minX
        const ch = Math.max(...MONITORS.map((m) => m.y + m.height)) - minY
        const scale = Math.min(aw / cw, (ah - label) / ch, 1)
        return {
          canvasWidth: cw, canvasHeight: ch, scale,
          stageWidth: Math.round(cw * scale),
          stageHeight: Math.round(ch * scale) + label,
          monitors: MONITORS.map((m) => ({
            ...m,
            previewX: Math.round((m.x - minX) * scale),
            previewY: Math.round((m.y - minY) * scale),
            previewWidth: Math.round(m.width * scale),
            previewHeight: Math.round(m.height * scale),
            aspectRatio: m.width / m.height,
          })),
        }
      }
      case 'GetMonitors': return MONITORS
      case 'GetVersion': return '1.0.0'
      case 'CheckDependencies': return { ffmpeg: true, ffprobe: true, mpv: true }
      case 'GetSettings': return SETTINGS
      case 'GetGPUAdapters': return ['NVIDIA GeForce RTX 4070', 'Intel(R) UHD Graphics']
      case 'GetGPUStats': return { usedMB: 118, totalMB: 12288 }
      case 'GetMonitorThumbnail': return ART[nameOf(a[0])] || ''
      case 'IsVideoFile': return /\\.(mp4|mkv|avi|mov|webm|m4v|flv|gif)$/i.test(a[0])
      case 'FileExists': return true
      case 'ToggleVideoPause': return false
      default: return null
    }
  },
}
`

// ── Driver ────────────────────────────────────────────────────────────────────

async function waitForServer(url, ms = 60000) {
  const end = Date.now() + ms
  while (Date.now() < end) {
    try {
      if ((await fetch(url)).ok) return
    } catch {}
    await new Promise((r) => setTimeout(r, 400))
  }
  throw new Error(`Astro dev server did not start on ${url}`)
}

const server = spawn('bun', ['x', '--bun', 'astro', 'dev', '--port', String(PORT)], {
  stdio: 'ignore',
  shell: true,
})
const stopServer = () =>
  spawn('taskkill', ['/pid', String(server.pid), '/t', '/f'], { stdio: 'ignore' })

try {
  await mkdir(OUT, { recursive: true })
  await waitForServer(BASE)

  const browser = await chromium.launch({ channel: 'chrome' })
  const ctx = await browser.newContext({
    viewport: VIEWPORT,
    deviceScaleFactor: 2,
    colorScheme: 'dark',
  })
  await ctx.route('**/wails/runtime.js', (r) =>
    r.fulfill({ contentType: 'text/javascript', body: RUNTIME_MOCK })
  )
  // Restore the last session: one image per monitor, as the real app would.
  await ctx.addInitScript(() => {
    localStorage.setItem(
      'livepaper_state_v1',
      JSON.stringify({
        0: { filePath: 'C:\\Wallpapers\\aurora.jpg', isVideo: false },
        1: { filePath: 'C:\\Wallpapers\\neon-city.mp4', isVideo: true },
      })
    )
  })

  const page = await ctx.newPage()
  await page.goto(BASE)
  await page.waitForSelector('#mb-0.has-file')

  // Seed the Library (IndexedDB) with generated history, then render it.
  await page.evaluate(
    async ({ names, art }) => {
      const { upsertRecent } = await import('/scripts/db.js')
      for (const [i, n] of names.entries()) {
        await upsertRecent({
          fileKey: `k${i}`,
          filePath: `C:\\Wallpapers\\${n}`,
          thumbnail: art[n],
          isVideo: /\.(mp4|gif)$/.test(n),
          width: 2560,
          height: 1440,
        })
      }
    },
    {
      names: Object.keys(WALLPAPERS),
      art: Object.fromEntries(Object.keys(WALLPAPERS).map((n) => [n, art(n)])),
    }
  )

  await page.reload()
  await page.waitForSelector('#mb-0.has-file')

  const shot = (name) => page.screenshot({ path: `${OUT}/${name}.png` })

  await page.waitForTimeout(400)
  await shot('displays')

  await page.click('.nav-item[data-view="library"]')
  await page.waitForSelector('.lib-card')
  await page.waitForTimeout(300)
  await shot('library')

  await page.click('#tb-settings')
  await page.waitForTimeout(300)
  await shot('settings-general')

  const tabs = await page.$$eval('.settings-tab', (els) => els.map((e) => e.dataset.stab))
  for (const id of tabs.filter((t) => ['performance', 'hotkeys'].includes(t))) {
    await page.click(`.settings-tab[data-stab="${id}"]`)
    await page.waitForTimeout(250)
    await shot(`settings-${id}`)
  }

  await browser.close()
  console.log(`Screenshots written to ${OUT}`)
} finally {
  stopServer()
}
