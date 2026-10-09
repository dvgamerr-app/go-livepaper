import { expect, test } from 'bun:test'
import { plugin } from 'bun'
import { resolve } from 'node:path'

// The browser modules import '/scripts/...'; map that root to public/scripts.
plugin({
  name: 'public-scripts',
  setup(build) {
    build.onResolve({ filter: /^\/scripts\// }, (args) => ({
      path: resolve(import.meta.dir, '../public', args.path.slice(1)),
    }))
  },
})

const { initWails } = await import('../public/scripts/store.js')

const deleted = []
globalThis.indexedDB = {
  open() {
    const req = {}
    setTimeout(() => {
      req.onsuccess({
        target: {
          result: {
            transaction: () => ({ objectStore: () => ({ delete: (id) => deleted.push(id) }) }),
          },
        },
      })
    }, 0)
    return req
  },
}

const { dropMissingFiles } = await import('../public/scripts/db.js')

test('dropMissingFiles removes history rows whose file is gone', async () => {
  const present = new Set(['C:\a.png'])
  initWails({ ByName: async (_m, path) => present.has(path) }, { On() {} })
  const items = [
    { id: 1, filePath: 'C:\a.png' },
    { id: 2, filePath: 'C:\bin\data\old' },
  ]
  const kept = await dropMissingFiles(items)
  await new Promise((r) => setTimeout(r, 10))
  expect(kept.map((i) => i.id)).toEqual([1])
  expect(deleted).toEqual([2])
})

test('dropMissingFiles keeps rows when the existence check fails', async () => {
  initWails({ ByName: async () => Promise.reject(new Error('x')) }, { On() {} })
  const kept = await dropMissingFiles([{ id: 3, filePath: 'C:\b.png' }])
  expect(kept).toHaveLength(1)
})
