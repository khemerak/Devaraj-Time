// Rasterize the original clock brand mark without external assets or dependencies.
import { deflateSync } from "node:zlib";
import { writeFileSync } from "node:fs";
const crc = (b) => {
  let c = 0xffffffff;
  for (const v of b) {
    c ^= v;
    for (let n = 0; n < 8; n++) c = (c >>> 1) ^ (c & 1 ? 0xedb88320 : 0);
  }
  return (c ^ 0xffffffff) >>> 0;
};
function chunk(type, data) {
  const name = Buffer.from(type),
    b = Buffer.alloc(data.length + 12);
  b.writeUInt32BE(data.length);
  name.copy(b, 4);
  data.copy(b, 8);
  b.writeUInt32BE(crc(Buffer.concat([name, data])), b.length - 4);
  return b;
}
function lineDistance(x, y, ax, ay, bx, by) {
  const dx = bx - ax,
    dy = by - ay,
    t = Math.max(
      0,
      Math.min(1, ((x - ax) * dx + (y - ay) * dy) / (dx * dx + dy * dy)),
    );
  return Math.hypot(x - ax - t * dx, y - ay - t * dy);
}
for (const size of [192, 512]) {
  const pixels = Buffer.alloc((size * 4 + 1) * size);
  for (let y = 0; y < size; y++) {
    for (let x = 0; x < size; x++) {
      const sx = (x / size) * 512,
        sy = (y / size) * 512,
        white =
          Math.abs(Math.hypot(sx - 256, sy - 256) - 155) < 12.5 ||
          lineDistance(sx, sy, 256, 158, 256, 256) < 12.5 ||
          lineDistance(sx, sy, 256, 256, 331, 301) < 12.5;
      const o = y * (size * 4 + 1) + 1 + x * 4;
      pixels[o] = white ? 255 : 23;
      pixels[o + 1] = white ? 255 : 37;
      pixels[o + 2] = white ? 255 : 84;
      pixels[o + 3] = 255;
    }
  }
  const header = Buffer.alloc(13);
  header.writeUInt32BE(size);
  header.writeUInt32BE(size, 4);
  header[8] = 8;
  header[9] = 6;
  writeFileSync(
    new URL(`../frontend/public/icon-${size}.png`, import.meta.url),
    Buffer.concat([
      Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
      chunk("IHDR", header),
      chunk("IDAT", deflateSync(pixels)),
      chunk("IEND", Buffer.alloc(0)),
    ]),
  );
}
