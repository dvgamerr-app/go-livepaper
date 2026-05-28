import { mkdirSync, writeFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { deflateSync } from 'node:zlib'

const SIZES = [16, 32, 64, 128, 256]
const __dirname = path.dirname(fileURLToPath(import.meta.url))
const outDir = path.join(__dirname, '..', 'public')

const GRADIENT_START = [0x2f, 0x8a, 0xef]
const GRADIENT_END = [0xa8, 0x55, 0xf7]
const WHITE = [0xff, 0xff, 0xff]
const BG_RADIUS_RATIO = 0.22
const ICON_INSET_RATIO = 24 / 256
const ICON_BOUNDS = {
  minX: 0.75,
  minY: 0.75,
  width: 22.5,
  height: 22.5,
}

const MAIN_SPARKLE = [
  [11.017, 2.814],
  [12.983, 2.814],
  [14.034, 8.372],
  [15.628, 9.966],
  [21.186, 11.017],
  [21.186, 12.983],
  [15.628, 14.034],
  [14.034, 15.628],
  [12.983, 21.186],
  [11.017, 21.186],
  [9.966, 15.628],
  [8.372, 14.034],
  [2.814, 12.983],
  [2.814, 11.017],
  [8.372, 9.966],
  [9.966, 8.372],
]

const PLUS_SEGMENTS = [
  [
    [20, 2],
    [20, 6],
  ],
  [
    [18, 4],
    [22, 4],
  ],
]

const DOT_CIRCLE = { cx: 4, cy: 20, r: 2 }

const crcTable = new Uint32Array(256).map((_, n) => {
  let c = n
  for (let k = 0; k < 8; k += 1) {
    c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1
  }
  return c >>> 0
})

function clamp(value, min, max) {
  return Math.max(min, Math.min(max, value))
}

function lerp(a, b, t) {
  return a + (b - a) * t
}

function alphaFromDistance(distance) {
  return clamp(0.5 - distance, 0, 1)
}

function roundedRectCoverage(x, y, left, top, right, bottom, radius) {
  const cx = (left + right) * 0.5
  const cy = (top + bottom) * 0.5
  const halfW = (right - left) * 0.5 - radius
  const halfH = (bottom - top) * 0.5 - radius
  const qx = Math.abs(x - cx) - halfW
  const qy = Math.abs(y - cy) - halfH
  const ox = Math.max(qx, 0)
  const oy = Math.max(qy, 0)
  const signedDistance = Math.hypot(ox, oy) + Math.min(Math.max(qx, qy), 0) - radius
  return alphaFromDistance(signedDistance)
}

function distanceToSegment(px, py, ax, ay, bx, by) {
  const abx = bx - ax
  const aby = by - ay
  const apx = px - ax
  const apy = py - ay
  const denom = abx * abx + aby * aby
  const t = denom === 0 ? 0 : clamp((apx * abx + apy * aby) / denom, 0, 1)
  const qx = ax + abx * t
  const qy = ay + aby * t
  return Math.hypot(px - qx, py - qy)
}

function strokeCoverage(distance, strokeWidth) {
  return alphaFromDistance(distance - strokeWidth * 0.5)
}

function createTransform(size) {
  const inset = size * ICON_INSET_RATIO
  const usable = size - inset * 2
  const scale = Math.min(usable / ICON_BOUNDS.width, usable / ICON_BOUNDS.height)
  const offsetX = inset - ICON_BOUNDS.minX * scale
  const offsetY = inset - ICON_BOUNDS.minY * scale
  return {
    scale,
    map([x, y]) {
      return [offsetX + x * scale, offsetY + y * scale]
    },
  }
}

function renderIconCoverage(x, y, size) {
  const { scale, map } = createTransform(size)
  const strokeWidth = 2.5 * scale
  let coverage = 0

  for (let i = 0; i < MAIN_SPARKLE.length; i += 1) {
    const [ax, ay] = map(MAIN_SPARKLE[i])
    const [bx, by] = map(MAIN_SPARKLE[(i + 1) % MAIN_SPARKLE.length])
    coverage = Math.max(
      coverage,
      strokeCoverage(distanceToSegment(x, y, ax, ay, bx, by), strokeWidth)
    )
  }

  for (const [[ax0, ay0], [bx0, by0]] of PLUS_SEGMENTS) {
    const [ax, ay] = map([ax0, ay0])
    const [bx, by] = map([bx0, by0])
    coverage = Math.max(
      coverage,
      strokeCoverage(distanceToSegment(x, y, ax, ay, bx, by), strokeWidth)
    )
  }

  const [cx, cy] = map([DOT_CIRCLE.cx, DOT_CIRCLE.cy])
  const radius = DOT_CIRCLE.r * scale
  const ringDistance = Math.abs(Math.hypot(x - cx, y - cy) - radius)
  coverage = Math.max(coverage, strokeCoverage(ringDistance, strokeWidth))

  return coverage
}

function renderIcon(size) {
  const rgba = Buffer.alloc(size * size * 4)
  const left = 0
  const top = 0
  const right = size
  const bottom = size
  const radius = size * BG_RADIUS_RATIO
  const samples = 4
  const totalSamples = samples * samples

  for (let y = 0; y < size; y += 1) {
    for (let x = 0; x < size; x += 1) {
      let accumR = 0
      let accumG = 0
      let accumB = 0
      let accumA = 0

      for (let sy = 0; sy < samples; sy += 1) {
        for (let sx = 0; sx < samples; sx += 1) {
          const px = x + (sx + 0.5) / samples
          const py = y + (sy + 0.5) / samples
          const bgAlpha = roundedRectCoverage(px, py, left, top, right, bottom, radius)
          const gradientT = clamp((px + py) / (size * 2), 0, 1)
          let r = lerp(GRADIENT_START[0], GRADIENT_END[0], gradientT)
          let g = lerp(GRADIENT_START[1], GRADIENT_END[1], gradientT)
          let b = lerp(GRADIENT_START[2], GRADIENT_END[2], gradientT)
          const iconAlpha = renderIconCoverage(px, py, size) * bgAlpha

          r = lerp(r, WHITE[0], iconAlpha)
          g = lerp(g, WHITE[1], iconAlpha)
          b = lerp(b, WHITE[2], iconAlpha)

          accumR += r * bgAlpha
          accumG += g * bgAlpha
          accumB += b * bgAlpha
          accumA += bgAlpha * 255
        }
      }

      const idx = (y * size + x) * 4
      rgba[idx] = Math.round(accumR / totalSamples)
      rgba[idx + 1] = Math.round(accumG / totalSamples)
      rgba[idx + 2] = Math.round(accumB / totalSamples)
      rgba[idx + 3] = Math.round(accumA / totalSamples)
    }
  }

  return rgba
}

function crc32(buffer) {
  let crc = 0xffffffff
  for (let i = 0; i < buffer.length; i += 1) {
    crc = crcTable[(crc ^ buffer[i]) & 0xff] ^ (crc >>> 8)
  }
  return (crc ^ 0xffffffff) >>> 0
}

function pngChunk(type, data) {
  const typeBuffer = Buffer.from(type, 'ascii')
  const length = Buffer.alloc(4)
  length.writeUInt32BE(data.length, 0)
  const crc = Buffer.alloc(4)
  crc.writeUInt32BE(crc32(Buffer.concat([typeBuffer, data])), 0)
  return Buffer.concat([length, typeBuffer, data, crc])
}

function encodePNG(size, rgba) {
  const signature = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a])
  const ihdr = Buffer.alloc(13)
  ihdr.writeUInt32BE(size, 0)
  ihdr.writeUInt32BE(size, 4)
  ihdr[8] = 8
  ihdr[9] = 6
  ihdr[10] = 0
  ihdr[11] = 0
  ihdr[12] = 0

  const stride = size * 4
  const raw = Buffer.alloc((stride + 1) * size)
  for (let y = 0; y < size; y += 1) {
    const rowOffset = y * (stride + 1)
    raw[rowOffset] = 0
    rgba.copy(raw, rowOffset + 1, y * stride, (y + 1) * stride)
  }

  const compressed = deflateSync(raw, { level: 9 })
  return Buffer.concat([
    signature,
    pngChunk('IHDR', ihdr),
    pngChunk('IDAT', compressed),
    pngChunk('IEND', Buffer.alloc(0)),
  ])
}

function encodeICO(images) {
  const header = Buffer.alloc(6)
  header.writeUInt16LE(0, 0)
  header.writeUInt16LE(1, 2)
  header.writeUInt16LE(images.length, 4)

  const directory = Buffer.alloc(images.length * 16)
  let offset = header.length + directory.length

  images.forEach(({ size, png }, index) => {
    const base = index * 16
    directory[base] = size === 256 ? 0 : size
    directory[base + 1] = size === 256 ? 0 : size
    directory[base + 2] = 0
    directory[base + 3] = 0
    directory.writeUInt16LE(1, base + 4)
    directory.writeUInt16LE(32, base + 6)
    directory.writeUInt32LE(png.length, base + 8)
    directory.writeUInt32LE(offset, base + 12)
    offset += png.length
  })

  return Buffer.concat([header, directory, ...images.map(({ png }) => png)])
}

function main() {
  mkdirSync(outDir, { recursive: true })

  const images = SIZES.map((size) => {
    const rgba = renderIcon(size)
    const png = encodePNG(size, rgba)
    writeFileSync(path.join(outDir, `icon-${size}.png`), png)
    return { size, png }
  })

  writeFileSync(path.join(outDir, 'icon.png'), images[images.length - 1].png)
  writeFileSync(path.join(outDir, 'icon.ico'), encodeICO(images))

  const rendered = images.map(({ size }) => `icon-${size}.png`).join(', ')
  console.log(`Generated ${rendered}, icon.png, and icon.ico in ${outDir}`)
}

main()
