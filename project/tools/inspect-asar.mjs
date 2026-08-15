import fs from "node:fs";

const archivePath = process.argv[2];
if (!archivePath) {
  throw new Error("usage: node inspect-asar.mjs <archive> [pattern]");
}
const pattern = process.argv[3] ? new RegExp(process.argv[3], "i") : null;
const archive = fs.readFileSync(archivePath);
const headerSize = archive.readUInt32LE(4);
const jsonSize = archive.readUInt32LE(12);
const header = JSON.parse(archive.subarray(16, 16 + jsonSize).toString("utf8"));
const dataOffset = 8 + headerSize;

const files = [];
function walk(node, prefix = "") {
  for (const [name, entry] of Object.entries(node.files ?? {})) {
    const path = prefix ? `${prefix}/${name}` : name;
    if (entry.files) {
      walk(entry, path);
      continue;
    }
    files.push({ path, ...entry });
  }
}
walk(header);

const matches = [];
for (const file of files) {
  if (file.unpacked || !file.size || !file.offset) continue;
  if (!/\.(?:js|cjs|mjs|json|html|css|ts|map)$/i.test(file.path)) continue;
  const start = dataOffset + Number(file.offset);
  const body = archive.subarray(start, start + Number(file.size)).toString("utf8");
  if (!pattern || pattern.test(body) || pattern.test(file.path)) {
    matches.push({ path: file.path, size: Number(file.size), body: pattern ? body : undefined });
  }
}

if (!pattern) {
  console.log(JSON.stringify({ headerSize, jsonSize, dataOffset, count: files.length, files: matches }, null, 2));
} else {
  for (const match of matches) {
    console.log(`=== ${match.path} (${match.size}) ===`);
    const source = match.body;
    const index = source.search(pattern);
    console.log(source.slice(Math.max(0, index - 1000), Math.min(source.length, index + 3000)));
  }
}
