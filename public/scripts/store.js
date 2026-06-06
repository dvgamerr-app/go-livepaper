// Shared mutable state. All feature modules import from here.

let _Call = null
let _Events = null
const _pendingEvents = []

export function initWails(Call, Events) {
  _Call = Call
  _Events = Events
  for (const { name, handler } of _pendingEvents) Events.On(name, handler)
  _pendingEvents.length = 0
}

export async function call(method, ...args) {
  return await _Call.ByName('main.AppService.' + method, ...args)
}

export function onEvent(name, handler) {
  if (_Events) _Events.On(name, handler)
  else _pendingEvents.push({ name, handler })
}

export const lp = {
  // Monitor wallpaper assignments: monitorIndex -> { filePath, cachedPath, isVideo, ready, thumbnail }
  state: {},
  monitors: [],
  lastAppliedState: {},
  pendingChanges: false,

  // App globals
  currentUser: null,
  appSettings: null,
  appVersion: '',
  galleryCursor: -1,
  currentView: 'displays',

  // Gallery
  galleryItems: [],

  // Discover
  discoverItems: [],
  discoverLoaded: false,
  discoverActiveFilter: 'all',

  // Storage (admin)
  storageItems: [],

  // Billing / connections
  billingPricing: null,
  connectionsPromise: null,

  // SSO polling
  pollTimer: null,
  pollTimeout: null,
  activeState: null,
  pollPauseUntil: 0,

  // Gallery strip preview state
  _gpItems: [],
  _gpIndex: 0,

  // Discover full preview state
  _pvItems: [],
  _pvIndex: 0,
  _pvSearch: '',
  _pvOverlay: null,
  _pvKeyHandler: null,

  // Upload modal state
  _uploadFilePath: '',
  _uploadTier: 'free',

  // Settings hotkey capture
  capturing: null,
  resizeTimer: null,

  // Cross-module function registry — modules register here during init
  fn: {},
}

// ── Constants ─────────────────────────────────────────────────────────────────

export const API_BASE_KEY = 'livepaper_api_base'

function resolveApiBase() {
  try {
    const override = localStorage.getItem(API_BASE_KEY)?.trim().replace(/\/+$/, '')
    if (override) return override
  } catch (_) {}
  return 'https://sso.dvgamerr.app'
}

export const API_BASE = resolveApiBase()
export const TOKEN_KEY = 'livepaper_auth_token'
export const USER_KEY = 'livepaper_auth_user'
export const DOWNLOADED_KEY = 'livepaper_downloaded_v1'
export const STORAGE_KEY = 'livepaper_state_v1'
export const WALLPAPER_LIMIT = 60
export const GH_REPO_DEFAULT = 'dvgamerr/go-livepaper'
export const IDB_NAME = 'livepaper'
export const IDB_VERSION = 2
export const STORE_RECENT = 'recent'
export const RECENT_LIMIT = 50

export async function apiFetch(path, options = {}, timeoutMs = 8000) {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), timeoutMs)
  try {
    return await fetch(`${API_BASE}${path}`, { ...options, signal: controller.signal })
  } finally {
    clearTimeout(timer)
  }
}
