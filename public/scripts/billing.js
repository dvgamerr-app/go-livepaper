// Billing: subscription status, pricing UI, star discount.

import { lp, call, apiFetch, GH_REPO_DEFAULT } from '/scripts/store.js'
import { status, track, escapeHtml } from '/scripts/ui.js'

// ── Connections shared data ────────────────────────────────────────────────────

export function prefetchConnections() {
  if (!lp.fn.getToken?.() || lp.connectionsPromise) return
  lp.connectionsPromise = apiFetch('/api/connections', { headers: lp.fn.authHeaders?.() || {} })
    .then((r) => (r.ok ? r.json() : null))
    .catch(() => null)
    .then((data) => {
      if (!data) lp.connectionsPromise = null
      return data
    })
}

export function invalidateConnections() {
  lp.connectionsPromise = null
}

// ── Billing display ────────────────────────────────────────────────────────────

export async function onShowBilling() {
  const signin = document.getElementById('billing-signin')
  const free = document.getElementById('billing-free')
  const active = document.getElementById('billing-active')
  if (!signin || !free || !active) return
  if (!lp.fn.getToken?.()) {
    signin.classList.remove('hidden')
    signin.classList.add('flex')
    free.classList.add('hidden')
    active.classList.add('hidden')
    return
  }
  signin.classList.add('hidden')
  signin.classList.remove('flex')
  if (lp.currentUser?.subscriptionTier === 'admin') {
    renderBillingAdmin()
    return
  }
  let st = null
  try {
    const res = await apiFetch('/api/billing/status', { headers: lp.fn.authHeaders?.() || {} })
    st = res.ok ? await res.json() : null
  } catch (_) {}
  if (st && st.tier === 'premium' && st.subscription && st.subscription.status === 'active') {
    renderBillingActive(st.subscription)
  } else {
    await renderBillingFree()
  }
}

export function renderBillingAdmin() {
  document.getElementById('billing-active').classList.remove('hidden')
  document.getElementById('billing-free').classList.add('hidden')
  const planEl = document.getElementById('billing-active-plan')
  if (planEl) {
    const infoLine = planEl.closest('p')
    if (infoLine) infoLine.textContent = 'Admin access'
  }
  document.getElementById('billing-active-starred')?.classList.add('hidden')
  document.getElementById('billing-cancel-btn')?.classList.add('hidden')
}

export function renderBillingActive(sub) {
  document.getElementById('billing-active').classList.remove('hidden')
  document.getElementById('billing-free').classList.add('hidden')
  document.getElementById('billing-active-plan').textContent =
    sub.plan === 'monthly' ? 'Monthly' : sub.plan || 'Monthly'
  document.getElementById('billing-active-price').textContent = `$${sub.priceUsd}`
  document.getElementById('billing-active-renew').textContent = sub.currentPeriodEnd
    ? new Date(sub.currentPeriodEnd).toLocaleDateString()
    : '—'
  document.getElementById('billing-active-starred').classList.toggle('hidden', !sub.starred)
  document.getElementById('billing-cancel-btn')?.classList.remove('hidden')
}

export async function renderBillingFree() {
  document.getElementById('billing-free').classList.remove('hidden')
  document.getElementById('billing-active').classList.add('hidden')
  if (!lp.connectionsPromise) prefetchConnections()
  const c = await (lp.connectionsPromise || Promise.resolve(null))
  const starred = c?.starred || false
  const repo = c?.repo || GH_REPO_DEFAULT
  const pr = { baseUsd: 9, priceUsd: starred ? 4 : 9, starred, repo }
  lp.billingPricing = pr
  applyPricingUI(pr)
}

export function applyPricingUI(pr) {
  const repo = (pr && pr.repo) || GH_REPO_DEFAULT
  const repoEl = document.getElementById('billing-repo')
  if (repoEl) repoEl.textContent = repo
  const price = pr ? pr.priceUsd : 9
  const base = pr ? pr.baseUsd : 9
  const starred = pr ? pr.starred : false
  document.getElementById('billing-price').textContent = `$${price}`
  document.getElementById('billing-subscribe-price').textContent = `$${price}`
  const strike = document.getElementById('billing-price-strike')
  strike.classList.toggle('hidden', !starred)
  strike.textContent = `$${base}`
  const note = document.getElementById('billing-price-note')
  const statusEl2 = document.getElementById('billing-star-status')
  const starCard = document.getElementById('billing-star-card')
  const starIconWrap = document.getElementById('billing-star-icon-wrap')
  const starTitle = document.getElementById('billing-star-title')
  const starBtns = document.getElementById('billing-star-btns')
  if (starred) {
    note.textContent = 'GitHub-star discount applied'
    statusEl2.innerHTML = `You starred <span class="font-mono">${escapeHtml(repo)}</span> — enjoy $${price}/mo.`
    starCard?.classList.remove('border-lp-border', 'bg-lp-surface/40')
    starCard?.classList.add('border-[rgba(52,211,153,0.3)]', 'bg-[rgba(52,211,153,0.07)]')
    if (starIconWrap) {
      starIconWrap.className =
        'grid h-10 w-10 shrink-0 place-items-center rounded-lg bg-[rgba(52,211,153,0.15)] text-lp-success'
      starIconWrap.innerHTML =
        '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="20 6 9 17 4 12"></polyline></svg>'
    }
    if (starTitle) {
      starTitle.textContent = 'Starred — $4/mo unlocked'
      starTitle.className = 'text-lp-success text-[12.5px] font-semibold'
    }
    starBtns?.classList.add('hidden')
  } else {
    note.textContent = 'Apply any community wallpaper.'
    statusEl2.innerHTML = `Star <span class="font-mono">${escapeHtml(repo)}</span> on GitHub to drop the price from $${base} to $4.`
    starCard?.classList.add('border-lp-border', 'bg-lp-surface/40')
    starCard?.classList.remove('border-[rgba(52,211,153,0.3)]', 'bg-[rgba(52,211,153,0.07)]')
    if (starIconWrap) {
      starIconWrap.className =
        'grid h-10 w-10 shrink-0 place-items-center rounded-lg bg-[rgba(250,204,21,0.14)] text-[#facc15]'
      starIconWrap.innerHTML =
        '<svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"></polygon></svg>'
    }
    if (starTitle) {
      starTitle.textContent = 'Star the repo, save $5/mo'
      starTitle.className = 'text-lp-text text-[12.5px] font-semibold'
    }
    starBtns?.classList.remove('hidden')
  }
}

// ── Event listeners ────────────────────────────────────────────────────────────

document
  .getElementById('billing-signin-btn')
  ?.addEventListener('click', () => lp.fn.openLoginModal?.())
document.getElementById('billing-star-btn')?.addEventListener('click', () => {
  const repo = (lp.billingPricing && lp.billingPricing.repo) || GH_REPO_DEFAULT
  call('OpenExternal', `https://github.com/${repo}`).catch(() => {})
})
document.getElementById('billing-recheck-btn')?.addEventListener('click', async () => {
  invalidateConnections()
  try {
    const r = await (
      await apiFetch('/api/billing/star-check', {
        method: 'POST',
        headers: lp.fn.authHeaders?.() || {},
      })
    ).json()
    if (lp.billingPricing) {
      lp.billingPricing.starred = r.starred
      lp.billingPricing.priceUsd = r.priceUsd
    }
    applyPricingUI(lp.billingPricing || { priceUsd: r.priceUsd, baseUsd: 9, starred: r.starred })
    status(
      r.starred ? 'Star confirmed — $4/mo unlocked' : 'No star found yet — star the repo first',
      r.starred ? 'success' : 'error'
    )
  } catch (_) {
    status('Could not verify star', 'error')
  }
})
document.getElementById('billing-subscribe-btn')?.addEventListener('click', async () => {
  const btn = document.getElementById('billing-subscribe-btn')
  btn.disabled = true
  try {
    const r = await (
      await apiFetch('/api/billing/subscribe', {
        method: 'POST',
        headers: lp.fn.authHeaders?.() || {},
      })
    ).json()
    if (r && r.ok) {
      status('Community access active', 'success')
      track('subscribe', { priceUsd: r.subscription && r.subscription.priceUsd })
      await lp.fn.checkSession?.()
      onShowBilling()
    } else {
      status('Subscription failed', 'error')
    }
  } catch (_) {
    status('Subscription failed', 'error')
  }
  btn.disabled = false
})
document.getElementById('billing-cancel-btn')?.addEventListener('click', async () => {
  const btn = document.getElementById('billing-cancel-btn')
  btn.disabled = true
  try {
    const res = await apiFetch('/api/billing/cancel', {
      method: 'POST',
      headers: lp.fn.authHeaders?.() || {},
    })
    const r = await res.json()
    if (!res.ok || !r.ok) throw new Error(r.error || 'cancel failed')
    status('Subscription canceled', 'success', 3000)
    await lp.fn.checkSession?.()
    await onShowBilling()
  } catch (_) {
    status('Cancel failed', 'error', 3000)
  }
  btn.disabled = false
})

// Register in cross-module registry
lp.fn.prefetchConnections = prefetchConnections
lp.fn.invalidateConnections = invalidateConnections
lp.fn.onShowBilling = onShowBilling
