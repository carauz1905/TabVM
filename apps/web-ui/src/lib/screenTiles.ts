// paintTiles decodes one binary screen-stream frame and draws its JPEG tiles
// onto the canvas context. Shared by the full console and the live preview.
//
// Wire format (big-endian): uint16 tileCount, then per tile
// uint16 x, uint16 y, uint16 w, uint16 h, uint32 jpegLen, jpegLen bytes.
export async function paintTiles(ctx: CanvasRenderingContext2D, data: ArrayBuffer): Promise<void> {
  const view = new DataView(data);
  const count = view.getUint16(0, false);
  let off = 2;
  const jobs: Promise<void>[] = [];
  for (let i = 0; i < count; i++) {
    const x = view.getUint16(off, false);
    const y = view.getUint16(off + 2, false);
    const w = view.getUint16(off + 4, false);
    const h = view.getUint16(off + 6, false);
    const len = view.getUint32(off + 8, false);
    const jpegStart = off + 12;
    const jpegBytes = new Uint8Array(data, jpegStart, len);
    off = jpegStart + len;
    const blob = new Blob([jpegBytes], { type: 'image/jpeg' });
    jobs.push(
      createImageBitmap(blob).then((bmp) => {
        ctx.drawImage(bmp, x, y, w, h);
        bmp.close();
      }),
    );
  }
  await Promise.all(jobs);
}
