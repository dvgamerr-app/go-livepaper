// Library: local wallpaper grid (view only), monitor picker.

import { lp, call } from '/scripts/store.js'
import { status, escapeHtml, extOf, showView } from '/scripts/ui.js'
import { loadGalleryItems } from '/scripts/db.js'

// ── Render library grid ────────────────────────────────────────────────────────

export async function renderLibrary() {
  const grid = document.getElementById('library-grid')
  const countEl = document.getElementById('library-count')
  if (!grid) return
  const items = await loadGalleryItems()
  if (items.length === 0) {
    grid.innerHTML =
      '<div class="lib-empty">No wallpapers yet — apply one from Displays to add it here</div>'
    if (countEl) countEl.textContent = 'Recently applied wallpapers'
    return
  }
  grid.innerHTML = ''
  items.forEach((entry, index) => {
    const card = document.createElement('div')
    card.className = 'lib-card'
    const ext = entry.isVideo ? (extOf(entry.filePath || '') === 'gif' ? 'gif' : 'video') : 'image'
    const name =
      (entry.filePath || '')
        .replace(/\\/g, '/')
        .split('/')
        .pop()
        .replace(/\.[^.]+$/, '') || 'Wallpaper'
    card.innerHTML = `
      <img src="${entry.thumbnail || ''}" alt="${escapeHtml(name)}" loading="lazy" draggable="false">
      <span class="lib-card-type ${ext}">${ext.toUpperCase()}</span>
      <div class="lib-card-overlay">
        <span class="lib-card-name">${escapeHtml(name)}</span>
      </div>`
    card.addEventListener('click', () => lp.fn.openGalleryPreview?.(items, index))
    grid.appendChild(card)
  })
  const n = items.length
  if (countEl) countEl.textContent = `${n} wallpaper${n !== 1 ? 's' : ''}`
}

// ── Apply local entry to a monitor ────────────────────────────────────────────

export async function applyLocalEntryToMonitor(m, entry) {
  const isNonGifVideo = entry.isVideo && extOf(entry.filePath) !== 'gif'
  let cached = entry.cachedPath || entry.filePath
  if (isNonGifVideo && (entry.width !== m.width || entry.height !== m.height || !entry.width)) {
    status('Encoding…')
    try {
      cached = await call('PreprocessVideo', entry.filePath, m.width, m.height)
    } catch (_) {
      status('Encoding cancelled.')
      return
    }
  }
  status('Applying…')
  try {
    await call('ApplyWallpapers', [{ monitorIndex: m.index, filePath: cached }])
    lp.state[m.index] = {
      filePath: entry.filePath,
      cachedPath: cached,
      isVideo: entry.isVideo,
      ready: true,
      thumbnail: entry.thumbnail,
    }
    lp.fn.commitApply?.()
    showView('displays')
    lp.fn.applyThumb?.(m.index, entry.thumbnail, entry.filePath, entry.isVideo)
    lp.fn.refreshApply?.()
    status('Applied!', 'success', 3000)
  } catch (e) {
    status(`Failed: ${e}`, 'error')
  }
}

// ── Monitor picker for local items ─────────────────────────────────────────────

export function showLibraryMonitorPicker(entry) {
  const modal = document.getElementById('monitor-picker-modal')
  const grid = document.getElementById('monitor-picker-grid')
  if (!modal || !grid) return
  grid.innerHTML = ''
  lp.monitors.forEach((m) => {
    const opt = _buildMonitorOption(m, entry.thumbnail)
    opt.addEventListener('click', async () => {
      modal.hidden = true
      await applyLocalEntryToMonitor(m, entry)
    })
    grid.appendChild(opt)
  })
  modal.hidden = false
}

// ── Build monitor option card ──────────────────────────────────────────────────

export function _buildMonitorOption(m, newThumb) {
  const opt = document.createElement('div')
  opt.className = 'monitor-option'
  const currentThumb = lp.state[m.index]?.thumbnail || ''
  const currentName = lp.state[m.index]?.filePath
    ? lp.state[m.index].filePath
        .replace(/\\/g, '/')
        .split('/')
        .pop()
        .replace(/\.[^.]+$/, '') || 'Current'
    : 'No wallpaper'
  opt.innerHTML = `
    <div class="monitor-option-preview">
      ${currentThumb ? `<img src="${escapeHtml(currentThumb)}" alt="">` : ''}
      ${newThumb ? `<div class="monitor-option-new-preview" style="background-image:url('${escapeHtml(newThumb)}')"></div>` : ''}
    </div>
    <div class="monitor-option-footer">
      <div class="monitor-option-label">Monitor ${m.index + 1}${m.primary ? ' · Primary' : ''}</div>
      <div class="monitor-option-current">${escapeHtml(currentName)}</div>
    </div>`
  return opt
}

// ── Event listeners ────────────────────────────────────────────────────────────

document.getElementById('monitor-picker-cancel')?.addEventListener('click', () => {
  document.getElementById('monitor-picker-modal').hidden = true
})

// Register in cross-module registry
lp.fn.renderLibrary = renderLibrary
lp.fn.applyLocalEntryToMonitor = applyLocalEntryToMonitor
lp.fn.showLibraryMonitorPicker = showLibraryMonitorPicker
