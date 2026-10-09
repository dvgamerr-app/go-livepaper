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
  appSettings: null,
  galleryCursor: -1,
  currentView: 'displays',

  // Gallery
  galleryItems: [],

  // Gallery strip preview state
  _gpItems: [],
  _gpIndex: 0,
  _pvOverlay: null,
  _pvKeyHandler: null,

  // Settings hotkey capture
  capturing: null,
  resizeTimer: null,

  // Cross-module function registry — modules register here during init
  fn: {},
}

// ── Constants ─────────────────────────────────────────────────────────────────

export const STORAGE_KEY = 'livepaper_state_v1'
export const IDB_NAME = 'livepaper'
export const IDB_VERSION = 2
export const STORE_RECENT = 'recent'
export const RECENT_LIMIT = 50
