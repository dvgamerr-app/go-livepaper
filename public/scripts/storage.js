// Storage (Admin): wallpaper list, upload, delete.

import { lp, call, API_BASE } from '/scripts/store.js'
import { status, escapeHtml } from '/scripts/ui.js'

// ── Load and render ────────────────────────────────────────────────────────────

export async function loadStorageWallpapers() {
  const tbody = document.getElementById('storage-tbody')
  const countEl = document.getElementById('storage-count')
  if (!tbody) return
  tbody.innerHTML = '<tr><td colspan="6" class="storage-empty">Loading…</td></tr>'
  try {
    const res = await fetch(`${API_BASE}/api/admin/wallpapers`, {
      headers: lp.fn.authHeaders?.() || {},
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const data = await res.json()
    if (data.error) throw new Error(data.error)
    lp.storageItems = Array.isArray(data) ? data : data.items || []
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="4" class="storage-empty text-lp-danger">Failed: ${escapeHtml(String(e))}</td></tr>`
    return
  }
  renderStorageTable()
  if (countEl)
    countEl.textContent = `${lp.storageItems.length} wallpaper${lp.storageItems.length !== 1 ? 's' : ''}`
}

export function renderStorageTable() {
  const tbody = document.getElementById('storage-tbody')
  if (!tbody) return
  if (!lp.storageItems.length) {
    tbody.innerHTML =
      '<tr><td colspan="4" class="storage-empty">No wallpapers yet. Click Upload to add one.</td></tr>'
    return
  }
  tbody.innerHTML = ''
  lp.storageItems.forEach((it) => {
    const isPublished = it.isPublished !== false
    const shortId = it.id ? it.id.slice(0, 16) + '…' : '—'
    const tr = document.createElement('tr')
    tr.innerHTML = `
      <td><img class="storage-thumb" src="${escapeHtml(it.thumbnailUrl || '')}" alt=""></td>
      <td>
        <div class="storage-title" title="${escapeHtml(it.title || '')}">${escapeHtml(it.title || 'Untitled')}</div>
        <div class="storage-id">${escapeHtml(shortId)}</div>
      </td>
      <td>
        <span class="storage-status ${isPublished ? 'published' : 'hidden'}" title="${isPublished ? 'Live' : 'Hidden'}">
          <svg width="8" height="8" viewBox="0 0 8 8" aria-hidden="true"><circle cx="4" cy="4" r="4" fill="currentColor"/></svg>
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
            ${
              isPublished
                ? '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>'
                : '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>'
            }
          </button>
          <button class="storage-action-btn danger" title="Delete" data-action="delete" data-id="${escapeHtml(it.id)}" data-title="${escapeHtml(it.title || 'Untitled')}">
            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/></svg>
          </button>
        </div>
      </td>`
    const titleEl = tr.querySelector('.storage-title')
    titleEl.addEventListener('click', () => startEditTitle(titleEl, it.id, it))
    tr.querySelectorAll('[data-action]').forEach((btn) => {
      btn.addEventListener('click', () =>
        onStorageAction(btn.dataset.action, btn.dataset.id, btn.dataset)
      )
    })
    tbody.appendChild(tr)
  })
}

function startEditTitle(el, id, it) {
  const originalTitle = it.title || ''
  const input = document.createElement('input')
  input.type = 'text'
  input.value = originalTitle
  input.className = 'storage-title-input'
  el.replaceWith(input)
  input.focus()
  input.select()

  let done = false

  const restore = () => {
    const div = document.createElement('div')
    div.className = 'storage-title'
    div.title = it.title || ''
    div.textContent = it.title || 'Untitled'
    div.addEventListener('click', () => startEditTitle(div, id, it))
    input.replaceWith(div)
  }

  input.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      done = true
      it.title = originalTitle
      restore()
    }
    if (e.key === 'Enter') {
      e.preventDefault()
      input.blur()
    }
  })

  input.addEventListener('blur', async () => {
    if (done) return
    done = true
    const newTitle = input.value.trim()
    if (newTitle && newTitle !== originalTitle) {
      try {
        await call(
          'AdminPatchWallpaper',
          lp.fn.getToken?.() || '',
          id,
          JSON.stringify({ title: newTitle })
        )
        it.title = newTitle
        status('Saved', 'success')
        setTimeout(() => status(''), 1200)
      } catch (e) {
        it.title = originalTitle
        status(`Failed: ${e}`, 'error')
      }
    }
    restore()
  })
}

async function onStorageAction(action, id, data) {
  if (action === 'replace-thumb' || action === 'replace-orig') {
    const uploadType = action === 'replace-thumb' ? 'thumbnail' : 'original'
    let filePath
    try {
      filePath = await call('BrowseFile')
    } catch {
      return
    }
    if (!filePath) return
    status(`Replacing ${uploadType}…`)
    try {
      await call('AdminReplaceFile', lp.fn.getToken?.() || '', id, uploadType, filePath)
      status('Replaced!', 'success')
      setTimeout(() => {
        status('')
        loadStorageWallpapers()
      }, 1500)
    } catch (e) {
      status(`Failed: ${e}`, 'error')
    }
  }
  if (action === 'toggle-publish') {
    const nowPublished = data.published === 'true'
    try {
      await call(
        'AdminPatchWallpaper',
        lp.fn.getToken?.() || '',
        id,
        JSON.stringify({ isPublished: !nowPublished })
      )
      const it = lp.storageItems.find((i) => i.id === id)
      if (it) it.isPublished = !nowPublished
      renderStorageTable()
    } catch (e) {
      status(`Failed: ${e}`, 'error')
    }
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

// lp._uploadFiles: Array<{ filePath, title, thumb, thumbLoading, status, errorMsg }>

const SVG_X = `<svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>`
const SVG_CHECK = `<svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>`
const SVG_WARN = `<svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>`

function openUploadModal() {
  lp._uploadFiles = []
  const modal = document.getElementById('storage-upload-modal')
  const progressWrap = document.getElementById('upload-progress-wrap')
  const submitBtn = document.getElementById('upload-submit-btn')
  const cancelBtn = document.getElementById('upload-cancel-btn')
  if (progressWrap) progressWrap.classList.add('hidden')
  if (submitBtn) {
    submitBtn.disabled = true
    submitBtn.textContent = 'Upload'
  }
  if (cancelBtn) {
    cancelBtn.disabled = false
    cancelBtn.textContent = 'Cancel'
  }
  renderUploadFileList()
  if (modal) modal.hidden = false
}

function closeUploadModal() {
  const modal = document.getElementById('storage-upload-modal')
  if (modal) modal.hidden = true
  lp._uploadFiles = []
}

function statusHtml(s, msg) {
  if (s === 'uploading') return 'Uploading…'
  if (s === 'done') return `${SVG_CHECK} Uploaded`
  if (s === 'error') return `${SVG_WARN} ${escapeHtml(msg || 'Failed')}`
  return ''
}

function renderUploadFileList() {
  const list = document.getElementById('upload-file-list')
  const submitBtn = document.getElementById('upload-submit-btn')
  const zoneLabel = document.querySelector('#upload-file-zone .zone-label')
  if (!list) return
  if (zoneLabel)
    zoneLabel.textContent = lp._uploadFiles.length ? 'Add more files…' : 'Click to add files'
  if (!lp._uploadFiles.length) {
    list.classList.add('hidden')
    if (submitBtn) {
      submitBtn.disabled = true
      submitBtn.textContent = 'Upload'
    }
    return
  }
  list.classList.remove('hidden')
  list.innerHTML = ''
  lp._uploadFiles.forEach((file, idx) => {
    const item = document.createElement('div')
    item.className = 'upload-file-item'
    item.dataset.index = idx
    const thumbHtml = file.thumbLoading
      ? '<div class="upload-thumb-skel"></div>'
      : file.thumb
        ? `<img src="${file.thumb}" alt="" style="width:100%;height:100%;object-fit:cover;display:block;border-radius:4px">`
        : '<div style="width:100%;height:100%;background:#1a1a1d;border-radius:4px"></div>'
    const statusClass =
      file.status === 'uploading'
        ? 'uploading'
        : file.status === 'done'
          ? 'done'
          : file.status === 'error'
            ? 'error'
            : ''
    const canRemove = file.status === 'idle' || file.status === 'error'
    item.innerHTML = `
      <div class="upload-item-thumb">${thumbHtml}</div>
      <div class="upload-item-info">
        <input type="text" class="upload-item-title" value="${escapeHtml(file.title)}" placeholder="Title…" ${file.status !== 'idle' ? 'disabled' : ''}>
        <span class="upload-item-status ${statusClass}">${statusHtml(file.status, file.errorMsg)}</span>
      </div>
      <button class="upload-item-remove" data-idx="${idx}" ${canRemove ? '' : 'disabled'} title="Remove">${SVG_X}</button>`
    item.querySelector('.upload-item-title')?.addEventListener('input', (e) => {
      if (lp._uploadFiles[idx]) lp._uploadFiles[idx].title = e.target.value
    })
    item.querySelector('.upload-item-remove')?.addEventListener('click', () => {
      lp._uploadFiles.splice(idx, 1)
      renderUploadFileList()
    })
    list.appendChild(item)
  })
  const idleCount = lp._uploadFiles.filter((f) => f.status === 'idle').length
  if (submitBtn) {
    submitBtn.disabled = idleCount === 0
    submitBtn.textContent =
      idleCount > 0 ? `Upload ${idleCount} file${idleCount !== 1 ? 's' : ''}` : 'Upload'
  }
}

function updateFileThumb(idx) {
  const item = document.querySelector(`.upload-file-item[data-index="${idx}"]`)
  if (!item) return
  const thumbCell = item.querySelector('.upload-item-thumb')
  if (!thumbCell) return
  const file = lp._uploadFiles[idx]
  if (!file) return
  if (file.thumb) {
    thumbCell.innerHTML = `<img src="${file.thumb}" alt="" style="width:100%;height:100%;object-fit:cover;display:block;border-radius:4px">`
  } else {
    thumbCell.innerHTML =
      '<div style="width:100%;height:100%;background:#1a1a1d;border-radius:4px"></div>'
  }
}

function updateFileStatus(idx, s, msg) {
  if (lp._uploadFiles[idx]) {
    lp._uploadFiles[idx].status = s
    lp._uploadFiles[idx].errorMsg = msg || ''
  }
  const item = document.querySelector(`.upload-file-item[data-index="${idx}"]`)
  if (!item) return
  const statusEl = item.querySelector('.upload-item-status')
  const titleEl = item.querySelector('.upload-item-title')
  const removeBtn = item.querySelector('.upload-item-remove')
  if (statusEl) {
    statusEl.className = `upload-item-status ${s === 'uploading' ? 'uploading' : s === 'done' ? 'done' : s === 'error' ? 'error' : ''}`
    statusEl.innerHTML = statusHtml(s, msg)
  }
  if (titleEl) titleEl.disabled = s !== 'idle'
  if (removeBtn) removeBtn.disabled = !(s === 'idle' || s === 'error')
}

async function addFilesFromBrowse() {
  let raw
  try {
    raw = await call('BrowseFiles')
  } catch {
    return
  }
  let paths
  try {
    paths = JSON.parse(raw)
  } catch {
    return
  }
  if (!Array.isArray(paths) || !paths.length) return

  for (const filePath of paths) {
    const label = filePath.replace(/\\/g, '/').split('/').pop()
    const title = label.replace(/\.[^.]+$/, '').replace(/[-_]/g, ' ')
    lp._uploadFiles.push({
      filePath,
      title,
      thumb: null,
      thumbLoading: true,
      status: 'idle',
      errorMsg: '',
    })
  }
  renderUploadFileList()

  // Generate thumbnails async — update each cell as they arrive
  const startIdx = lp._uploadFiles.length - paths.length
  for (let i = 0; i < paths.length; i++) {
    const idx = startIdx + i
    const file = lp._uploadFiles[idx]
    if (!file) continue
    try {
      const thumb = await call('GetThumbnail', file.filePath)
      if (lp._uploadFiles[idx]) {
        lp._uploadFiles[idx].thumb = thumb || null
        lp._uploadFiles[idx].thumbLoading = false
        updateFileThumb(idx)
      }
    } catch {
      if (lp._uploadFiles[idx]) {
        lp._uploadFiles[idx].thumbLoading = false
        updateFileThumb(idx)
      }
    }
  }
}

async function doUpload() {
  const idleFiles = lp._uploadFiles
    .map((f, idx) => ({ ...f, idx }))
    .filter((f) => f.status === 'idle')
  if (!idleFiles.length) return

  const progressWrap = document.getElementById('upload-progress-wrap')
  const progressBar = document.getElementById('upload-progress-bar')
  const progressLabel = document.getElementById('upload-progress-label')
  const submitBtn = document.getElementById('upload-submit-btn')
  const cancelBtn = document.getElementById('upload-cancel-btn')

  if (progressWrap) progressWrap.classList.remove('hidden')
  if (submitBtn) submitBtn.disabled = true
  if (cancelBtn) cancelBtn.disabled = true

  let done = 0
  for (const file of idleFiles) {
    if (progressLabel) progressLabel.textContent = `Uploading ${done + 1} of ${idleFiles.length}…`
    if (progressBar) progressBar.style.width = `${Math.round((done / idleFiles.length) * 100)}%`
    updateFileStatus(file.idx, 'uploading')
    try {
      await call(
        'AdminUploadWallpaper',
        lp.fn.getToken?.() || '',
        file.filePath,
        file.title.trim() || 'Untitled',
        'premium'
      )
      updateFileStatus(file.idx, 'done')
      done++
    } catch (e) {
      updateFileStatus(
        file.idx,
        'error',
        String(e)
          .replace(/^Error:\s*/i, '')
          .substring(0, 60)
      )
    }
  }

  if (progressBar) progressBar.style.width = '100%'
  if (progressLabel) progressLabel.textContent = `${done} of ${idleFiles.length} uploaded`
  if (cancelBtn) {
    cancelBtn.disabled = false
    cancelBtn.textContent = done > 0 ? 'Close' : 'Cancel'
  }

  const stillIdle = lp._uploadFiles.some((f) => f.status === 'idle')
  if (submitBtn && stillIdle) submitBtn.disabled = false

  if (done === idleFiles.length) {
    setTimeout(() => {
      closeUploadModal()
      status('Uploaded!', 'success')
      setTimeout(() => {
        status('')
        loadStorageWallpapers()
      }, 1200)
    }, 800)
  }
}

// ── Event listeners ────────────────────────────────────────────────────────────

document.getElementById('storage-delete-cancel')?.addEventListener('click', () => {
  document.getElementById('storage-delete-modal').hidden = true
})
document.getElementById('storage-delete-confirm')?.addEventListener('click', async (e) => {
  const id = e.currentTarget.dataset.deleteId
  const btn = e.currentTarget
  btn.disabled = true
  btn.textContent = 'Deleting…'
  document.getElementById('storage-delete-modal').hidden = true
  try {
    await call('AdminDeleteWallpaper', lp.fn.getToken?.() || '', id)
    lp.storageItems = lp.storageItems.filter((i) => i.id !== id)
    renderStorageTable()
    const countEl = document.getElementById('storage-count')
    if (countEl)
      countEl.textContent = `${lp.storageItems.length} wallpaper${lp.storageItems.length !== 1 ? 's' : ''}`
    status('Deleted', 'success', 2000)
  } catch (err) {
    status(`Delete failed: ${err}`, 'error')
  }
  btn.disabled = false
  btn.textContent = 'Delete permanently'
})

document.getElementById('storage-upload-btn')?.addEventListener('click', openUploadModal)
document.getElementById('upload-cancel-btn')?.addEventListener('click', closeUploadModal)
document.getElementById('upload-file-zone')?.addEventListener('click', addFilesFromBrowse)
document.getElementById('upload-submit-btn')?.addEventListener('click', doUpload)

// Register in cross-module registry
lp.fn.loadStorageWallpapers = loadStorageWallpapers
lp.fn.renderStorageTable = renderStorageTable
