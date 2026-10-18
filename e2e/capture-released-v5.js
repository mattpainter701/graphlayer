// Captures synthetic workspace artifacts using the actual released v5 Store.
// Run only where normal Go test execution is permitted; no server is launched.
const { execFileSync } = require('node:child_process');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');

const release = '8086a83d0597543193400dee9f35f31c5ef5c126';
async function main() {
  if (process.argv.length !== 3) throw new Error('Provide a new empty output directory.');
  const output = path.resolve(process.argv[2]);
  await fs.mkdir(output, { recursive: true });
  if ((await fs.readdir(output)).length) throw new Error('Refusing to overwrite an existing output directory.');
  const root = path.resolve(__dirname, '..');
  const temporary = await fs.mkdtemp(path.join(os.tmpdir(), 'graphlayer-v5-capture-'));
  const archive = path.join(temporary, 'released.tar');
  try { execFileSync('git', ['cat-file', '-e', `${release}^{commit}`], { cwd: root, stdio: 'pipe' }); }
  catch { execFileSync('git', ['fetch', '--no-tags', '--depth=1', 'origin', release], { cwd: root, stdio: 'inherit' }); }
  execFileSync('git', ['archive', '--format=tar', '-o', archive, release, 'platform'], { cwd: root, env: { ...process.env, GIT_LFS_SKIP_SMUDGE: '1' } });
  execFileSync('tar', ['-xf', archive, '-C', temporary]);
  const test = `package investigation
import ("os"; "path/filepath"; "testing")
func TestCaptureReleasedV5Workspace(t *testing.T) {
 dir := os.Getenv("GRAPHLAYER_RELEASE_CAPTURE_DIR")
 if dir == "" { t.Fatal("output directory is required") }
 store, err := NewStore(dir); if err != nil { t.Fatal(err) }
 defer store.Close()
 sample := vendorNetworkSample()
 req := NetworkRequest{Source:sample.Source, Destination:sample.Destination, Links:sample.Links, Flow:sample.Flow}
 for _, item := range sample.Configs {
  snapshot, _, err := store.ImportVendor(item.Name, []byte(item.Config), item.Vendor)
  if err != nil { t.Fatal(err) }
  req.Nodes = append(req.Nodes, NetworkNodeRef{NodeID:item.NodeID, SnapshotID:snapshot.ID})
 }
 result, err := store.EvaluateNetwork(req)
 if err != nil || result.Result.Disposition != "modeled_round_trip" { t.Fatalf("released workflow failed: %v %#v", err, result) }
 if err := store.Close(); err != nil { t.Fatal(err) }
 if err := os.Remove(filepath.Join(dir,".workspace.lock")); err != nil { t.Fatal(err) }
}
`;
  await fs.writeFile(path.join(temporary, 'platform', 'investigation', 'release_capture_test.go'), test);
  execFileSync('go', ['test', './investigation', '-run', '^TestCaptureReleasedV5Workspace$', '-count=1'], {
    cwd: path.join(temporary, 'platform'), stdio: 'inherit',
    env: { ...process.env, GRAPHLAYER_RELEASE_CAPTURE_DIR: output }, windowsHide: true,
  });
  process.stdout.write(`Captured actual released ${release} workspace in ${output}\n`);
}
main().catch((error) => { process.stderr.write(`${error.stack || error.message}\n`); process.exitCode = 1; });
