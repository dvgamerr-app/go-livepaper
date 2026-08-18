// Discover: load catalog, cards, featured, preview, download, apply.

import { lp, call, API_BASE, WALLPAPER_LIMIT } from '/scripts/store.js'
import { status, escapeHtml, track, openSettingsTab } from '/scripts/ui.js'
import { isDownloaded, setDownloadedItem, getDownloadedItem, upsertRecent } from '/scripts/db.js'

const DC_SVG_LOCK = `<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>`
const DC_SVG_DL = `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>`
const DC_SVG_STAR = `<svg width="9" height="9" viewBox="0 0 24 24" fill="#f59e0b" stroke="#f59e0b" stroke-width="1" aria-hidden="true"><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/></svg>`
const DC_SVG_CHECK = `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="20 6 9 17 4 12"/></svg>`

function fmtCount(n) {
  if (!n) return '0'
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1).replace(/\.0$/, '') + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1).replace(/\.0$/, '') + 'k'
  return String(n)
}

// ── Lock UI refresh ────────────────────────────────────────────────────────────

export function refreshDiscoverLock() {
  const prem = lp.fn.isPremium?.() ?? false
  document
    .querySelectorAll('.dc-feat-lock-note')
    .forEach((el) => el.classList.toggle('hidden', prem))
  document.querySelectorAll('.dc-download:not(.downloaded)').forEach((b) => {
    b.classList.toggle('unlocked', prem)
    b.classList.toggle('locked', !prem)
    b.innerHTML = prem ? DC_SVG_DL : DC_SVG_LOCK
  })
  document.querySelectorAll('.dc-feat-btn').forEach((b) => {
    b.classList.toggle('unlocked', prem)
    b.classList.toggle('locked', !prem)
    b.innerHTML = prem ? `${DC_SVG_DL} Download` : `${DC_SVG_LOCK} Unlock to Download`
  })
}

// ── Load and render ────────────────────────────────────────────────────────────

export async function onShowDiscover() {
  refreshDiscoverLock()
  if (!lp.discoverLoaded) await loadDiscover()
}

export async function loadDiscover() {
  const grid = document.getElementById('discover-grid')
  if (!grid) return
  grid.innerHTML = '<div class="discover-empty">Loading community wallpapers…</div>'
  try {
    const res = await fetch(`${API_BASE}/api/wallpapers?limit=${WALLPAPER_LIMIT}`)
    const data = await res.json()
    lp.discoverItems = (data && data.items) || []
  } catch (_) {
    lp.discoverItems = []
  }
  lp.discoverLoaded = true
  renderDiscoverPills()
  if (lp.discoverItems.length > 0) renderDiscoverFeatured(lp.discoverItems[0])
  renderDiscover(document.getElementById('discover-search')?.value || '')
}

export function renderDiscoverPills() {
  const container = document.getElementById('discover-pills')
  if (!container) return
  const allTags = [...new Set(lp.discoverItems.flatMap((i) => i.tags || []))]
  const staticFilters = [
    { id: 'all', label: 'All', icon: '' },
    {
      id: 'trending',
      label: 'Trending',
      icon: `<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="22 7 13.5 15.5 8.5 10.5 2 17"/><polyline points="16 7 22 7 22 13"/></svg>`,
    },
    { id: 'new', label: 'New', icon: '' },
  ]
  const pills = [...staticFilters, ...allTags.map((t) => ({ id: t, label: t, icon: '' }))]
  container.innerHTML = ''
  for (const p of pills) {
    const btn = document.createElement('button')
    btn.className = 'discover-pill' + (p.id === lp.discoverActiveFilter ? ' active' : '')
    btn.dataset.filter = p.id
    btn.innerHTML = p.icon + p.label
    btn.addEventListener('click', () => {
      lp.discoverActiveFilter = p.id
      container
        .querySelectorAll('.discover-pill')
        .forEach((el) => el.classList.toggle('active', el.dataset.filter === p.id))
      renderDiscover(document.getElementById('discover-search')?.value || '')
    })
    container.appendChild(btn)
  }
  container.classList.remove('hidden')
}

export function renderDiscoverFeatured(it) {
  const container = document.getElementById('discover-featured')
  if (!container) return
  const isLocked = !(lp.fn.isPremium?.() ?? false)
  const dlText = it.downloadCount
    ? fmtCount(it.downloadCount) + ' downloads'
    : 'Community wallpaper'
  const ratingText = it.avgRating != null ? ` · ★ ${Number(it.avgRating).toFixed(1)}` : ''
  const featDl = isDownloaded(it.id)
  const featBtnClass = isLocked ? 'locked' : 'unlocked'
  const featBtnHtml = featDl
    ? `${DC_SVG_CHECK} Apply Wallpaper`
    : isLocked
      ? `${DC_SVG_LOCK} Unlock to Download`
      : `${DC_SVG_DL} Download`
  container.innerHTML = `
    <div class="dc-featured">
      <img alt="${it.title || ''}">
      <div class="dc-feat-grad"></div>
      <div class="dc-feat-content">
        <span class="dc-feat-label">
          <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m12 3-1.912 5.813a2 2 0 0 1-1.275 1.275L3 12l5.813 1.912a2 2 0 0 1 1.275 1.275L12 21l1.912-5.813a2 2 0 0 1 1.275-1.275L21 12l-5.813-1.912a2 2 0 0 1-1.275-1.275L12 3Z"/></svg>
          Featured this week
        </span>
        <h2 class="dc-feat-title"></h2>
        <p class="dc-feat-sub">${dlText}${ratingText}</p>
        <p class="dc-feat-lock-note${isLocked ? '' : ' hidden'}">
          <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>
          Browsing is free · download from <strong>$4/mo</strong>
        </p>
        <button class="dc-feat-btn ${featBtnClass}">${featBtnHtml}</button>
      </div>
    </div>`
  container.querySelector('img').src = it.thumbnailUrl || ''
  container.querySelector('.dc-feat-title').textContent = it.title || 'Untitled'
  container.querySelector('.dc-feat-btn').addEventListener('click', () => {
    if (featDl) onDiscoverApply(it)
    else onDiscoverDownload(it, container.querySelector('.dc-feat-btn'))
  })
  container.classList.remove('hidden')
}

export function renderDiscover(search) {
  const grid = document.getElementById('discover-grid')
  if (!grid) return
  const q = (search || '').trim().toLowerCase()
  let items = [...lp.discoverItems]
  if (lp.discoverActiveFilter === 'trending') {
    items.sort((a, b) => (b.downloadCount || 0) - (a.downloadCount || 0))
  } else if (lp.discoverActiveFilter !== 'all' && lp.discoverActiveFilter !== 'new') {
    items = items.filter((i) => (i.tags || []).includes(lp.discoverActiveFilter))
  }
  if (q) {
    items = items.filter((i) => {
      const titleMatch = (i.title || '').toLowerCase().includes(q)
      const tagMatch = (i.tags || []).some((t) => t.toLowerCase().includes(q))
      return titleMatch || tagMatch
    })
  }
  if (!items.length) {
    grid.innerHTML = `<div class="discover-empty">${lp.discoverItems.length ? 'No matches for your search.' : 'No community wallpapers yet — check back soon.'}</div>`
    return
  }
  grid.innerHTML = ''
  const prem = lp.fn.isPremium?.() ?? false
  items.forEach((it, idx) => {
    const isVideo = (it.contentType || '').startsWith('video/')
    const isFeatured = idx === 0 && !q && lp.discoverActiveFilter !== 'trending'
    const card = document.createElement('div')
    card.className = 'discover-card' + (isFeatured ? ' featured' : '')
    card.dataset.discoverId = it.id
    const badgeHtml = isVideo
      ? `<span class="dc-badge video"><svg width="9" height="9" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m22 8-6 4 6 4V8z"/><rect x="2" y="5" width="14" height="14" rx="2"/></svg>Video</span>`
      : `<span class="dc-badge image"><svg width="9" height="9" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-5-5L5 21"/></svg>Img</span>`
    const avgRating = it.avgRating != null ? Number(it.avgRating).toFixed(1) : null
    const ratingTitle = avgRating
      ? `Rating: ${avgRating}/5 (${it.ratingCount} vote${it.ratingCount === 1 ? '' : 's'}) — click to rate`
      : 'No ratings yet — click to rate'
    const ratingContent = avgRating ? `${DC_SVG_STAR}${avgRating}` : DC_SVG_STAR
    const dl = isDownloaded(it.id)
    let dlBtnClass, dlBtnLabel, dlBtnSvg
    if (dl) {
      dlBtnClass = 'dc-download downloaded'
      dlBtnLabel = 'Apply wallpaper'
      dlBtnSvg = DC_SVG_CHECK
    } else if (prem) {
      dlBtnClass = 'dc-download unlocked'
      dlBtnLabel = 'Download wallpaper'
      dlBtnSvg = DC_SVG_DL
    } else {
      dlBtnClass = 'dc-download locked'
      dlBtnLabel = 'Subscribe to download'
      dlBtnSvg = DC_SVG_LOCK
    }
    card.innerHTML = `
      <img alt="" loading="lazy" draggable="false">
      <div class="dc-grad"></div>
      ${badgeHtml}
      <button class="${dlBtnClass}" type="button" aria-label="${dlBtnLabel}">${dlBtnSvg}</button>
      <div class="dc-info">
        <div class="dc-title${isFeatured ? ' large' : ''}"></div>
        <div class="dc-meta">
          <span class="dc-author"><span class="dc-avatar"></span>community</span>
          <span class="dc-stats">
            <button class="dc-stat-rating" title="${ratingTitle}">${ratingContent}</button>
            <span class="dc-stat-dl">${DC_SVG_DL}${fmtCount(it.downloadCount)}</span>
          </span>
        </div>
      </div>`
    card.querySelector('img').src = it.thumbnailUrl || ''
    card.querySelector('.dc-title').textContent = it.title || 'Untitled'
    card.querySelector('.dc-download').addEventListener('click', (e) => {
      e.stopPropagation()
      const btn = e.currentTarget
      if (btn.classList.contains('downloaded')) onDiscoverApply(it)
      else onDiscoverDownload(it, btn)
    })
    card.querySelector('.dc-stat-rating').addEventListener('click', (e) => {
      e.stopPropagation()
      onDiscoverRate(card, it)
    })
    card.addEventListener('click', (e) => {
      if (!e.target.closest('.dc-download, .dc-stat-rating, .dc-rate-picker')) {
        onDiscoverPreview(it, items, idx)
      }
    })
    grid.appendChild(card)
  })
}

// ── Rate ───────────────────────────────────────────────────────────────────────

export function onDiscoverRate(card, it) {
  if (!lp.fn.getToken?.()) {
    lp.fn.openLoginModal?.()
    return
  }
  if (card.querySelector('.dc-rate-picker')) return
  const picker = document.createElement('div')
  picker.className = 'dc-rate-picker'
  picker.innerHTML = `
    <div class="dc-rp-label">Rate this wallpaper</div>
    <div class="dc-rp-stars">
      ${[1, 2, 3, 4, 5].map((s) => `<button class="dc-rp-star" data-score="${s}" title="${s} star${s > 1 ? 's' : ''}">★</button>`).join('')}
    </div>`
  const stars = picker.querySelectorAll('.dc-rp-star')
  stars.forEach((star) => {
    star.addEventListener('mouseenter', () => {
      const n = Number(star.dataset.score)
      stars.forEach((s) => s.classList.toggle('active', Number(s.dataset.score) <= n))
    })
    star.addEventListener('mouseleave', () => stars.forEach((s) => s.classList.remove('active')))
    star.addEventListener('click', async (e) => {
      e.stopPropagation()
      picker.remove()
      try {
        const res = await fetch(`${API_BASE}/api/wallpapers/${it.id}/rate`, {
          method: 'POST',
          headers: {
            'content-type': 'application/json',
            authorization: `Bearer ${lp.fn.getToken()}`,
          },
          body: JSON.stringify({ score: Number(star.dataset.score) }),
        })
        if (res.ok) {
          const data = await res.json()
          it.avgRating = data.avgRating
          it.ratingCount = data.ratingCount
          const ratingBtn = card.querySelector('.dc-stat-rating')
          if (ratingBtn) {
            const avg = data.avgRating != null ? Number(data.avgRating).toFixed(1) : '–'
            ratingBtn.innerHTML = `${DC_SVG_STAR}${avg}`
            ratingBtn.title = `Rating: ${avg}/5 (${data.ratingCount} vote${data.ratingCount === 1 ? '' : 's'}) — click to rate`
          }
        }
      } catch (_) {}
    })
  })
  const closePicker = (e) => {
    if (!picker.contains(e.target)) {
      picker.remove()
      document.removeEventListener('click', closePicker)
    }
  }
  card.appendChild(picker)
  setTimeout(() => document.addEventListener('click', closePicker), 0)
}

// ── Preview overlay ────────────────────────────────────────────────────────────

export function onDiscoverPreview(it, itemList, idx) {
  lp._pvItems = itemList || lp.discoverItems
  lp._pvIndex = idx != null ? idx : lp._pvItems.findIndex((i) => i.id === it.id)
  if (lp._pvIndex < 0) lp._pvIndex = 0
  lp._pvSearch = ''
  _renderPreview()
}

function _renderPreview() {
  if (lp._pvOverlay) {
    lp._pvOverlay.remove()
    document.removeEventListener('keydown', lp._pvKeyHandler)
  }
  const baseItems = lp._pvSearch
    ? lp.discoverItems.filter((i) => {
        const q = lp._pvSearch.toLowerCase()
        return (
          (i.title || '').toLowerCase().includes(q) ||
          (i.tags || []).some((t) => t.toLowerCase().includes(q))
        )
      })
    : lp._pvItems
  if (baseItems.length === 0) return
  if (lp._pvIndex >= baseItems.length) lp._pvIndex = 0
  const it = baseItems[lp._pvIndex]
  const isLocked = !(lp.fn.isPremium?.() ?? false)
  const dl = isDownloaded(it.id)
  const avgRating = it.avgRating != null ? Number(it.avgRating).toFixed(1) : null
  const tags = it.tags || []
  let actionClass, actionHtml
  if (dl) {
    actionClass = 'dc-preview-btn success'
    actionHtml = `${DC_SVG_CHECK} Apply Wallpaper`
  } else if (isLocked) {
    actionClass = 'dc-preview-btn locked'
    actionHtml = `${DC_SVG_LOCK} Unlock to Download`
  } else {
    actionClass = 'dc-preview-btn primary'
    actionHtml = `${DC_SVG_DL} Download`
  }
  const tagsHtml = tags.map((t) => `<span class="dc-preview-tag">${escapeHtml(t)}</span>`).join('')
  const metaHtml = [
    avgRating ? `${DC_SVG_STAR} ${avgRating}` : '',
    it.downloadCount ? `${DC_SVG_DL} ${fmtCount(it.downloadCount)}` : '',
  ]
    .filter(Boolean)
    .join('<span class="meta-sep">·</span>')
  const ov = document.createElement('div')
  ov.className = 'dc-preview-overlay'
  ov.innerHTML = `
    <img class="dc-preview-bg" src="${it.thumbnailUrl || ''}" alt="" draggable="false">
    <div class="dc-preview-scrim"></div>
    <div class="dc-preview-top">
      <button class="dc-preview-close" aria-label="Close preview">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
      </button>
      <div class="dc-preview-search-wrap">
        <svg class="dc-preview-search-icon" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></svg>
        <input id="pv-search" class="dc-preview-search" type="search" placeholder="Search by name or tag…" value="${escapeHtml(lp._pvSearch)}" autocomplete="off">
      </div>
    </div>
    <div class="dc-preview-bottom">
      <div class="dc-preview-title">${escapeHtml(it.title || 'Untitled')}</div>
      ${tags.length ? `<div class="dc-preview-tags">${tagsHtml}</div>` : ''}
      ${metaHtml ? `<div class="dc-preview-meta">${metaHtml}</div>` : ''}
      <div class="dc-preview-actions">
        <button id="pv-action" class="${actionClass}">${actionHtml}</button>
        <button id="pv-back" class="dc-preview-btn secondary">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="15 18 9 12 15 6"/></svg>
          Back
        </button>
        <button id="pv-random" class="dc-preview-btn secondary">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="16 3 21 3 21 8"/><line x1="4" y1="20" x2="21" y2="3"/><polyline points="21 16 21 21 16 21"/><line x1="15" y1="15" x2="21" y2="21"/></svg>
          Random
        </button>
        <button id="pv-next" class="dc-preview-btn secondary">
          Next
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="9 18 15 12 9 6"/></svg>
        </button>
        <span class="dc-preview-counter">${lp._pvIndex + 1} / ${baseItems.length}</span>
      </div>
    </div>`
  const closePreview = () => {
    ov.remove()
    document.removeEventListener('keydown', lp._pvKeyHandler)
    lp._pvOverlay = null
  }
  ov.querySelector('.dc-preview-close')?.addEventListener('click', closePreview)
  ov.querySelector('#pv-action').addEventListener('click', async () => {
    if (isLocked) {
      closePreview()
      openSettingsTab('billing')
      return
    }
    const btn = ov.querySelector('#pv-action')
    if (dl) {
      closePreview()
      await onDiscoverApply(it)
    } else {
      await onDiscoverDownload(it, btn)
      lp._pvItems = baseItems
      _renderPreview()
    }
  })
  ov.querySelector('#pv-back').addEventListener('click', () => {
    lp._pvItems = baseItems
    lp._pvIndex = (lp._pvIndex - 1 + baseItems.length) % baseItems.length
    _renderPreview()
  })
  ov.querySelector('#pv-next').addEventListener('click', () => {
    lp._pvItems = baseItems
    lp._pvIndex = (lp._pvIndex + 1) % baseItems.length
    _renderPreview()
  })
  ov.querySelector('#pv-random').addEventListener('click', () => {
    lp._pvItems = baseItems
    lp._pvIndex = Math.floor(Math.random() * baseItems.length)
    _renderPreview()
  })
  const searchEl = ov.querySelector('#pv-search')
  searchEl?.addEventListener('input', (e) => {
    lp._pvSearch = e.target.value
    lp._pvIndex = 0
    _renderPreview()
    setTimeout(() => document.querySelector('#pv-search')?.focus(), 0)
  })
  lp._pvKeyHandler = (e) => {
    if (e.key === 'Escape') closePreview()
    if (!e.target.closest('#pv-search')) {
      if (e.key === 'ArrowLeft') ov.querySelector('#pv-back')?.click()
      if (e.key === 'ArrowRight') ov.querySelector('#pv-next')?.click()
    }
  }
  document.body.appendChild(ov)
  document.addEventListener('keydown', lp._pvKeyHandler)
  lp._pvOverlay = ov
}

// ── Download ───────────────────────────────────────────────────────────────────

export async function onDiscoverDownload(it, btn) {
  if (!lp.fn.getToken?.()) {
    lp.fn.openLoginModal?.()
    return
  }
  if (!(lp.fn.isPremium?.() ?? false)) {
    openSettingsTab('billing')
    return
  }
  if (btn) {
    btn.disabled = true
    btn.innerHTML = '<span class="lp-loading-text">Downloading…</span>'
  }
  status('Downloading…')
  let path
  try {
    path = await call('DownloadToTemp', it.downloadUrl, lp.fn.getToken(), it.id)
  } catch (e) {
    if (String(e).includes('premium_required')) {
      openSettingsTab('billing')
      status('')
    } else status('Download failed', 'error')
    if (btn) {
      btn.disabled = false
      btn.innerHTML = DC_SVG_DL
    }
    return
  }
  const isVideo = await call('IsVideoFile', path)
  const thumbnail = await call('GetThumbnail', path)
  setDownloadedItem(it.id, { path, thumbnail, title: it.title, isVideo, tags: it.tags || [] })
  await upsertRecent({
    fileKey: `discover:${it.id}`,
    filePath: path,
    cachedPath: path,
    isVideo,
    thumbnail,
    width: 0,
    height: 0,
  }).catch(() => {})
  lp.fn.pruneAndRefresh?.()
  status('Downloaded!', 'success', 2500)
  track('discover_download', { id: it.id })
  if (btn) {
    btn.disabled = false
    btn.className = 'dc-download downloaded'
    btn.setAttribute('aria-label', 'Apply wallpaper')
    btn.innerHTML = DC_SVG_CHECK
    btn.onclick = (e) => {
      e.stopPropagation()
      onDiscoverApply(it)
    }
  }
  document
    .querySelectorAll(`.discover-card[data-discover-id="${it.id}"] .dc-download`)
    .forEach((b) => {
      if (b !== btn) {
        b.className = 'dc-download downloaded'
        b.setAttribute('aria-label', 'Apply wallpaper')
        b.innerHTML = DC_SVG_CHECK
        b.onclick = (e) => {
          e.stopPropagation()
          onDiscoverApply(it)
        }
      }
    })
  if (lp.currentView === 'library') lp.fn.renderLibrary?.()
}

// ── Apply ──────────────────────────────────────────────────────────────────────

export async function onDiscoverApply(it) {
  if (!lp.fn.getToken?.()) {
    lp.fn.openLoginModal?.()
    return
  }
  if (!(lp.fn.isPremium?.() ?? false)) {
    openSettingsTab('billing')
    return
  }
  let dlInfo = getDownloadedItem(it.id)
  if (dlInfo) {
    const exists = await call('FileExists', dlInfo.path).catch(() => false)
    if (!exists) dlInfo = null
  }
  if (!dlInfo) {
    await onDiscoverDownload(it, null)
    dlInfo = getDownloadedItem(it.id)
    if (!dlInfo) return
  }
  const { path, isVideo, thumbnail } = dlInfo
  if (lp.monitors.length <= 1) {
    const target = lp.monitors[0]
    if (!target) {
      status('No display detected', 'error')
      return
    }
    await lp.fn.applyToMonitor?.(target, path, isVideo, thumbnail, it)
  } else {
    lp.fn.showMonitorPicker?.(it, path, isVideo, thumbnail)
  }
}

// ── Remote gallery drop onto monitor ──────────────────────────────────────────

export async function applyRemoteEntryToMonitor(entry, m) {
  if (!lp.fn.getToken?.()) {
    status('Sign in to apply community wallpapers', 'error')
    openSettingsTab('billing')
    return
  }
  if (!(lp.fn.isPremium?.() ?? false)) {
    openSettingsTab('billing')
    return
  }
  status('Downloading…')
  let path
  try {
    path = await call('DownloadToTemp', entry.downloadUrl, lp.fn.getToken(), entry.id)
  } catch (e) {
    if (String(e).includes('premium_required')) {
      openSettingsTab('billing')
      status('')
    } else status('Download failed', 'error')
    return
  }
  const isVideo = await call('IsVideoFile', path)
  const thumbnail = await call('GetThumbnail', path)
  let cached = path
  if (isVideo && lp.fn.extOf?.(path) !== 'gif') {
    lp.state[m.index] = { filePath: path, cachedPath: path, isVideo: true, ready: false, thumbnail }
    lp.fn.applyThumb?.(m.index, thumbnail, path, true)
    lp.fn.refreshApply?.()
    lp.fn.setEncoding?.(m.index, true, 0)
    try {
      cached = await call('PreprocessVideo', path, m.width, m.height)
    } catch (_) {
      lp.fn.cancelEncode?.(m.index)
      status('Encoding cancelled.', '')
      return
    }
    lp.fn.setEncoding?.(m.index, false, 100)
  }
  lp.state[m.index] = { filePath: path, cachedPath: cached, isVideo, ready: true, thumbnail }
  lp.fn.applyThumb?.(m.index, thumbnail, path, isVideo)
  lp.pendingChanges = true
  lp.fn.refreshApply?.()
  status('Ready — click Apply Wallpapers', '')
}

// ── Event listeners ────────────────────────────────────────────────────────────

document
  .getElementById('discover-search')
  ?.addEventListener('input', (e) => renderDiscover(e.target.value))
document.getElementById('discover-refresh')?.addEventListener('click', () => {
  lp.discoverLoaded = false
  lp.discoverActiveFilter = 'all'
  document.getElementById('discover-featured')?.classList.add('hidden')
  const pills = document.getElementById('discover-pills')
  if (pills) pills.classList.add('hidden')
  loadDiscover()
})

// Register in cross-module registry
lp.fn.onShowDiscover = onShowDiscover
lp.fn.refreshDiscoverLock = refreshDiscoverLock
lp.fn.onDiscoverPreview = onDiscoverPreview
lp.fn.onDiscoverApply = onDiscoverApply
lp.fn.onDiscoverDownload = onDiscoverDownload
lp.fn.applyRemoteEntryToMonitor = applyRemoteEntryToMonitor
