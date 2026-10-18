const { spawn, spawnSync } = require('node:child_process');
const fs = require('node:fs/promises');
const path = require('node:path');

async function main() {
  const root = path.resolve(__dirname, '..');
  const args = process.argv.slice(2);
  const modes = {
    '--exploration': { runtime: '.e2e-exploration-runtime', listen: '127.0.0.1:8089' },
    '--workspace': { runtime: '.e2e-workspace-runtime', listen: '127.0.0.1:8090' },
  };
  if (args.length && (args.length !== 1 || !Object.hasOwn(modes, args[0]))) throw new Error('Unknown E2E server mode.');
  const { runtime, listen } = args.length ? modes[args[0]] : { runtime: '.e2e-runtime', listen: '127.0.0.1:8088' };
  const dataDir = path.join(root, runtime);
  const binaryDir = path.join(root, 'platform', runtime);
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

  const engine = process.env.GRAPHLAYER_ENGINE;
  if (!engine || !path.isAbsolute(engine)) throw new Error('Build the Rust engine and set its absolute GRAPHLAYER_ENGINE path before browser tests.');
  const packagedEngine = path.join(binaryDir, process.platform === 'win32' ? 'graphlayer-engine.exe' : 'graphlayer-engine');
  await fs.copyFile(engine, packagedEngine);
  if (process.platform !== 'win32') await fs.chmod(packagedEngine, 0o755);
  const childEnvironment = { ...process.env };
  delete childEnvironment.GRAPHLAYER_ENGINE;

  const child = spawn(binary, [
    '--listen', listen,
    '--data-dir', path.join(dataDir, 'workspace'),
  ], { cwd: root, env: childEnvironment, stdio: 'inherit', windowsHide: true });

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
