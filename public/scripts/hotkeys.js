// Hotkeys: Go events, GPU stats polling.

import { lp, call, onEvent } from '/scripts/store.js'
import { status } from '/scripts/ui.js'

// ── Cycle gallery wallpapers ───────────────────────────────────────────────────

async function cycleGallery(dir) {
  const localItems = lp.galleryItems.filter((it) => !it.remote)
  if (!localItems.length) { status('No local wallpapers to cycle', 'error'); return }
  lp.galleryCursor = (lp.galleryCursor + dir + localItems.length) % localItems.length
  const entry = localItems[lp.galleryCursor]
  const target = lp.monitors.find((m) => m.primary) || lp.monitors[0]
  if (!target || !entry) return
  status('Applying…')
  try {
    let cached = entry.cachedPath || entry.filePath
    const isNonGifVideo = entry.isVideo && lp.fn.extOf?.(entry.filePath) !== 'gif'
    if (isNonGifVideo && (entry.width !== target.width || entry.height !== target.height)) {
      cached = await call('PreprocessVideo', entry.filePath, target.width, target.height)
    }
    await call('ApplyWallpapers', [{ monitorIndex: target.index, filePath: cached }])
    lp.state[target.index] = { filePath: entry.filePath, cachedPath: cached, isVideo: entry.isVideo, ready: true, thumbnail: entry.thumbnail }
    lp.fn.commitApply?.()
    lp.fn.applyThumb?.(target.index, entry.thumbnail, entry.filePath, entry.isVideo)
    lp.fn.refreshApply?.()
    status('Applied!', 'success', 2000)
  } catch (e) { status(`Failed: ${e}`, 'error') }
}

// ── Wails events ───────────────────────────────────────────────────────────────

onEvent('hotkey:next', () => cycleGallery(1))
onEvent('hotkey:prev', () => cycleGallery(-1))
onEvent('hotkey:open', () => lp.fn.showView?.('displays'))
onEvent('video:paused', (evt) => {
  const p = evt && evt.data && evt.data.paused
  lp.fn.syncPauseBtn?.(p)
  status(p ? 'Paused' : 'Resumed', '', 1500)
})

// ── GPU stats polling ──────────────────────────────────────────────────────────

const gpuEl = document.getElementById('gpu-stats')
const gpuSep = document.getElementById('gpu-sep')

async function updateGPUStats() {
  if (!gpuEl) return
  try {
    const s = await call('GetGPUStats')
    if (!s) return
    const pct = Math.round(s.usagePct)
    const vram = s.usedMB >= 1024 ? `${(s.usedMB / 1024).toFixed(1)} GB` : `${s.usedMB} MB`
    gpuEl.textContent = `GPU · ${pct}% · ${vram} VRAM`
    gpuEl.classList.remove('hidden')
    if (gpuSep) gpuSep.classList.remove('hidden')
  } catch (_) {
    gpuEl.classList.add('hidden')
    if (gpuSep) gpuSep.classList.add('hidden')
  }
}

setTimeout(() => { updateGPUStats(); setInterval(updateGPUStats, 2000) }, 1500)

// ── Telemetry on apply ─────────────────────────────────────────────────────────

document.getElementById('apply-btn')?.addEventListener('click', () => lp.fn.track?.('wallpaper_applied'))
