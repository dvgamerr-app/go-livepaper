// Connections: GitHub integration, star status.

import { lp, call, API_BASE, GH_REPO_DEFAULT } from '/scripts/store.js'
import { status } from '/scripts/ui.js'

// ── Show connections panel ─────────────────────────────────────────────────────

export async function onShowConnections() {
  const signin = document.getElementById('connections-signin')
  const content = document.getElementById('connections-content')
  if (!signin || !content) return
  if (!lp.fn.getToken?.()) {
    signin.classList.remove('hidden'); signin.classList.add('flex')
    content.classList.add('hidden')
    return
  }
  signin.classList.add('hidden'); signin.classList.remove('flex')
  content.classList.remove('hidden')
  lp.fn.prefetchConnections?.()
  const c = await (lp.connectionsPromise || Promise.resolve(null))
  renderConnections(c)
}

export function renderConnections(c) {
  const ghStatus = document.getElementById('gh-status')
  const ghBtn = document.getElementById('gh-connect-btn')
  const starStatus = document.getElementById('conn-star-status')
  const repoEl = document.getElementById('conn-repo')
  if (!c) {
    if (ghStatus) {
      ghStatus.textContent = 'Unable to load — check your connection and try again.'
      ghStatus.className = 'setting-desc text-lp-warn'
    }
    return
  }
  const gh = c.github
  if (repoEl && c.repo) repoEl.textContent = c.repo
  if (ghStatus && ghBtn) {
    if (gh && gh.connected) {
      ghStatus.textContent = gh.login ? `Connected as @${gh.login}` : 'Connected'
      ghStatus.className = 'setting-desc text-lp-success'
      ghBtn.textContent = 'Reconnect'
    } else {
      ghStatus.textContent = 'Not connected — link GitHub to unlock the star discount.'
      ghStatus.className = 'setting-desc'
      ghBtn.textContent = 'Connect GitHub'
    }
  }
  const starBtn = document.getElementById('conn-star-btn')
  if (starStatus) {
    const starred = c.starred
    starStatus.textContent = starred
      ? 'You starred the repo — $4/mo unlocked.'
      : 'Not starred yet — star the repo to save $5/mo.'
    starStatus.className = 'setting-desc' + (starred ? ' text-lp-success' : '')
    if (starBtn) setConnStarBtn(starBtn, starred)
  }
}

export function setConnStarBtn(btn, starred) {
  const starSvg = '<svg width="13" height="13" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"></polygon></svg>'
  if (starred) {
    btn.innerHTML = starSvg + ' Starred'
    btn.className = 'inline-flex h-9 cursor-default items-center gap-1.5 rounded-md border border-[rgba(52,211,153,0.4)] bg-[rgba(52,211,153,0.1)] px-3 text-[12px] font-medium text-lp-success'
    btn.disabled = true
  } else {
    btn.innerHTML = starSvg + ' Star on GitHub'
    btn.className = 'text-lp-text inline-flex h-9 cursor-pointer items-center gap-1.5 rounded-md border border-white/[0.12] bg-white/[0.04] px-3 text-[12px] font-medium transition-colors hover:bg-white/[0.08]'
    btn.disabled = false
  }
}

// ── Event listeners ────────────────────────────────────────────────────────────

document.getElementById('connections-signin-btn')?.addEventListener('click', () => lp.fn.openLoginModal?.())
document.getElementById('gh-connect-btn')?.addEventListener('click', () => {
  const btn = document.getElementById('gh-connect-btn')
  const origText = btn?.textContent?.trim() || 'Connect GitHub'
  if (btn) { btn.disabled = true; btn.textContent = 'Connecting…' }
  lp.fn.startSSO?.('github', {
    silent: true,
    onSuccess: async () => {
      lp.fn.invalidateConnections?.()
      lp.fn.prefetchConnections?.()
      await onShowConnections()
      if (btn) btn.disabled = false
      status('GitHub connected', 'success', 3000)
    },
    onFail: () => {
      if (btn) { btn.disabled = false; btn.textContent = origText }
      status('GitHub connection failed — try again', 'error')
    },
  })
})
document.getElementById('conn-star-btn')?.addEventListener('click', () => {
  const repo = document.getElementById('conn-repo')?.textContent?.trim() || GH_REPO_DEFAULT
  call('OpenExternal', `https://github.com/${repo}`).catch(() => {})
})
document.getElementById('conn-recheck-btn')?.addEventListener('click', async () => {
  const btn = document.getElementById('conn-recheck-btn')
  if (btn) { btn.disabled = true; btn.textContent = 'Checking…' }
  lp.fn.invalidateConnections?.()
  try {
    const r = await (await fetch(`${API_BASE}/api/billing/star-check`, { method: 'POST', headers: lp.fn.authHeaders?.() || {} })).json()
    const starStatus = document.getElementById('conn-star-status')
    const starBtn2 = document.getElementById('conn-star-btn')
    if (starStatus) {
      starStatus.textContent = r.starred
        ? 'You starred the repo — $4/mo unlocked.'
        : 'Not starred yet — star the repo to save $5/mo.'
      starStatus.className = 'setting-desc' + (r.starred ? ' text-lp-success' : '')
    }
    if (starBtn2) setConnStarBtn(starBtn2, r.starred)
    status(r.starred ? 'Star confirmed — $4/mo unlocked' : 'No star found yet — star the repo first', r.starred ? 'success' : 'error')
  } catch (_) { status('Could not verify star', 'error') }
  if (btn) { btn.disabled = false; btn.textContent = 'Re-check' }
})

// Register in cross-module registry
lp.fn.onShowConnections = onShowConnections
