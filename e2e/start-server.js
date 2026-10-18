const { spawn, spawnSync } = require('node:child_process');
const fs = require('node:fs/promises');
const path = require('node:path');

async function main() {
  const root = path.resolve(__dirname, '..');
  const dataDir = path.join(root, '.e2e-runtime');
  const binaryDir = path.join(root, 'platform', '.e2e-runtime');
  for (const target of [dataDir, binaryDir]) {
    const relative = path.relative(root, target);
    if (!relative || relative.startsWith('..') || path.isAbsolute(relative) || path.resolve(root, relative) !== target) {
      throw new Error('Refusing to clear an E2E path outside the repository runtime folders.');
    }
  }
  await fs.rm(dataDir, { recursive: true, force: true });
  await fs.mkdir(dataDir, { recursive: true });
  await fs.rm(binaryDir, { recursive: true, force: true });
  await fs.mkdir(binaryDir, { recursive: true });

  const binary = path.join(binaryDir, process.platform === 'win32' ? 'graphlayer-investigate.exe' : 'graphlayer-investigate');
  const build = spawnSync('go', ['build', '-o', binary, './cmd/graphlayer-investigate'], {
    cwd: path.join(root, 'platform'), stdio: 'inherit', windowsHide: true,
  });
  if (build.error || build.status !== 0) {
    throw build.error || new Error(`go build exited with status ${build.status}`);
  }

  const child = spawn(binary, [
    '--listen', '127.0.0.1:8088',
    '--data-dir', path.join(dataDir, 'workspace'),
  ], { cwd: root, stdio: 'inherit', windowsHide: true });

  const stop = () => child.kill('SIGTERM');
  process.on('SIGINT', stop);
  process.on('SIGTERM', stop);
  child.on('error', (error) => {
    process.stderr.write(`Could not start graphlayer-investigate: ${error.message}\n`);
    process.exitCode = 1;
  });
  child.on('exit', (code, signal) => {
    process.exitCode = signal ? 1 : (code ?? 1);
  });
}

main().catch((error) => {
  process.stderr.write(`Could not prepare the E2E workspace: ${error.stack || error.message}\n`);
  process.exitCode = 1;
});
