// Settings: bindings, lp-select, pause button, tabs, hotkey capture, reapply video.

import { lp, call } from '/scripts/store.js'
import { status, debounce, track } from '/scripts/ui.js'

// ── Window theme ───────────────────────────────────────────────────────────────

export function applyWindowThemeCss(theme) {
  document.documentElement.dataset.windowTheme = theme || 'mica'
}

// ── Toggle helper ──────────────────────────────────────────────────────────────

export function setToggle(el, on) {
  el.classList.toggle('on', !!on)
  el.setAttribute('aria-checked', on ? 'true' : 'false')
}

const saveSettingsDebounced = debounce(() => {
  if (lp.appSettings) call('SaveSettings', lp.appSettings).catch(() => {})
}, 250)

// ── Render settings from appSettings ──────────────────────────────────────────

export function renderSettings() {
  if (!lp.appSettings) return
  document
    .querySelectorAll('.lp-toggle[data-setting]')
    .forEach((b) => setToggle(b, lp.appSettings[b.dataset.setting]))
  document.querySelectorAll('.lp-select[data-setting]').forEach((sel) => {
    const val = lp.appSettings[sel.dataset.setting]
    if (val != null) setLpSelectValue(sel, String(val))
  })
  const vram = document.querySelector('input[type=range][data-setting="vramCapMB"]')
  if (vram) {
    vram.value = lp.appSettings.vramCapMB
    const lbl = document.getElementById('vram-cap-label')
    if (lbl) lbl.textContent = `${lp.appSettings.vramCapMB} MB`
  }
  document
    .querySelectorAll('[data-setting="windowTheme"][data-value]')
    .forEach((s) => s.classList.toggle('active', s.dataset.value === lp.appSettings.windowTheme))
  document.querySelectorAll('.lp-kbd[data-hotkey]').forEach((b) => {
    b.textContent = (lp.appSettings.hotkeys && lp.appSettings.hotkeys[b.dataset.hotkey]) || '—'
  })
}

// ── Bind settings controls ─────────────────────────────────────────────────────

export function bindSettingsControls() {
  document.querySelectorAll('.lp-toggle[data-setting]').forEach((btn) => {
    btn.addEventListener('click', () => {
      const key = btn.dataset.setting
      lp.appSettings[key] = !lp.appSettings[key]
      setToggle(btn, lp.appSettings[key])
      if (key === 'telemetry' && lp.appSettings.telemetry) track('telemetry_enabled')
      saveSettingsDebounced()
      if (key === 'gpuAcceleration') reapplyVideoWallpapers()
    })
  })
  document.querySelectorAll('.lp-select[data-setting]').forEach((sel) => {
    sel.addEventListener('lp:change', (e) => {
      lp.appSettings[sel.dataset.setting] = e.detail.value
      saveSettingsDebounced()
    })
  })
  const vram = document.querySelector('input[type=range][data-setting="vramCapMB"]')
  if (vram) {
    vram.addEventListener('input', () => {
      lp.appSettings.vramCapMB = parseInt(vram.value, 10)
      const lbl = document.getElementById('vram-cap-label')
      if (lbl) lbl.textContent = `${lp.appSettings.vramCapMB} MB`
      saveSettingsDebounced()
    })
  }
  document.querySelectorAll('[data-setting="windowTheme"][data-value]').forEach((seg) => {
    seg.addEventListener('click', () => {
      lp.appSettings.windowTheme = seg.dataset.value
      document
        .querySelectorAll('[data-setting="windowTheme"][data-value]')
        .forEach((s) =>
          s.classList.toggle('active', s.dataset.value === lp.appSettings.windowTheme)
        )
      applyWindowThemeCss(lp.appSettings.windowTheme)
      saveSettingsDebounced()
    })
  })
  document.querySelectorAll('.lp-kbd[data-hotkey]').forEach((btn) => {
    btn.addEventListener('click', () => startHotkeyCapture(btn))
  })
  document.getElementById('pause-wallpaper-btn')?.addEventListener('click', async () => {
    const paused = await call('ToggleVideoPause')
    syncPauseBtn(paused)
  })
}

// ── lp-select helpers ──────────────────────────────────────────────────────────

export function setLpSelectValue(wrapper, value) {
  const valEl = wrapper.querySelector('.lp-select-val')
  const items = wrapper.querySelectorAll('.lp-select-item')
  let label = null
  items.forEach((item) => {
    const sel = item.dataset.value === value
    item.classList.toggle('selected', sel)
    item.setAttribute('aria-selected', sel ? 'true' : 'false')
    if (sel) label = item.textContent.trim()
  })
  if (valEl && label != null) valEl.textContent = label
}

function openLpSelect(wrapper) {
  wrapper.classList.add('open')
  const trigger = wrapper.querySelector('.lp-select-trigger')
  const dropdown = wrapper.querySelector('.lp-select-dropdown')
  if (!trigger || !dropdown) return
  const rect = trigger.getBoundingClientRect()
  dropdown.style.top = rect.bottom + 4 + 'px'
  dropdown.style.left = rect.left + 'px'
  dropdown.style.minWidth = rect.width + 'px'
  dropdown.hidden = false
  trigger.setAttribute('aria-expanded', 'true')
}

function closeLpSelect(wrapper) {
  wrapper.classList.remove('open')
  const dropdown = wrapper.querySelector('.lp-select-dropdown')
  const trigger = wrapper.querySelector('.lp-select-trigger')
  if (dropdown) dropdown.hidden = true
  if (trigger) trigger.setAttribute('aria-expanded', 'false')
}

export function initLpSelect(wrapper) {
  const trigger = wrapper.querySelector('.lp-select-trigger')
  const dropdown = wrapper.querySelector('.lp-select-dropdown')
  if (!trigger || !dropdown) return
  const newTrigger = trigger.cloneNode(true)
  trigger.replaceWith(newTrigger)
  const newDropdown = dropdown.cloneNode(true)
  dropdown.replaceWith(newDropdown)
  newTrigger.addEventListener('click', (e) => {
    e.stopPropagation()
    const isOpen = !newDropdown.hidden
    document.querySelectorAll('.lp-select.open').forEach((el) => {
      if (el !== wrapper) closeLpSelect(el)
    })
    isOpen ? closeLpSelect(wrapper) : openLpSelect(wrapper)
  })
  newDropdown.addEventListener('click', (e) => {
    const item = e.target.closest('.lp-select-item')
    if (!item) return
    setLpSelectValue(wrapper, item.dataset.value)
    closeLpSelect(wrapper)
    wrapper.dispatchEvent(new CustomEvent('lp:change', { detail: { value: item.dataset.value } }))
  })
}

export function initCustomSelects() {
  document.querySelectorAll('.lp-select').forEach((el) => initLpSelect(el))
  document.addEventListener('click', () => {
    document.querySelectorAll('.lp-select.open').forEach((el) => closeLpSelect(el))
  })
}

// ── Pause button state ─────────────────────────────────────────────────────────

export function syncPauseBtn(paused) {
  const btn = document.getElementById('pause-wallpaper-btn')
  if (!btn) return
  btn.classList.toggle('paused', paused)
  btn.setAttribute('aria-pressed', paused ? 'true' : 'false')
  const lbl = document.getElementById('pause-wallpaper-label')
  const ico = document.getElementById('pause-wallpaper-icon')
  if (lbl) lbl.textContent = paused ? 'Resume' : 'Pause'
  if (ico)
    ico.innerHTML = paused
      ? '<polygon points="5 3 19 12 5 21 5 3"></polygon>'
      : '<rect x="6" y="4" width="4" height="16"></rect><rect x="14" y="4" width="4" height="16"></rect>'
}

// ── Settings tab switching ─────────────────────────────────────────────────────

document.querySelectorAll('.settings-tab[data-stab]').forEach((tab) => {
  tab.addEventListener('click', () => {
    const id = tab.dataset.stab
    document.querySelectorAll('.settings-tab[data-stab]').forEach((t) => {
      const on = t === tab
      t.classList.toggle('bg-lp-accent-soft', on)
      t.classList.toggle('bg-transparent', !on)
      t.classList.toggle('text-lp-text', on)
      t.classList.toggle('text-lp-muted', !on)
      t.setAttribute('aria-selected', on ? 'true' : 'false')
      const dot = t.querySelector('.dot')
      if (dot) dot.classList.toggle('hidden', !on)
    })
    document.querySelectorAll('[data-spanel]').forEach((p) => {
      p.hidden = p.dataset.spanel !== id
    })
    if (id === 'billing') lp.fn.onShowBilling?.()
    if (id === 'connections') lp.fn.onShowConnections?.()
  })
})

// ── Hotkey capture ─────────────────────────────────────────────────────────────

function startHotkeyCapture(btn) {
  if (lp.capturing) {
    lp.capturing.btn.classList.remove('capturing')
    renderSettings()
  }
  lp.capturing = { btn, action: btn.dataset.hotkey }
  btn.classList.add('capturing')
  btn.textContent = 'Press keys…'
}

function displayKey(e) {
  const k = e.key
  if (k === 'Control' || k === 'Shift' || k === 'Alt' || k === 'Meta') return null
  if (k === ' ') return 'Space'
  if (k === 'ArrowUp') return 'Up'
  if (k === 'ArrowDown') return 'Down'
  if (k === 'ArrowLeft') return 'Left'
  if (k === 'ArrowRight') return 'Right'
  if (k.length === 1) return k.toUpperCase()
  return k
}

window.addEventListener(
  'keydown',
  (e) => {
    if (!lp.capturing) return
    e.preventDefault()
    e.stopPropagation()
    if (e.key === 'Escape') {
      lp.capturing.btn.classList.remove('capturing')
      lp.capturing = null
      renderSettings()
      return
    }
    const k = displayKey(e)
    if (!k) return
    const parts = []
    if (e.ctrlKey) parts.push('Ctrl')
    if (e.shiftKey) parts.push('Shift')
    if (e.altKey) parts.push('Alt')
    if (e.metaKey) parts.push('Win')
    parts.push(k)
    const combo = parts.join(' + ')
    if (!lp.appSettings.hotkeys) lp.appSettings.hotkeys = {}
    lp.appSettings.hotkeys[lp.capturing.action] = combo
    lp.capturing.btn.textContent = combo
    lp.capturing.btn.classList.remove('capturing')
    lp.capturing = null
    call('SaveSettings', lp.appSettings).catch(() => {})
  },
  true
)

// ── Reapply video wallpapers ───────────────────────────────────────────────────

export async function reapplyVideoWallpapers() {
  const list = Object.entries(lp.state)
    .filter(([, s]) => s.filePath && s.ready)
    .map(([idx, s]) => ({ monitorIndex: parseInt(idx, 10), filePath: s.cachedPath || s.filePath }))
  if (!list.length) return
  try {
    await call('SaveSettings', lp.appSettings)
  } catch (_) {}
  status('Reapplying…')
  try {
    await call('ApplyWallpapers', list)
    status(
      lp.appSettings.gpuAcceleration ? 'GPU acceleration on' : 'GPU acceleration off',
      'success',
      2500
    )
  } catch (e) {
    status(`Failed: ${e}`, 'error')
  }
}

// ── Init settings ──────────────────────────────────────────────────────────────

export async function initSettings() {
  try {
    lp.appVersion = await call('GetVersion')
  } catch (_) {}
  try {
    lp.appSettings = await call('GetSettings')
  } catch (_) {
    lp.appSettings = null
  }
  if (!lp.appSettings) return
  applyWindowThemeCss(lp.appSettings.windowTheme)
  try {
    const adapters = await call('GetGPUAdapters')
    const selWrap = document.getElementById('gpu-adapter-select')
    if (selWrap && Array.isArray(adapters)) {
      const list = selWrap.querySelector('.lp-select-dropdown')
      if (list) {
        for (const a of adapters) {
          const btn = document.createElement('button')
          btn.type = 'button'
          btn.className = 'lp-select-item'
          btn.dataset.value = a
          btn.setAttribute('role', 'option')
          btn.setAttribute('aria-selected', 'false')
          btn.textContent = a
          list.appendChild(btn)
        }
      }
      initLpSelect(selWrap)
      if (lp.appSettings.gpuAdapter != null)
        setLpSelectValue(selWrap, String(lp.appSettings.gpuAdapter))
    }
  } catch (_) {}
  renderSettings()
  bindSettingsControls()
  initCustomSelects()
  lp.fn.refreshAuthDependentUI?.()
  track('app_open')
}

// Register in cross-module registry
lp.fn.renderSettings = renderSettings
lp.fn.syncPauseBtn = syncPauseBtn
lp.fn.initSettings = initSettings
lp.fn.reapplyVideoWallpapers = reapplyVideoWallpapers
lp.fn.initLpSelect = initLpSelect
lp.fn.setLpSelectValue = setLpSelectValue
