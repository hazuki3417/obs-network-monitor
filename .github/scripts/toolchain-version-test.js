const assert = require('node:assert/strict');
const fs = require('node:fs');

const goMod = fs.readFileSync('go.mod', 'utf8');
const goDirective = goMod.match(/^go\s+(\d+\.\d+)(?:\.\d+)?\s*$/m);
assert.ok(goDirective, 'go.mod must contain a valid go directive');

const goSeries = goDirective[1];
const devboxPackage = `go@${goSeries}`;
const devbox = JSON.parse(fs.readFileSync('devbox.json', 'utf8'));
const goPackages = devbox.packages.filter((name) => name.startsWith('go@'));
assert.deepEqual(
  goPackages,
  [devboxPackage],
  `devbox.json must contain exactly ${devboxPackage}`,
);

const lockfile = JSON.parse(fs.readFileSync('devbox.lock', 'utf8'));
const lockedGo = lockfile.packages[devboxPackage];
assert.ok(lockedGo, `devbox.lock must contain ${devboxPackage}`);
assert.match(
  lockedGo.version,
  new RegExp(`^${goSeries.replace('.', '\\.')}\\.`),
  `devbox.lock must resolve ${devboxPackage} to the same Go series`,
);

for (const path of ['README.md', 'CONTRIBUTING.md']) {
  const document = fs.readFileSync(path, 'utf8');
  assert.match(
    document,
    new RegExp(`Go ${goSeries.replace('.', '\\.')}以上`),
    `${path} must document Go ${goSeries} or newer`,
  );
}

console.log(`Toolchain versions are aligned on Go ${goSeries}.`);
