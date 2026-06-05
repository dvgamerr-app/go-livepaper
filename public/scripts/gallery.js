// Gallery strip: build cards, render strip, preview overlay.

import { lp, call, API_BASE, WALLPAPER_LIMIT } from '/scripts/store.js'
import { loadGalleryItems, pruneRecent } from '/scripts/db.js'
import { extOf, resolutionBadgeText, escapeHtml } from '/scripts/ui.js'

// ── Gallery strip ─────────────────────────────────────────────────────────────

export function buildGalleryCard(entry, index) {
  const card = document.createElement('div')
  card.className = 'gallery-strip-card wails-no-drag'
  card.dataset.index = index

  let name, badgeClass, badgeText, lockHtml
  if (entry.remote) {
    card.dataset.remote = '1'
    name = entry.title || 'Wallpaper'
    badgeClass = (entry.contentType || '').startsWith('video/') ? 'video' : 'image'
    badgeText = resolutionBadgeText(entry.width, entry.height) || 'HD'
    if (entry.width && entry.height) card.style.aspectRatio = `${entry.width} / ${entry.height}`
    lockHtml = `<span class="gc-lock${lp.fn.isPremium?.() ? ' hidden' : ''}" aria-hidden="true">
      <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>
    </span>`
  } else {
    const ext = extOf(entry.filePath)
    badgeClass = entry.isVideo && ext !== 'gif' ? 'video' : ext === 'gif' ? 'gif' : 'image'
    badgeText = entry.isVideo && ext !== 'gif' ? 'VIDEO' : ext === 'gif' ? 'GIF' : 'IMG'
    name = entry.filePath.replace(/\\/g, '/').split('/').pop().replace(/\.[^.]+$/, '')
    if (entry.width && entry.height) card.style.aspectRatio = `${entry.width} / ${entry.height}`
    lockHtml = ''
  }

  const isLocalVideo = !entry.remote && entry.isVideo && extOf(entry.filePath || '') !== 'gif'

  let payload
  if (entry.remote) {
    payload = JSON.stringify({ remote: true, id: entry.id, downloadUrl: entry.downloadUrl, tier: entry.tier, thumbnail: entry.thumbnail, title: entry.title })
  } else {
    payload = JSON.stringify({ filePath: entry.filePath, cachedPath: entry.cachedPath, isVideo: entry.isVideo, thumbnail: entry.thumbnail, width: entry.width || 0, height: entry.height || 0 })
  }

  card.draggable = true
  card.innerHTML = `
    <img class="gc-img" src="${entry.thumbnail || ''}" alt="" draggable="false">
    <div class="gc-overlay"></div>
    <span class="gc-badge ${badgeClass}">${badgeText}</span>
    ${lockHtml || ''}
    <div class="gc-grip" aria-hidden="true">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="9" cy="5" r="1"/><circle cx="9" cy="12" r="1"/><circle cx="9" cy="19" r="1"/>
        <circle cx="15" cy="5" r="1"/><circle cx="15" cy="12" r="1"/><circle cx="15" cy="19" r="1"/>
      </svg>
    </div>
    <div class="gc-info"><div class="gc-name"></div></div>
  `
  card.querySelector('.gc-name').textContent = name
  const imgEl = card.querySelector('.gc-img')
  imgEl.alt = name

  card.addEventListener('dragstart', (e) => {
    e.dataTransfer.setData('application/livepaper', payload)
    e.dataTransfer.effectAllowed = 'copy'
    card.classList.add('dragging')
  })
  card.addEventListener('dragend', () => card.classList.remove('dragging'))

  let _gifLoading = false
  let _gifUrl = entry._animGif || null
  let _hovering = false

  card.addEventListener('mouseenter', async () => {
    setActiveDot(index)
    _hovering = true
    if (!isLocalVideo) return
    if (_gifUrl) { imgEl.src = _gifUrl; return }
    if (_gifLoading) return
    _gifLoading = true
    try {
      const gif = await call('GetAnimatedThumbnail', entry.filePath)
      if (gif) { _gifUrl = gif; entry._animGif = gif; if (_hovering) imgEl.src = gif }
    } catch (_) {}
    _gifLoading = false
  })

  card.addEventListener('mouseleave', () => {
    _hovering = false
    if (isLocalVideo && _gifUrl) imgEl.src = entry.thumbnail || ''
  })

  card.addEventListener('click', () => openGalleryPreview(lp.galleryItems, index))

  return card
}

export function setActiveDot(index) {
  const dotsEl = document.getElementById('gallery-dots')
  if (!dotsEl) return
  dotsEl.querySelectorAll('.gallery-dot').forEach((d, i) => {
    d.classList.toggle('active', i === index)
  })
}

export function renderGalleryStrip() {
  const strip = document.getElementById('gallery-strip')
  const countEl = document.getElementById('gallery-count')
  const dotsEl = document.getElementById('gallery-dots')
  const prevBtn = document.getElementById('gallery-prev')
  const nextBtn = document.getElementById('gallery-next')
  if (!strip) return

  strip.innerHTML = ''

  if (lp.galleryItems.length === 0) {
    strip.innerHTML = '<div class="gallery-empty">No recent wallpapers yet — apply one to add it here</div>'
    if (countEl) countEl.textContent = 'No wallpapers yet'
    if (dotsEl) dotsEl.innerHTML = ''
    return
  }

  lp.galleryItems.forEach((entry, i) => strip.appendChild(buildGalleryCard(entry, i)))

  const n = lp.galleryItems.length
  if (countEl) countEl.textContent = `${n} wallpaper${n !== 1 ? 's' : ''} · Drag onto a display to apply`

  if (dotsEl) {
    dotsEl.innerHTML = ''
    lp.galleryItems.forEach((_, i) => {
      const dot = document.createElement('button')
      dot.className = 'gallery-dot' + (i === 0 ? ' active' : '')
      dot.setAttribute('aria-label', `Wallpaper ${i + 1}`)
      dot.addEventListener('click', () => {
        const cards = strip.querySelectorAll('.gallery-strip-card')
        if (cards[i]) cards[i].scrollIntoView({ behavior: 'smooth', inline: 'center', block: 'nearest' })
        setActiveDot(i)
      })
      dotsEl.appendChild(dot)
    })
  }

  if (prevBtn) prevBtn.onclick = () => strip.scrollBy({ left: -260, behavior: 'smooth' })
  if (nextBtn) nextBtn.onclick = () => strip.scrollBy({ left: 260, behavior: 'smooth' })
}

async function loadRemoteGalleryItems() {
  try {
    const res = await fetch(`${API_BASE}/api/wallpapers?limit=${WALLPAPER_LIMIT}`)
    const data = await res.json()
    return (data?.items || []).map((it) => ({
      remote: true, id: it.id, title: it.title, tier: it.tier || 'free',
      contentType: it.contentType || '', downloadUrl: it.downloadUrl,
      thumbnail: it.thumbnailUrl, width: it.width || 0, height: it.height || 0,
    }))
  } catch (_) { return [] }
}

export async function refreshGallery() {
  const local = await loadGalleryItems()
  lp.galleryItems = local.length > 0 ? local : await loadRemoteGalleryItems()
  renderGalleryStrip()
}

export function refreshGalleryLocks() {
  document.querySelectorAll('.gallery-strip-card[data-remote="1"] .gc-lock').forEach((el) => {
    el.classList.toggle('hidden', lp.fn.isPremium?.() ?? false)
  })
}

export function pruneAndRefresh() {
  pruneRecent().catch(() => {})
  refreshGallery().catch(() => {})
}

// ── Gallery strip preview overlay ─────────────────────────────────────────────

export function openGalleryPreview(items, index) {
  const entry = items[index]
  if (!entry) return
  if (entry.remote) {
    const full = lp.discoverItems.find((i) => i.id === entry.id)
    if (full) { lp.fn.onDiscoverPreview?.(full, lp.discoverItems, lp.discoverItems.indexOf(full)); return }
  }
  lp._gpItems = items
  lp._gpIndex = index
  _renderGalleryPreview()
}

function _renderGalleryPreview() {
  if (lp._pvOverlay) {
    lp._pvOverlay.remove()
    document.removeEventListener('keydown', lp._pvKeyHandler)
    lp._pvOverlay = null
  }

  const items = lp._gpItems
  const entry = items[lp._gpIndex]
  if (!entry) return

  const isRemote = !!entry.remote
  const name = isRemote
    ? (entry.title || 'Wallpaper')
    : ((entry.filePath || '').replace(/\\/g, '/').split('/').pop().replace(/\.[^.]+$/, '') || 'Wallpaper')
  const ext = entry.isVideo ? (extOf(entry.filePath || '') === 'gif' ? 'gif' : 'video') : 'image'

  const ov = document.createElement('div')
  ov.className = 'dc-preview-overlay'
  ov.innerHTML = `
    <img class="dc-preview-bg" src="${entry.thumbnail || ''}" alt="" draggable="false">
    <div class="dc-preview-scrim"></div>
    <div class="dc-preview-top">
      <button class="dc-preview-close" aria-label="Close preview">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
      </button>
      <span></span>
    </div>
    <div class="dc-preview-bottom">
      <div class="dc-preview-title">${escapeHtml(name)}</div>
      ${!isRemote && entry.width && entry.height ? `<div class="dc-preview-meta">${ext.toUpperCase()} · ${entry.width}×${entry.height}</div>` : ''}
      <div class="dc-preview-actions">
        <button id="gp-apply" class="dc-preview-btn primary">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polygon points="5 3 19 12 5 21 5 3"/></svg>
          Apply Wallpaper
        </button>
        <button id="gp-back" class="dc-preview-btn secondary">
          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="15 18 9 12 15 6"/></svg>
          Back
        </button>
        <button id="gp-random" class="dc-preview-btn secondary">
          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="16 3 21 3 21 8"/><line x1="4" y1="20" x2="21" y2="3"/><polyline points="21 16 21 21 16 21"/><line x1="15" y1="15" x2="21" y2="21"/></svg>
          Random
        </button>
        <button id="gp-next" class="dc-preview-btn secondary">
          Next
          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="9 18 15 12 9 6"/></svg>
        </button>
        <span class="dc-preview-counter">${lp._gpIndex + 1} / ${items.length}</span>
      </div>
    </div>`

  const closeGP = () => {
    ov.remove()
    document.removeEventListener('keydown', gpKey)
    lp._pvOverlay = null
  }

  ov.querySelector('.dc-preview-close').addEventListener('click', closeGP)

  ov.querySelector('#gp-apply').addEventListener('click', async () => {
    closeGP()
    if (isRemote) {
      const synthetic = { id: entry.id, downloadUrl: entry.downloadUrl, title: entry.title, tags: [], tier: entry.tier }
      await lp.fn.onDiscoverApply?.(synthetic)
    } else {
      if (lp.monitors.length <= 1) {
        const target = lp.monitors[0]
        if (target) await lp.fn.applyLocalEntryToMonitor?.(target, entry)
      } else {
        lp.fn.showLibraryMonitorPicker?.(entry)
      }
    }
  })

  ov.querySelector('#gp-back').addEventListener('click', () => {
    lp._gpIndex = (lp._gpIndex - 1 + items.length) % items.length
    _renderGalleryPreview()
  })
  ov.querySelector('#gp-next').addEventListener('click', () => {
    lp._gpIndex = (lp._gpIndex + 1) % items.length
    _renderGalleryPreview()
  })
  ov.querySelector('#gp-random').addEventListener('click', () => {
    lp._gpIndex = Math.floor(Math.random() * items.length)
    _renderGalleryPreview()
  })

  const gpKey = (e) => {
    if (e.key === 'Escape') closeGP()
    if (e.key === 'ArrowLeft') ov.querySelector('#gp-back')?.click()
    if (e.key === 'ArrowRight') ov.querySelector('#gp-next')?.click()
  }

  document.body.appendChild(ov)
  document.addEventListener('keydown', gpKey)
  lp._pvOverlay = ov
  lp._pvKeyHandler = gpKey
}

// ── Gallery wheel → horizontal scroll ────────────────────────────────────────

document.getElementById('gallery-strip')?.addEventListener('wheel', (e) => {
  if (Math.abs(e.deltaY) > Math.abs(e.deltaX)) {
    e.preventDefault()
    e.currentTarget.scrollLeft += e.deltaY * 3
  }
}, { passive: false })

// Register in cross-module registry
lp.fn.refreshGallery = refreshGallery
lp.fn.refreshGalleryLocks = refreshGalleryLocks
lp.fn.pruneAndRefresh = pruneAndRefresh
lp.fn.openGalleryPreview = openGalleryPreview
lp.fn.renderGalleryStrip = renderGalleryStrip
