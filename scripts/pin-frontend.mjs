import { readFileSync, writeFileSync } from "node:fs";
const packagePath = new URL("../frontend/package.json", import.meta.url);
const lockPath = new URL("../frontend/package-lock.json", import.meta.url);
const manifest = JSON.parse(readFileSync(packagePath, "utf8"));
const lock = JSON.parse(readFileSync(lockPath, "utf8"));
for (const section of ["dependencies", "devDependencies"]) {
  for (const name of Object.keys(manifest[section] || {})) {
    const version = lock.packages["node_modules/" + name]?.version;
    if (!version) throw new Error("No locked version for " + name);
    manifest[section][name] = version;
  }
  lock.packages[""][section] = manifest[section];
}
writeFileSync(packagePath, JSON.stringify(manifest, null, 2) + "\n");
writeFileSync(lockPath, JSON.stringify(lock, null, 2) + "\n");
