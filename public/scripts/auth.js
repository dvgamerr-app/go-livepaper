// Authentication: token management, session check, profile UI, SSO, user dropdown.

import { lp, call, API_BASE, TOKEN_KEY, USER_KEY } from '/scripts/store.js'
import { status } from '/scripts/ui.js'

// ── Token management ──────────────────────────────────────────────────────────

export function getToken() {
  return localStorage.getItem(TOKEN_KEY)
}
export function setToken(t) {
  localStorage.setItem(TOKEN_KEY, t)
}
export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

function getCachedUser() {
  try {
    return JSON.parse(localStorage.getItem(USER_KEY) || 'null')
  } catch {
    return null
  }
}
function setCachedUser(u) {
  localStorage.setItem(USER_KEY, JSON.stringify(u))
}
function clearCachedUser() {
  localStorage.removeItem(USER_KEY)
}

export function authHeaders() {
  const t = getToken()
  return t
    ? { Authorization: `Bearer ${t}`, 'Content-Type': 'application/json' }
    : { 'Content-Type': 'application/json' }
}

export function isPremium() {
  const t = lp.currentUser?.subscriptionTier
  return t === 'premium' || t === 'admin'
}

// ── Gravatar ──────────────────────────────────────────────────────────────────

async function gravatarUrl(email) {
  const e = email.trim().toLowerCase()
  const buf = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(e))
  const hash = Array.from(new Uint8Array(buf))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('')
  return `https://www.gravatar.com/avatar/${hash}?s=56&d=mp`
}

// ── Profile UI ────────────────────────────────────────────────────────────────

export async function updateProfileUI(user) {
  lp.currentUser = user || null
  const nameEl = document.getElementById('sidebar-name')
  const tierEl = document.getElementById('sidebar-tier')
  const loginBtn = document.getElementById('sidebar-login-btn')
  const chevron = document.getElementById('user-chevron')
  const avatarImg = document.getElementById('sidebar-avatar-img')

  if (user) {
    setCachedUser(user)
    if (nameEl) nameEl.textContent = user.name || user.email
    if (tierEl) {
      const t = user.subscriptionTier
      tierEl.textContent = t === 'admin' ? 'Admin' : t === 'premium' ? 'Premium' : 'Free'
    }
    if (loginBtn) loginBtn.classList.add('hidden')
    if (chevron) chevron.classList.remove('hidden')
    if (avatarImg && user.email) {
      avatarImg.src = await gravatarUrl(user.email)
      avatarImg.classList.remove('hidden')
      avatarImg.onerror = () => avatarImg.classList.add('hidden')
    }
  } else {
    clearCachedUser()
    closeDropdown()
    if (nameEl) nameEl.textContent = 'Personal'
    if (tierEl) tierEl.textContent = 'Free Lifetime'
    if (loginBtn) loginBtn.classList.remove('hidden')
    if (chevron) chevron.classList.add('hidden')
    if (avatarImg) {
      avatarImg.src = ''
      avatarImg.classList.add('hidden')
    }
  }
  lp.fn.refreshAuthDependentUI?.()
}

export async function checkSession() {
  if (!getToken()) {
    clearCachedUser()
    return updateProfileUI(null)
  }
  const cached = getCachedUser()
  if (cached) await updateProfileUI(cached)
  try {
    const res = await fetch(`${API_BASE}/api/auth/get-session`, { headers: authHeaders() })
    const data = await res.json()
    if (data?.user) await updateProfileUI(data.user)
    else {
      clearToken()
      await updateProfileUI(null)
    }
  } catch (_) {}
}

// ── Login modal ───────────────────────────────────────────────────────────────

const loginModal = document.getElementById('login-modal')
const loginCloseBtn = document.getElementById('login-modal-close')
const sidebarLoginBtn = document.getElementById('sidebar-login-btn')
const ssoButtons = document.getElementById('sso-buttons')
const ssoWaitingEl = document.getElementById('sso-waiting-state')
const ssoErrorEl = document.getElementById('sso-error')
const ssoErrorText = document.getElementById('sso-error-text')
const ssoWaitLabel = document.getElementById('sso-waiting-label')

function showSSOError(msg) {
  if (!ssoErrorEl || !ssoErrorText) return
  ssoErrorText.textContent = msg
  ssoErrorEl.classList.toggle('hidden', !msg)
  ssoErrorEl.classList.toggle('flex', !!msg)
}

function setWaiting(busy) {
  ssoButtons?.classList.toggle('hidden', busy)
  ssoButtons?.classList.toggle('flex', !busy)
  ssoWaitingEl?.classList.toggle('hidden', !busy)
  ssoWaitingEl?.classList.toggle('flex', busy)
  if (!busy) showSSOError('')
}

export function openLoginModal() {
  if (loginModal) loginModal.hidden = false
  setWaiting(false)
}

function closeLoginModal() {
  if (loginModal) loginModal.hidden = true
  stopPoll()
  setWaiting(false)
}

sidebarLoginBtn?.addEventListener('click', openLoginModal)

// ── User dropdown ─────────────────────────────────────────────────────────────

const userDropdown = document.getElementById('user-dropdown')

function closeDropdown() {
  if (userDropdown) userDropdown.hidden = true
}

document.getElementById('user-profile-row')?.addEventListener('click', (e) => {
  const lb = document.getElementById('sidebar-login-btn')
  if (lb && (e.target === lb || lb.contains(e.target))) return
  if (!getToken()) return
  if (userDropdown) userDropdown.hidden = !userDropdown.hidden
})

document.addEventListener('click', (e) => {
  const section = document.getElementById('user-profile-section')
  if (section && !section.contains(e.target)) closeDropdown()
})

document.getElementById('dd-settings')?.addEventListener('click', () => {
  closeDropdown()
  lp.fn.openSettings?.()
})

document.getElementById('dd-signout')?.addEventListener('click', () => {
  closeDropdown()
  document.getElementById('signout-confirm-modal').hidden = false
})

document.getElementById('signout-cancel-btn')?.addEventListener('click', () => {
  document.getElementById('signout-confirm-modal').hidden = true
})

document.getElementById('signout-confirm-btn')?.addEventListener('click', async () => {
  const btn = document.getElementById('signout-confirm-btn')
  btn.disabled = true
  btn.textContent = '…'
  try {
    await fetch(`${API_BASE}/api/auth/sign-out`, {
      method: 'POST',
      headers: authHeaders(),
      body: '{}',
    })
  } catch (_) {}
  clearToken()
  await updateProfileUI(null)
  document.getElementById('signout-confirm-modal').hidden = true
  btn.disabled = false
  btn.textContent = 'Sign out'
})

loginCloseBtn?.addEventListener('click', closeLoginModal)
loginModal?.addEventListener('click', (e) => {
  if (e.target === loginModal) closeLoginModal()
})
document.getElementById('sso-cancel')?.addEventListener('click', () => {
  stopPoll()
  setWaiting(false)
})

// ── SSO — external browser + state polling ────────────────────────────────────

function stopPoll() {
  if (lp.pollTimer) {
    clearInterval(lp.pollTimer)
    lp.pollTimer = null
  }
  if (lp.pollTimeout) {
    clearTimeout(lp.pollTimeout)
    lp.pollTimeout = null
  }
  lp.activeState = null
}

async function startSSO(provider, { onSuccess, onFail, silent = false } = {}) {
  stopPoll()
  showSSOError('')
  lp.activeState = crypto.randomUUID().replace(/-/g, '')
  const callbackURL = encodeURIComponent(`${API_BASE}/auth-callback?state=${lp.activeState}`)
  const url = `${API_BASE}/api/auth/sign-in/social?provider=${provider}&callbackURL=${callbackURL}`
  call('OpenExternal', url).catch(() => {})

  if (!silent) {
    if (ssoWaitLabel)
      ssoWaitLabel.textContent = `Waiting for ${provider === 'github' ? 'GitHub' : 'Google'}…`
    setWaiting(true)
  }

  const capturedState = lp.activeState
  lp.pollTimer = setInterval(async () => {
    if (Date.now() < lp.pollPauseUntil) return
    try {
      const res = await fetch(`${API_BASE}/api/auth/exchange?state=${capturedState}`)
      if (res.ok) {
        const { token, user } = await res.json()
        stopPoll()
        setToken(token)
        await updateProfileUI(user)
        if (!silent) closeLoginModal()
        if (onSuccess) await onSuccess(user)
      } else if (res.status === 429) {
        lp.pollPauseUntil = Date.now() + 10_000
      } else if (res.status >= 400 && res.status < 500 && res.status !== 404) {
        stopPoll()
        if (!silent) {
          setWaiting(false)
          showSSOError('Sign-in failed — please try again.')
        }
        if (onFail) onFail('failed')
      }
    } catch (_) {}
  }, 4000)

  lp.pollTimeout = setTimeout(() => {
    if (lp.activeState === capturedState) {
      stopPoll()
      if (!silent) {
        setWaiting(false)
        showSSOError('Sign-in timed out — please try again.')
      }
      if (onFail) onFail('timeout')
    }
  }, 180_000)
}

document.getElementById('sso-github')?.addEventListener('click', () => startSSO('github'))
document.getElementById('sso-google')?.addEventListener('click', () => startSSO('google'))

// ── Admin UI ──────────────────────────────────────────────────────────────────

export function refreshAdminUI() {
  const isAdmin = lp.currentUser?.subscriptionTier === 'admin'
  const section = document.getElementById('admin-sidebar-section')
  const items = document.getElementById('admin-nav-items')
  if (section) section.classList.toggle('hidden', !isAdmin)
  if (items) {
    items.classList.toggle('hidden', !isAdmin)
    items.classList.toggle('flex', isAdmin)
  }
}

// ── Auth-dependent UI refresh ─────────────────────────────────────────────────

export function refreshAuthDependentUI() {
  const lic = document.getElementById('about-license')
  const licDesc = document.getElementById('about-license-desc')
  if (lic) {
    const prem = isPremium()
    lic.textContent = prem ? 'Community' : 'Free Lifetime'
    if (licDesc)
      licDesc.textContent = prem
        ? 'Community access — apply community wallpapers.'
        : 'Free — all local features, unlimited.'
  }
  lp.fn.refreshDiscoverLock?.()
  lp.fn.refreshGalleryLocks?.()
  lp.fn.refreshAdminUI?.()
  const billingPanel = document.querySelector('[data-spanel="billing"]')
  if (lp.currentView === 'settings' && billingPanel && !billingPanel.hidden) lp.fn.onShowBilling?.()
  const connPanel = document.querySelector('[data-spanel="connections"]')
  if (lp.currentView === 'settings' && connPanel && !connPanel.hidden) lp.fn.onShowConnections?.()
}

// Register in cross-module registry
lp.fn.getToken = getToken
lp.fn.setToken = setToken
lp.fn.clearToken = clearToken
lp.fn.authHeaders = authHeaders
lp.fn.isPremium = isPremium
lp.fn.updateProfileUI = updateProfileUI
lp.fn.checkSession = checkSession
lp.fn.openLoginModal = openLoginModal
lp.fn.refreshAuthDependentUI = refreshAuthDependentUI
lp.fn.refreshAdminUI = refreshAdminUI
lp.fn.startSSO = startSSO
