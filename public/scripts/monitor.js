// Monitor display: virtual desktop builder, browse/encode flow, apply/reset, state persistence.

import { lp, call, onEvent, STORAGE_KEY } from '/scripts/store.js'
import { upsertRecent } from '/scripts/db.js'
import { contentEl, applyBtn, resetBtn, discardBtn } from '/scripts/ui.js'
import { status, refreshApply, escapeHtml, extOf } from '/scripts/ui.js'
import { pruneAndRefresh, refreshGallery } from '/scripts/gallery.js'

// ── Monitor viewport ──────────────────────────────────────────────────────────

function getMonitorViewport() {
  const cs = getComputedStyle(contentEl)
  return {
    width: Math.max(
      1,
      Math.floor(contentEl.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight))
    ),
    height: Math.max(
      1,
      Math.floor(contentEl.clientHeight - parseFloat(cs.paddingTop) - parseFloat(cs.paddingBottom))
    ),
    labelHeight: 28,
  }
}

// ── Virtual desktop builder ───────────────────────────────────────────────────

function buildStage(layout) {
  const stage = document.createElement('div')
  stage.className = 'vd-stage'
  stage.style.width = `${layout.stageWidth}px`
  stage.style.height = `${layout.stageHeight}px`

  const grid = document.createElement('div')
  grid.className = 'vd-grid'
  stage.appendChild(grid)

  for (const m of layout.monitors) {
    const wrapper = document.createElement('div')
    wrapper.className = 'mon-wrapper'
    wrapper.style.cssText = `left:${m.previewX}px;top:${m.previewY}px;width:${m.previewWidth}px`

    const block = document.createElement('div')
    block.className = 'monitor-block' + (m.primary ? ' primary-mon' : '')
    block.id = `mb-${m.index}`
    block.style.height = `${m.previewHeight}px`

    block.innerHTML = `
      <div class="mon-inner">
        <img class="mon-bg" id="bg-${m.index}" src="" alt="">
        ${m.primary ? `<div class="mon-primary">Primary</div>` : ''}
        <span class="mon-badge" id="bd-${m.index}"></span>
        <div class="mon-info"><div class="mon-res">${m.width}×${m.height}</div></div>
        <div class="mon-progress" id="mp-${m.index}">
          <div class="prog-ring-wrap">
            <svg class="prog-ring" viewBox="0 0 36 36" aria-hidden="true">
              <circle class="prog-ring-bg" cx="18" cy="18" r="15.9"/>
              <circle class="prog-ring-fill" id="pf-${m.index}" cx="18" cy="18" r="15.9"/>
            </svg>
            <span class="prog-pct" id="pl-${m.index}" aria-live="polite">0%</span>
            <div class="prog-cancel-btn" id="pc-${m.index}" role="button" aria-label="Cancel encoding" tabindex="0">
              <div class="prog-cancel-icon"><span class="prog-cancel-label">Cancel</span></div>
            </div>
          </div>
          <span class="prog-status-label">Encoding…</span>
        </div>
        <div class="mon-hover" aria-hidden="true">
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/>
          </svg>
          Choose File
        </div>
        <div class="mon-drop-overlay" aria-hidden="true">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/>
          </svg>
          Drop to assign
        </div>
      </div>`

    block.addEventListener('click', () => {
      const s = lp.state[m.index]
      if (s && s.isVideo && !s.ready) return
      browse(m.index, m.width, m.height)
    })

    const cancelBtn = block.querySelector(`#pc-${m.index}`)
    const doCancel = (e) => {
      e.stopPropagation()
      const s = lp.state[m.index]
      if (s?.filePath) call('CancelEncoding', s.filePath)
    }
    cancelBtn?.addEventListener('click', doCancel)
    cancelBtn?.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' || e.key === ' ') doCancel(e)
    })

    block.addEventListener('dragover', (e) => {
      if (!e.dataTransfer.types.includes('application/livepaper')) return
      const s = lp.state[m.index]
      if (s?.isVideo && !s?.ready) return
      e.preventDefault()
      e.dataTransfer.dropEffect = 'copy'
      block.classList.add('drop-over')
    })
    block.addEventListener('dragleave', (e) => {
      if (!block.contains(e.relatedTarget)) block.classList.remove('drop-over')
    })
    block.addEventListener('drop', async (e) => {
      e.preventDefault()
      block.classList.remove('drop-over')
      const s = lp.state[m.index]
      if (s?.isVideo && !s?.ready) return
      let entry
      try {
        entry = JSON.parse(e.dataTransfer.getData('application/livepaper'))
      } catch {
        return
      }

      if (entry.remote) {
        await lp.fn.applyRemoteEntryToMonitor?.(entry, m)
        return
      }

      const isNonGifVideo = entry.isVideo && extOf(entry.filePath) !== 'gif'
      const resolutionMismatch =
        isNonGifVideo &&
        (!entry.width || !entry.height || entry.width !== m.width || entry.height !== m.height)

      if (resolutionMismatch) {
        lp.state[m.index] = {
          filePath: entry.filePath,
          cachedPath: entry.filePath,
          isVideo: true,
          ready: false,
          thumbnail: entry.thumbnail,
        }
        applyThumb(m.index, entry.thumbnail, entry.filePath, true)
        lp.pendingChanges = true
        refreshApply()
        setEncoding(m.index, true, 0)
        try {
          const cached = await call('PreprocessVideo', entry.filePath, m.width, m.height)
          lp.state[m.index].cachedPath = cached
          lp.state[m.index].ready = true
        } catch (_) {
          cancelEncode(m.index)
          status('Encoding cancelled.', '')
          return
        }
        setEncoding(m.index, false, 100)
        refreshApply()
        return
      }

      lp.state[m.index] = {
        filePath: entry.filePath,
        cachedPath: entry.cachedPath || entry.filePath,
        isVideo: entry.isVideo,
        ready: true,
        thumbnail: entry.thumbnail,
      }
      applyThumb(m.index, entry.thumbnail, entry.filePath, entry.isVideo)
      lp.pendingChanges = true
      refreshApply()
    })

    const label = document.createElement('p')
    label.className = 'mon-label'
    label.textContent = `Monitor ${m.index + 1}`

    wrapper.appendChild(block)
    wrapper.appendChild(label)
    stage.appendChild(wrapper)
  }
  return stage
}

function reapplyStageState() {
  for (const [idxStr, s] of Object.entries(lp.state)) {
    if (!s.filePath) continue
    const idx = parseInt(idxStr)
    const block = document.getElementById(`mb-${idx}`)
    if (!block) continue
    block.classList.add('has-file')
    const bg = document.getElementById(`bg-${idx}`)
    if (bg) bg.src = s.thumbnail || ''
    const bd = document.getElementById(`bd-${idx}`)
    if (bd) {
      const e = extOf(s.filePath)
      bd.style.display = 'inline'
      if (s.isVideo && e !== 'gif') {
        bd.textContent = 'VIDEO'
        bd.className = 'mon-badge video'
      } else if (e === 'gif') {
        bd.textContent = 'GIF'
        bd.className = 'mon-badge gif'
      } else {
        bd.textContent = 'IMG'
        bd.className = 'mon-badge image'
      }
    }
    if (s.isVideo && !s.ready) setEncoding(idx, true, 0)
  }
}

export async function renderMonitorLayout() {
  const viewport = getMonitorViewport()
  const layout = await call(
    'GetMonitorLayout',
    viewport.width,
    viewport.height,
    viewport.labelHeight
  )
  lp.monitors = layout?.monitors || []
  contentEl.innerHTML = ''
  contentEl.appendChild(buildStage(layout))
  reapplyStageState()
}

// ── Card update helpers ───────────────────────────────────────────────────────

export function applyThumb(idx, thumbnail, filePath, isVideo) {
  const block = document.getElementById(`mb-${idx}`)
  if (!block) return
  block.classList.add('has-file')
  const bg = document.getElementById(`bg-${idx}`)
  if (bg) bg.src = thumbnail || ''
  const e = extOf(filePath)
  const bd = document.getElementById(`bd-${idx}`)
  if (bd) {
    bd.style.display = 'inline'
    if (isVideo && e !== 'gif') {
      bd.textContent = 'VIDEO'
      bd.className = 'mon-badge video'
    } else if (e === 'gif') {
      bd.textContent = 'GIF'
      bd.className = 'mon-badge gif'
    } else {
      bd.textContent = 'IMG'
      bd.className = 'mon-badge image'
    }
  }
}

export function setEncoding(idx, active, pct) {
  const block = document.getElementById(`mb-${idx}`)
  const mp = document.getElementById(`mp-${idx}`)
  const pf = document.getElementById(`pf-${idx}`)
  const pl = document.getElementById(`pl-${idx}`)
  if (!mp) return
  mp.classList.toggle('active', active)
  if (block) block.classList.toggle('encoding', active)
  if (pf) pf.style.strokeDashoffset = `${100 - pct}`
  if (pl) pl.textContent = `${pct}%`
}

export function resetMonitor(idx) {
  delete lp.state[idx]
  const block = document.getElementById(`mb-${idx}`)
  if (!block) return
  block.classList.remove('has-file', 'encoding')
  const bg = document.getElementById(`bg-${idx}`)
  if (bg) bg.src = ''
  const bd = document.getElementById(`bd-${idx}`)
  if (bd) {
    bd.style.display = 'none'
    bd.className = 'mon-badge'
  }
  const mp = document.getElementById(`mp-${idx}`)
  if (mp) mp.classList.remove('active')
}

export function cancelEncode(idx) {
  const prev = lp.lastAppliedState[idx]
  if (prev) {
    lp.state[idx] = { ...prev }
    setEncoding(idx, false, 0)
    applyThumb(idx, prev.thumbnail, prev.filePath, prev.isVideo)
  } else {
    resetMonitor(idx)
  }
  const allIdxs = new Set([
    ...Object.keys(lp.state).map(Number),
    ...Object.keys(lp.lastAppliedState).map(Number),
  ])
  let dirty = false
  for (const i of allIdxs) {
    if ((lp.state[i]?.filePath || '') !== (lp.lastAppliedState[i]?.filePath || '')) {
      dirty = true
      break
    }
  }
  lp.pendingChanges = dirty
  refreshApply()
}

// ── Browse / encode flow ──────────────────────────────────────────────────────

export async function browse(idx, w, h) {
  status('Opening file picker…')
  let filePath
  try {
    filePath = await call('BrowseFile')
  } catch (e) {
    status(`Dialog error: ${e}`, 'error')
    return
  }
  status('')
  if (!filePath) return

  const isVideo = await call('IsVideoFile', filePath)
  const thumbnail = await call('GetMonitorThumbnail', filePath, w, h)

  lp.state[idx] = { filePath, cachedPath: filePath, isVideo, ready: !isVideo, thumbnail }
  applyThumb(idx, thumbnail, filePath, isVideo)
  lp.pendingChanges = true
  refreshApply()

  if (isVideo) {
    setEncoding(idx, true, 0)
    try {
      const cached = await call('PreprocessVideo', filePath, w, h)
      lp.state[idx].cachedPath = cached
      lp.state[idx].ready = true
    } catch (e) {
      cancelEncode(idx)
      status('Encoding cancelled.', '')
      return
    }
    setEncoding(idx, false, 100)
    refreshApply()
  }
}

// ── Progress events from Go ───────────────────────────────────────────────────

onEvent('video:progress', (evt) => {
  const d = evt?.data || {}
  for (const [idx, s] of Object.entries(lp.state)) {
    if (s.filePath === d.file && !s.ready) setEncoding(parseInt(idx), true, d.progress ?? 0)
  }
})

// ── Apply ─────────────────────────────────────────────────────────────────────

export function commitApply() {
  lp.lastAppliedState = {}
  for (const [idx, s] of Object.entries(lp.state)) {
    if (s.filePath) lp.lastAppliedState[parseInt(idx)] = { ...s }
  }
  lp.pendingChanges = false
  saveState()
}

applyBtn.addEventListener('click', async () => {
  applyBtn.disabled = true
  status('Applying…')

  const list = Object.entries(lp.state)
    .filter(([, s]) => s.filePath)
    .map(([idx, s]) => ({ monitorIndex: parseInt(idx), filePath: s.cachedPath || s.filePath }))

  try {
    await call('ApplyWallpapers', list)
    status('Applied!', 'success')
    commitApply()
    await Promise.all(
      Object.entries(lp.state)
        .filter(([, s]) => s.filePath)
        .map(([idx, s]) => {
          const monitor = lp.monitors.find((mo) => mo.index === parseInt(idx))
          const w = monitor?.width || 0
          const h = monitor?.height || 0
          const isNonGifVideo = s.isVideo && extOf(s.filePath) !== 'gif'
          const fileKey = isNonGifVideo ? `${s.filePath}|${w}x${h}` : s.filePath
          return upsertRecent({
            fileKey,
            filePath: s.filePath,
            cachedPath: s.cachedPath,
            isVideo: s.isVideo,
            thumbnail: s.thumbnail,
            width: w,
            height: h,
          }).catch(() => {})
        })
    )
    pruneAndRefresh()
    setTimeout(() => status(''), 3000)
  } catch (e) {
    status(`Failed: ${e}`, 'error')
  }
  refreshApply()
})

discardBtn?.addEventListener('click', () => {
  for (const idx of Object.keys(lp.state).map(Number)) {
    if (!lp.lastAppliedState[idx]) resetMonitor(idx)
  }
  for (const idx of Object.keys(lp.state)) delete lp.state[idx]
  for (const [idxStr, s] of Object.entries(lp.lastAppliedState)) {
    const idx = parseInt(idxStr)
    lp.state[idx] = { ...s }
    applyThumb(idx, s.thumbnail, s.filePath, s.isVideo)
  }
  lp.pendingChanges = false
  refreshApply()
})

document.getElementById('clean-btn')?.addEventListener('click', async () => {
  const btn = document.getElementById('clean-btn')
  btn.disabled = true
  status('Cleaning cache…')
  try {
    await call('CleanTempFiles')
    await lp.fn.clearRecentHistory?.()
    await refreshGallery()
    status('Cache cleaned', 'success', 3000)
  } catch (e) {
    status(`Clean failed: ${e}`, 'error')
  }
  btn.disabled = false
})

resetBtn.addEventListener('click', async () => {
  resetBtn.disabled = true
  applyBtn.disabled = true
  status('Resetting…')
  try {
    await call('ResetWallpapers')
  } catch (_) {}
  for (const idx of Object.keys(lp.state).map(Number)) resetMonitor(idx)
  lp.lastAppliedState = {}
  lp.pendingChanges = false
  localStorage.removeItem(STORAGE_KEY)
  await lp.fn.clearRecentHistory?.().catch(() => {})
  await refreshGallery().catch(() => {})
  status('Reset complete', 'success', 3000)
  refreshApply()
})

// ── State persistence ─────────────────────────────────────────────────────────

export function saveState() {
  const saved = {}
  for (const [idx, s] of Object.entries(lp.state)) {
    if (s.filePath)
      saved[idx] = { filePath: s.filePath, cachedPath: s.cachedPath, isVideo: s.isVideo }
  }
  localStorage.setItem(STORAGE_KEY, JSON.stringify(saved))
}

export async function restoreState() {
  let saved
  try {
    saved = JSON.parse(localStorage.getItem(STORAGE_KEY) || 'null')
  } catch (_) {}
  if (!saved) return false

  for (const [idx, s] of Object.entries(saved)) {
    const i = parseInt(idx)
    if (!lp.monitors.find((m) => m.index === i)) continue
    const exists = await call('FileExists', s.filePath)
    if (!exists) continue
    const cachedPath = s.cachedPath || s.filePath
    if (s.isVideo && !(await call('FileExists', cachedPath))) continue
    const mon = lp.monitors.find((m) => m.index === i)
    const thumbnail = await call(
      'GetMonitorThumbnail',
      s.filePath,
      mon?.width || 0,
      mon?.height || 0
    )
    lp.state[i] = { filePath: s.filePath, cachedPath, isVideo: s.isVideo, ready: true, thumbnail }
    applyThumb(i, thumbnail, s.filePath, s.isVideo)
  }
  refreshApply()
  return Object.keys(lp.state).length > 0
}

export async function autoApply() {
  const list = Object.entries(lp.state)
    .filter(([, s]) => s.filePath && s.ready)
    .map(([idx, s]) => ({ monitorIndex: parseInt(idx), filePath: s.cachedPath || s.filePath }))
  if (list.length === 0) return
  status('Restoring…')
  try {
    await call('ApplyWallpapers', list)
    lp.lastAppliedState = {}
    for (const [idx, s] of Object.entries(lp.state)) {
      if (s.filePath) lp.lastAppliedState[parseInt(idx)] = { ...s }
    }
    status('Restored', 'success', 3000)
  } catch (e) {
    status(`failed: ${e}`, 'error')
  }
}

// ── Resize observer + titlebar buttons ───────────────────────────────────────

const ro = new ResizeObserver(() => {
  clearTimeout(lp.resizeTimer)
  lp.resizeTimer = setTimeout(async () => {
    if (lp.monitors.length === 0) return
    await renderMonitorLayout()
  }, 80)
})

const depWarnEl = document.getElementById('dep-warn')
const depWarnMsgEl = document.getElementById('dep-warn-msg')
const depInstallBtn = document.getElementById('dep-install-btn')

async function refreshDependencyWarning() {
  const deps = await call('CheckDependencies')
  const missing = Object.entries(deps)
    .filter(([, ok]) => !ok)
    .map(([name]) => name)
  if (missing.length === 0) {
    depWarnEl?.classList.add('hidden')
    if (depWarnEl) depWarnEl.style.display = ''
    return []
  }
  if (depWarnMsgEl) {
    depWarnMsgEl.innerHTML = `Missing: <strong>${missing.map(escapeHtml).join(', ')}</strong> — video wallpapers won't work.`
  }
  depWarnEl?.classList.remove('hidden')
  if (depWarnEl) depWarnEl.style.display = 'flex'
  return missing
}

depInstallBtn?.addEventListener('click', async () => {
  depInstallBtn.disabled = true
  depInstallBtn.textContent = 'Installing…'
  depInstallBtn.setAttribute('aria-busy', 'true')
  if (depWarnMsgEl) depWarnMsgEl.textContent = 'Downloading and installing ffmpeg and mpv…'
  status('Installing video dependencies…')
  try {
    await call('InstallDependencies')
    const missing = await refreshDependencyWarning()
    if (missing.length > 0) throw new Error(`Still missing: ${missing.join(', ')}`)
    status('Video dependencies installed', 'success', 4000)
  } catch (error) {
    if (depWarnMsgEl) depWarnMsgEl.textContent = `Install failed: ${String(error)}`
    depWarnEl?.classList.remove('hidden')
    if (depWarnEl) depWarnEl.style.display = 'flex'
    status('Dependency installation failed', 'error', 5000)
  } finally {
    depInstallBtn.disabled = false
    depInstallBtn.textContent = 'Install'
    depInstallBtn.removeAttribute('aria-busy')
  }
})

document
  .getElementById('tb-min')
  ?.addEventListener('click', () => call('WindowMinimise').catch(() => {}))
document
  .getElementById('tb-max')
  ?.addEventListener('click', () => call('WindowToggleMaximise').catch(() => {}))
document
  .getElementById('tb-close')
  ?.addEventListener('click', () => call('WindowHide').catch(() => {}))

// ── Init ──────────────────────────────────────────────────────────────────────

export async function init() {
  try {
    const ver = await call('GetVersion')
    const verText = ver === 'dev' ? 'preview' : ver ? `v${ver}` : ''
    document.getElementById('tb-version').textContent = verText
    const modalVer = document.getElementById('modal-version')
    if (modalVer) modalVer.textContent = verText || '—'
    await renderMonitorLayout()
    const n = lp.monitors.length
    document.getElementById('monitor-subtitle').textContent =
      `${n} display${n !== 1 ? 's' : ''} connected · Click to browse or drag from gallery`
  } catch (e) {
    contentEl.innerHTML = `<div class="loading text-lp-danger">Failed to load monitors: ${escapeHtml(String(e))}</div>`
    return
  }

  try {
    await refreshDependencyWarning()
  } catch (_) {}

  ro.observe(contentEl)

  if (!lp.appSettings) {
    try {
      lp.appSettings = await call('GetSettings')
    } catch (_) {}
  }
  const restoreEnabled = !lp.appSettings || lp.appSettings.restoreLastPlaylist !== false
  const hasRestored = restoreEnabled ? await restoreState() : false
  if (hasRestored) {
    autoApply()
  } else {
    call('WindowShow').catch(() => {})
  }

  await refreshGallery()
}

// Register in cross-module registry
lp.fn.renderMonitorLayout = renderMonitorLayout
lp.fn.applyThumb = applyThumb
lp.fn.setEncoding = setEncoding
lp.fn.resetMonitor = resetMonitor
lp.fn.cancelEncode = cancelEncode
lp.fn.commitApply = commitApply
lp.fn.saveState = saveState
lp.fn.init = init
lp.fn.autoApply = autoApply
