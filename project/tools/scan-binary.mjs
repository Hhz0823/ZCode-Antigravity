import fs from "node:fs";

const filePath = process.argv[2];
const needle = process.argv[3];
const radius = Number(process.argv[4] ?? 1500);
if (!filePath || !needle) {
  throw new Error("usage: node scan-binary.mjs <file> <needle> [radius]");
}
const source = fs.readFileSync(filePath);
const target = Buffer.from(needle);
let offset = 0;
let count = 0;
while ((offset = source.indexOf(target, offset)) >= 0 && count < 30) {
  const start = Math.max(0, offset - radius);
  const end = Math.min(source.length, offset + target.length + radius);
  const printable = source
    .subarray(start, end)
    .toString("latin1")
    .replace(/[^\x20-\x7e]+/g, "\n")
    .replace(/\n{2,}/g, "\n");
  console.log(`=== offset ${offset} ===\n${printable}`);
  offset += target.length;
  count += 1;
}
