// UI helpers: status bar, view router, text utilities, telemetry.

import { lp, API_BASE } from '/scripts/store.js'

const contentEl = document.getElementById('content')
const applyBtn = document.getElementById('apply-btn')
const resetBtn = document.getElementById('reset-btn')
const discardBtn = document.getElementById('discard-btn')
const statusEl = document.getElementById('status')
const dotEl = document.getElementById('ab-dot')
const pendingActions = document.getElementById('footer-pending-actions')
const pauseBtn = document.getElementById('pause-wallpaper-btn')

export { contentEl, applyBtn, resetBtn, discardBtn }

// ── Status bar ────────────────────────────────────────────────────────────────

export function status(msg, type, clearAfter) {
  statusEl.textContent = msg || 'Running'
  statusEl.className = 'status' + (type ? ` ${type}` : '')
  if (dotEl) {
    if (type === 'error') dotEl.className = 'ab-dot error'
    else if (type === 'success' || !msg) dotEl.className = 'ab-dot'
    else dotEl.className = 'ab-dot busy'
  }
  if (clearAfter) setTimeout(() => status(''), clearAfter)
}

// ── Footer mode ───────────────────────────────────────────────────────────────

export function setFooterMode(mode) {
  const isPending = mode === 'pending'
  pendingActions?.classList.toggle('hidden', !isPending)
  pendingActions?.classList.toggle('flex', isPending)
  pauseBtn?.classList.toggle('hidden', isPending)
}

// ── Apply button state ────────────────────────────────────────────────────────

export function refreshApply() {
  let anySelected = false
  let anyProcessing = false
  for (const assignment of Object.values(lp.state)) {
    if (!assignment.filePath) continue
    anySelected = true
    if (assignment.isVideo && !assignment.ready) {
      anyProcessing = true
      break
    }
  }
  applyBtn.disabled = !anySelected || anyProcessing
  resetBtn.disabled = !anySelected || anyProcessing
  setFooterMode(lp.pendingChanges && anySelected && !anyProcessing ? 'pending' : 'applied')
}

// ── View router ───────────────────────────────────────────────────────────────

const VIEW_PANELS = {
  displays: 'main-content',
  settings: 'settings-panel',
  discover: 'discover-panel',
  library: 'library-panel',
  storage: 'storage-panel',
}
const viewPanels = Object.entries(VIEW_PANELS).map(([name, id]) => [
  name,
  document.getElementById(id),
])

export function showView(name) {
  if (!VIEW_PANELS[name]) return
  for (const [viewName, el] of viewPanels) {
    if (!el) continue
    const on = viewName === name
    el.classList.toggle('hidden', !on)
    el.classList.toggle('flex', on)
    el.classList.toggle('lp-view', on)
  }
  document.querySelectorAll('.nav-item[data-view]').forEach((b) => {
    b.classList.toggle('active', b.dataset.view === name)
  })
  lp.currentView = name
  if (name === 'discover') lp.fn.onShowDiscover?.()
  if (name === 'library') lp.fn.renderLibrary?.()
  if (name === 'storage') lp.fn.loadStorageWallpapers?.()
}

export function openSettings() {
  showView('settings')
  lp.fn.prefetchConnections?.()
}

export function openSettingsTab(tabId) {
  showView('settings')
  lp.fn.prefetchConnections?.()
  const tab = document.querySelector(`.settings-tab[data-stab="${tabId}"]`)
  if (tab) tab.click()
}

// ── Text utilities ────────────────────────────────────────────────────────────

export function escapeHtml(s) {
  return String(s).replace(
    /[&<>"]/g,
    (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c]
  )
}

export function debounce(fn, ms) {
  let t
  return (...a) => {
    clearTimeout(t)
    t = setTimeout(() => fn(...a), ms)
  }
}

export function extOf(p) {
  return p.replace(/\\/g, '/').split('/').pop().split('.').pop().toLowerCase()
}

export function resolutionBadgeText(w, h) {
  if (!w || !h) return ''
  function gcd(a, b) {
    return b ? gcd(b, a % b) : a
  }
  const g = gcd(w, h)
  let ratio = `${w / g}:${h / g}`
  const r = w / h
  if (Math.abs(r - 16 / 9) < 0.05) ratio = '16:9'
  else if (Math.abs(r - 21 / 9) < 0.05) ratio = '21:9'
  else if (Math.abs(r - 4 / 3) < 0.05) ratio = '4:3'
  const res = w >= 7680 ? '8K' : w >= 3840 ? '4K' : w >= 2560 ? '2K' : `${w}p`
  return `${res} · ${ratio}`
}

// ── Telemetry ─────────────────────────────────────────────────────────────────

export async function track(name, props) {
  if (!lp.appSettings || !lp.appSettings.telemetry) return
  try {
    await fetch(`${API_BASE}/api/telemetry`, {
      method: 'POST',
      headers: lp.fn.authHeaders?.() || { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        appVersion: lp.appVersion,
        events: [{ name, props: props || {}, ts: Date.now() }],
      }),
    })
  } catch (_) {}
}

// ── Event listeners ───────────────────────────────────────────────────────────

document.getElementById('tb-settings')?.addEventListener('click', openSettings)
document.getElementById('settings-back-btn')?.addEventListener('click', () => showView('displays'))
document.querySelectorAll('.nav-item[data-view]').forEach((b) => {
  b.addEventListener('click', () => showView(b.dataset.view))
})

// Register in cross-module registry
lp.fn.status = status
lp.fn.refreshApply = refreshApply
lp.fn.showView = showView
lp.fn.openSettings = openSettings
lp.fn.openSettingsTab = openSettingsTab
lp.fn.escapeHtml = escapeHtml
lp.fn.debounce = debounce
lp.fn.extOf = extOf
lp.fn.resolutionBadgeText = resolutionBadgeText
lp.fn.track = track
