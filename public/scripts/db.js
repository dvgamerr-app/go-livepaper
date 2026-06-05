// IndexedDB (recent wallpaper history) + localStorage (downloaded items) helpers.

import { lp, IDB_NAME, IDB_VERSION, STORE_RECENT, RECENT_LIMIT, DOWNLOADED_KEY } from '/scripts/store.js'

// ── IndexedDB ─────────────────────────────────────────────────────────────────

export function openDB() {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(IDB_NAME, IDB_VERSION)
    req.onupgradeneeded = (e) => {
      const db = e.target.result
      if (db.objectStoreNames.contains(STORE_RECENT)) db.deleteObjectStore(STORE_RECENT)
      const store = db.createObjectStore(STORE_RECENT, { keyPath: 'id', autoIncrement: true })
      store.createIndex('fileKey', 'fileKey', { unique: true })
      store.createIndex('filePath', 'filePath', { unique: false })
    }
    req.onsuccess = (e) => resolve(e.target.result)
    req.onerror = (e) => reject(e.target.error)
  })
}

export async function upsertRecent(entry) {
  const db = await openDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_RECENT, 'readwrite')
    const store = tx.objectStore(STORE_RECENT)
    const getReq = store.index('fileKey').get(entry.fileKey)
    getReq.onsuccess = () => {
      const existing = getReq.result
      const record = { ...entry, appliedAt: Date.now() }
      if (existing) { record.id = existing.id; store.put(record) }
      else store.add(record)
    }
    tx.oncomplete = () => resolve()
    tx.onerror = (e) => reject(e.target.error)
  })
}

export async function pruneRecent() {
  const db = await openDB()
  return new Promise((resolve) => {
    const tx = db.transaction(STORE_RECENT, 'readwrite')
    const store = tx.objectStore(STORE_RECENT)
    store.count().onsuccess = (e) => {
      const total = e.target.result
      if (total <= RECENT_LIMIT) return
      const del = total - RECENT_LIMIT
      let deleted = 0
      store.openCursor().onsuccess = (ce) => {
        const cursor = ce.target.result
        if (!cursor || deleted >= del) return
        cursor.delete(); deleted++; cursor.continue()
      }
    }
    tx.oncomplete = () => resolve()
  })
}

export async function loadGalleryItems() {
  const db = await openDB()
  return new Promise((resolve) => {
    const tx = db.transaction(STORE_RECENT, 'readonly')
    const store = tx.objectStore(STORE_RECENT)
    const items = []
    store.openCursor(null, 'prev').onsuccess = (e) => {
      const cursor = e.target.result
      if (cursor) { items.push(cursor.value); cursor.continue() }
      else resolve(items)
    }
    tx.onerror = () => resolve([])
  })
}

export async function clearRecentHistory() {
  const db = await openDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_RECENT, 'readwrite')
    tx.objectStore(STORE_RECENT).clear()
    tx.oncomplete = () => resolve()
    tx.onerror = (e) => reject(e.target.error)
  })
}

// ── Downloaded wallpapers tracking (localStorage) ─────────────────────────────

export function getDownloadedMap() {
  try { return JSON.parse(localStorage.getItem(DOWNLOADED_KEY) || '{}') } catch { return {} }
}

export function setDownloadedItem(id, info) {
  const m = getDownloadedMap()
  m[id] = info
  localStorage.setItem(DOWNLOADED_KEY, JSON.stringify(m))
}

export function getDownloadedItem(id) { return getDownloadedMap()[id] || null }
export function isDownloaded(id) { return !!getDownloadedItem(id) }

// Register in cross-module registry
lp.fn.upsertRecent = upsertRecent
lp.fn.pruneRecent = pruneRecent
lp.fn.loadGalleryItems = loadGalleryItems
lp.fn.clearRecentHistory = clearRecentHistory
lp.fn.getDownloadedItem = getDownloadedItem
lp.fn.setDownloadedItem = setDownloadedItem
lp.fn.isDownloaded = isDownloaded
