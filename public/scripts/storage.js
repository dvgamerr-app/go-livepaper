// Storage (Admin): wallpaper list, upload, delete.

import { lp, call } from '/scripts/store.js'
import { status, escapeHtml } from '/scripts/ui.js'

// ── Load and render ────────────────────────────────────────────────────────────

export async function loadStorageWallpapers() {
  const tbody = document.getElementById('storage-tbody')
  const countEl = document.getElementById('storage-count')
  if (!tbody) return
  tbody.innerHTML = '<tr><td colspan="6" class="storage-empty">Loading…</td></tr>'
  try {
    const raw = await call('AdminListWallpapers', lp.fn.getToken?.() || '')
    const data = JSON.parse(raw)
    if (data.error) throw new Error(data.error)
    lp.storageItems = Array.isArray(data) ? data : (data.items || [])
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="6" class="storage-empty text-lp-danger">Failed: ${escapeHtml(String(e))}</td></tr>`
    return
  }
  renderStorageTable()
  if (countEl) countEl.textContent = `${lp.storageItems.length} wallpaper${lp.storageItems.length !== 1 ? 's' : ''}`
}

export function renderStorageTable() {
  const tbody = document.getElementById('storage-tbody')
  if (!tbody) return
  if (!lp.storageItems.length) {
    tbody.innerHTML = '<tr><td colspan="6" class="storage-empty">No wallpapers yet. Click Upload to add one.</td></tr>'
    return
  }
  tbody.innerHTML = ''
  lp.storageItems.forEach((it) => {
    const isVideo = (it.contentType || '').startsWith('video/')
    const isPublished = it.isPublished !== false
    const tier = it.tier || 'free'
    const shortId = it.id ? it.id.slice(0, 16) + '…' : '—'
    const tr = document.createElement('tr')
    tr.innerHTML = `
      <td><img class="storage-thumb" src="${escapeHtml(it.thumbnailUrl || '')}" alt=""></td>
      <td>
        <div class="storage-title" title="${escapeHtml(it.title || '')}">${escapeHtml(it.title || 'Untitled')}</div>
        <div class="storage-id">${escapeHtml(shortId)}</div>
      </td>
      <td><span class="tier-badge ${tier}">${tier}</span></td>
      <td class="storage-type">${isVideo ? '🎬 Video' : '🖼 Image'}</td>
      <td>
        <span class="storage-status ${isPublished ? 'published' : 'hidden'}">
          <svg width="8" height="8" viewBox="0 0 8 8" aria-hidden="true"><circle cx="4" cy="4" r="4" fill="currentColor"/></svg>
          ${isPublished ? 'Live' : 'Hidden'}
        </span>
      </td>
      <td>
        <div class="storage-actions">
          <button class="storage-action-btn" title="Replace thumbnail" data-action="replace-thumb" data-id="${escapeHtml(it.id)}">
            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21 15 16 10 5 21"/></svg>
          </button>
          <button class="storage-action-btn" title="Replace original file" data-action="replace-orig" data-id="${escapeHtml(it.id)}">
            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>
          </button>
          <button class="storage-action-btn" title="${isPublished ? 'Hide' : 'Show'}" data-action="toggle-publish" data-id="${escapeHtml(it.id)}" data-published="${isPublished}">
            ${isPublished
              ? '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>'
              : '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>'}
          </button>
          <button class="storage-action-btn danger" title="Delete" data-action="delete" data-id="${escapeHtml(it.id)}" data-title="${escapeHtml(it.title || 'Untitled')}">
            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/></svg>
          </button>
        </div>
      </td>`
    tr.querySelectorAll('[data-action]').forEach((btn) => {
      btn.addEventListener('click', () => onStorageAction(btn.dataset.action, btn.dataset.id, btn.dataset))
    })
    tbody.appendChild(tr)
  })
}

async function onStorageAction(action, id, data) {
  if (action === 'replace-thumb' || action === 'replace-orig') {
    const uploadType = action === 'replace-thumb' ? 'thumbnail' : 'original'
    let filePath
    try { filePath = await call('BrowseFile') } catch { return }
    if (!filePath) return
    status(`Replacing ${uploadType}…`)
    try {
      await call('AdminReplaceFile', lp.fn.getToken?.() || '', id, uploadType, filePath)
      status('Replaced!', 'success')
      setTimeout(() => { status(''); loadStorageWallpapers() }, 1500)
    } catch (e) { status(`Failed: ${e}`, 'error') }
  }
  if (action === 'toggle-publish') {
    const nowPublished = data.published === 'true'
    try {
      await call('AdminPatchWallpaper', lp.fn.getToken?.() || '', id, JSON.stringify({ isPublished: !nowPublished }))
      const it = lp.storageItems.find((i) => i.id === id)
      if (it) it.isPublished = !nowPublished
      renderStorageTable()
    } catch (e) { status(`Failed: ${e}`, 'error') }
  }
  if (action === 'delete') {
    const modal = document.getElementById('storage-delete-modal')
    const nameEl = document.getElementById('storage-delete-name')
    if (nameEl) nameEl.textContent = `"${data.title}"`
    if (modal) modal.hidden = false
    document.getElementById('storage-delete-confirm').dataset.deleteId = id
    document.getElementById('storage-delete-confirm').dataset.deleteTitle = data.title
  }
}

// ── Upload modal ───────────────────────────────────────────────────────────────

function openUploadModal() {
  lp._uploadFilePath = ''
  lp._uploadTier = 'free'
  const modal = document.getElementById('storage-upload-modal')
  const fileLabel = document.getElementById('upload-file-label')
  const thumbWrap = document.getElementById('upload-thumb-wrap')
  const titleInput = document.getElementById('upload-title')
  const progressWrap = document.getElementById('upload-progress-wrap')
  const submitBtn = document.getElementById('upload-submit-btn')
  if (fileLabel) fileLabel.textContent = 'Click to choose file…'
  if (thumbWrap) thumbWrap.classList.add('hidden')
  if (titleInput) titleInput.value = ''
  if (progressWrap) progressWrap.classList.add('hidden')
  if (submitBtn) { submitBtn.disabled = true; submitBtn.textContent = 'Upload' }
  document.querySelectorAll('.upload-tier-btn').forEach((b) => b.classList.toggle('active', b.dataset.tier === 'free'))
  if (modal) modal.hidden = false
}

function closeUploadModal() {
  const modal = document.getElementById('storage-upload-modal')
  if (modal) modal.hidden = true
}

// ── Event listeners ────────────────────────────────────────────────────────────

document.getElementById('storage-delete-cancel')?.addEventListener('click', () => {
  document.getElementById('storage-delete-modal').hidden = true
})
document.getElementById('storage-delete-confirm')?.addEventListener('click', async (e) => {
  const id = e.currentTarget.dataset.deleteId
  const btn = e.currentTarget
  btn.disabled = true; btn.textContent = 'Deleting…'
  document.getElementById('storage-delete-modal').hidden = true
  try {
    await call('AdminDeleteWallpaper', lp.fn.getToken?.() || '', id)
    lp.storageItems = lp.storageItems.filter((i) => i.id !== id)
    renderStorageTable()
    const countEl = document.getElementById('storage-count')
    if (countEl) countEl.textContent = `${lp.storageItems.length} wallpaper${lp.storageItems.length !== 1 ? 's' : ''}`
    status('Deleted', 'success', 2000)
  } catch (err) { status(`Delete failed: ${err}`, 'error') }
  btn.disabled = false; btn.textContent = 'Delete permanently'
})
document.getElementById('storage-upload-btn')?.addEventListener('click', openUploadModal)
document.getElementById('upload-cancel-btn')?.addEventListener('click', closeUploadModal)
document.getElementById('upload-file-zone')?.addEventListener('click', async () => {
  let filePath
  try { filePath = await call('BrowseFile') } catch { return }
  if (!filePath) return
  lp._uploadFilePath = filePath
  const label = filePath.replace(/\\/g, '/').split('/').pop()
  const fileLabel = document.getElementById('upload-file-label')
  if (fileLabel) fileLabel.textContent = label
  const titleInput = document.getElementById('upload-title')
  if (titleInput && !titleInput.value) {
    titleInput.value = label.replace(/\.[^.]+$/, '').replace(/[-_]/g, ' ')
  }
  const thumbWrap = document.getElementById('upload-thumb-wrap')
  const thumbImg = document.getElementById('upload-thumb-img')
  const fileInfo = document.getElementById('upload-file-info')
  const thumbNote = document.getElementById('upload-thumb-note')
  const submitBtn = document.getElementById('upload-submit-btn')
  if (thumbNote) thumbNote.textContent = 'Generating thumbnail…'
  if (thumbWrap) { thumbWrap.classList.remove('hidden'); thumbWrap.classList.add('flex') }
  try {
    const thumb = await call('GetThumbnail', filePath)
    if (thumbImg && thumb) thumbImg.src = thumb
    if (fileInfo) fileInfo.textContent = label
    const isVid = await call('IsVideoFile', filePath)
    if (thumbNote) thumbNote.textContent = isVid ? 'GIF thumbnail will be generated for upload' : 'JPEG thumbnail generated'
  } catch { if (thumbNote) thumbNote.textContent = 'Preview unavailable' }
  if (submitBtn) submitBtn.disabled = false
})
document.querySelectorAll('.upload-tier-btn').forEach((btn) => {
  btn.addEventListener('click', () => {
    lp._uploadTier = btn.dataset.tier
    document.querySelectorAll('.upload-tier-btn').forEach((b) => b.classList.toggle('active', b === btn))
  })
})
document.getElementById('upload-submit-btn')?.addEventListener('click', async () => {
  if (!lp._uploadFilePath) return
  const title = document.getElementById('upload-title')?.value?.trim() || 'Untitled'
  const submitBtn = document.getElementById('upload-submit-btn')
  const progressWrap = document.getElementById('upload-progress-wrap')
  const progressBar = document.getElementById('upload-progress-bar')
  const progressLabel = document.getElementById('upload-progress-label')
  submitBtn.disabled = true
  if (progressWrap) progressWrap.classList.remove('hidden')
  const steps = ['Generating thumbnail…', 'Creating entry…', 'Uploading thumbnail…', 'Uploading original…', 'Done!']
  let step = 0
  const tick = () => {
    if (progressBar) progressBar.style.width = `${Math.round((step / (steps.length - 1)) * 100)}%`
    if (progressLabel) progressLabel.textContent = steps[step] || '…'
    step++
  }
  tick()
  try {
    await call('AdminUploadWallpaper', lp.fn.getToken?.() || '', lp._uploadFilePath, title, lp._uploadTier)
    tick(); tick(); tick(); tick()
    closeUploadModal()
    status('Uploaded!', 'success')
    setTimeout(() => { status(''); loadStorageWallpapers() }, 1200)
  } catch (e) {
    if (progressLabel) progressLabel.textContent = `Failed: ${e}`
    if (progressBar) progressBar.style.background = 'var(--lp-danger)'
    submitBtn.disabled = false
  }
})

// Register in cross-module registry
lp.fn.loadStorageWallpapers = loadStorageWallpapers
lp.fn.renderStorageTable = renderStorageTable
