#!/usr/bin/env node
// Capture the pre-Rust ASA path projection from the exact release source.
// Intended for hosted CI only; it runs the Go test binary from a git archive.

const { execFileSync } = require("node:child_process");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");

const releaseRevision = "d6d11db4ce7a411b013a6030af0e4d4a8d358028";

function parseArgs(argv) {
  const result = {};
  for (let i = 0; i < argv.length; i += 1) {
    if (argv[i] === "--output" && argv[i + 1]) result.output = argv[++i];
    else throw new Error(`Unknown or incomplete argument: ${argv[i]}`);
  }
  if (!result.output) throw new Error("Usage: capture-released-asa-path.js --output <artifact.json>");
  return result;
}

const args = parseArgs(process.argv.slice(2));
const repository = path.resolve(__dirname, "..");
const output = path.resolve(args.output);
if (fs.existsSync(output)) throw new Error(`Refusing to overwrite existing output: ${output}`);
try {
  execFileSync("git", ["cat-file", "-e", `${releaseRevision}^{commit}`], { cwd: repository, stdio: "ignore" });
} catch {
  execFileSync("git", ["fetch", "--depth=1", "origin", releaseRevision], {
    cwd: repository,
    stdio: "inherit",
    env: { ...process.env, GIT_LFS_SKIP_SMUDGE: "1" },
  });
}
const resolvedRevision = execFileSync("git", ["rev-parse", `${releaseRevision}^{commit}`], {
  cwd: repository,
  encoding: "utf8",
  env: { ...process.env, GIT_LFS_SKIP_SMUDGE: "1" },
}).trim();
if (resolvedRevision !== releaseRevision) {
  throw new Error(`Release revision mismatch: expected ${releaseRevision}`);
}

const temporaryDirectory = fs.mkdtempSync(path.join(os.tmpdir(), "asa-path-golden-"));
try {
  const archivePath = path.join(temporaryDirectory, "release.tar");
  const archiveFd = fs.openSync(archivePath, "w");
  try {
    execFileSync("git", ["archive", "--format=tar", releaseRevision, "platform"], {
      cwd: repository,
      stdio: ["ignore", archiveFd, "inherit"],
      env: { ...process.env, GIT_LFS_SKIP_SMUDGE: "1" },
    });
  } finally {
    fs.closeSync(archiveFd);
  }
  execFileSync("tar", ["-xf", archivePath, "-C", temporaryDirectory], { stdio: "inherit" });

  const releasePlatform = path.join(temporaryDirectory, "platform");
  const testPath = path.join(releasePlatform, "firewall", "asa_path_parser_test.go");
  const originalTests = fs.readFileSync(testPath, "utf8");
  const helperCall = "\taddASAPathInventory(c, parsed)\n\treturn c\n}";
  if (originalTests.split(helperCall).length !== 2) {
    throw new Error("Release ASA path test helper did not match the expected source");
  }
  fs.writeFileSync(testPath, originalTests.replace(helperCall, "\taddASAPathInventory(c, parsed)\n\trecordASAPathGolden(input, parsed, c)\n\treturn c\n}"));

  const goTag = String.fromCharCode(96);
  const captureTest = String.raw`package firewall

import (
    "encoding/json"
    "os"
    "testing"

    asa "github.com/mattpainter701/graphlayer/platform/vendors/cisco/asa"
)

type capturedASAPath struct {
    Input struct {
        ArtifactSHA256 string ${goTag}json:"artifact_sha256"${goTag}
        Lines []capturedLine ${goTag}json:"lines"${goTag}
        NetworkObjects map[string]capturedNetworkObject ${goTag}json:"network_objects"${goTag}
    } ${goTag}json:"input"${goTag}
    Output struct {
        Path *PathConfig ${goTag}json:"path"${goTag}
        Evidence []Evidence ${goTag}json:"evidence"${goTag}
        SourceLineUpdates []SourceLine ${goTag}json:"source_line_updates"${goTag}
    } ${goTag}json:"output"${goTag}
}

type capturedLine struct {
    Number int ${goTag}json:"number"${goTag}
    Text string ${goTag}json:"text"${goTag}
}

type capturedNetworkObject struct {
    Group bool ${goTag}json:"group"${goTag}
    Members []string ${goTag}json:"members"${goTag}
    UnsupportedLines []int ${goTag}json:"unsupported_lines"${goTag}
}

var asaPathGoldenCases []capturedASAPath

func recordASAPathGolden(input []byte, parsed *asa.ASAConfig, config *Config) {
    item := capturedASAPath{}
    item.Input.ArtifactSHA256 = parsed.ArtifactSHA256
    item.Input.Lines = make([]capturedLine, 0, len(parsed.Lines))
    for _, line := range parsed.Lines {
        item.Input.Lines = append(item.Input.Lines, capturedLine{Number: line.Number, Text: line.Text})
    }
    item.Input.NetworkObjects = make(map[string]capturedNetworkObject, len(parsed.Objects))
    for name, object := range parsed.Objects {
        item.Input.NetworkObjects[name] = capturedNetworkObject{
            Group: object.Group,
            Members: append([]string{}, object.Members...),
            UnsupportedLines: append([]int{}, object.UnsupportedLines...),
        }
    }
    item.Output.Path = config.Path
    item.Output.Evidence = append([]Evidence{}, config.Evidence...)
    item.Output.SourceLineUpdates = []SourceLine{}
    for _, line := range config.SourceLines {
        if line.Text != "[OMITTED NON-POLICY CONFIGURATION]" {
            item.Output.SourceLineUpdates = append(item.Output.SourceLineUpdates, line)
        }
    }
    asaPathGoldenCases = append(asaPathGoldenCases, item)
}

func TestZZZWriteASAPathGolden(t *testing.T) {
    // Capture sanitized embedded samples directly as well as every test case
    // that already goes through parsePathInventoryForTest.
    parsePathInventoryForTest(t, sampleASA)
    parsePathInventoryForTest(t, sampleServicesASA)
    parsePathInventoryForTest(t, samplePathASA)
    for _, input := range asaPathGoldenSyntheticCorpus() {
        parsePathInventoryForTest(t, []byte(input))
    }
    if len(asaPathGoldenCases) == 0 {
        t.Fatal("no release path parser cases were captured")
    }
    data, err := json.MarshalIndent(asaPathGoldenCases, "", "  ")
    if err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(os.Getenv("ASA_PATH_GOLDEN_OUTPUT"), append(data, '\n'), 0600); err != nil {
        t.Fatal(err)
    }
}

func asaPathGoldenSyntheticCorpus() []string {
    const base = "interface Gi0/0\n nameif inside\n security-level 100\n ip address 10.0.0.1 255.255.255.0\n no shutdown\ninterface Gi0/1\n nameif outside\n security-level 0\n ip address 192.0.2.1 255.255.255.0\n no shutdown\n"
    inputs := []string{}
    interfaceChanges := []string{
        " description uplink", " speed 1000", " duplex full", " mtu 1500", " flowcontrol send on",
        " mac-address 0011.2233.4455", " media-type rj45", " bandwidth 100000", " delay 10", " monitor-interface svc-module",
        " no shutdown", " shutdown", " no nameif", " no ip address", " ip address dhcp", " ip address 192.0.2.8 255.0.255.0",
        " security-level 101", " security-level 10", " ipv6 address 2001:db8::1/64", " bridge-group 1", " pppoe client vpdn group ISP",
        " management-only", " standby 192.0.2.2",
    }
    for _, change := range interfaceChanges {
        inputs = append(inputs, base+"interface Gi0/2\n nameif branch\n ip address 198.51.100.1 255.255.255.0\n"+change+"\n")
    }
    inputs = append(inputs,
        base+"interface Gi0/0\n nameif duplicate\n",
        base+"interface Gi0/2\n nameif inside\n ip address 10.0.1.1 255.255.255.0\n",
        "interface\n nameif malformed\n ip address 10.0.0.1 255.255.255.0\n",
        "interface Gi0/0\n nameif outside\n ip address 10.0.0.1 255.255.255.0\n",
    )

    routeForms := []string{
        "route outside 0 0 192.0.2.254", "route outside 0.0.0.0 0.0.0.0 192.0.2.254 10",
        "route outside 203.0.113.0 255.255.255.0 192.0.2.254 255", "route outside 203.0.113.0 255.255.255.0 192.0.2.254 track 4",
        "route outside 203.0.113.0 255.255.255.0 192.0.2.254 tunneled", "route outside 203.0.113.0 255.255.255.0 192.0.2.254 5 name test",
        "route outside 203.0.113.0 255.255.255.0 192.0.2.254", "route outside 203.0.113.0 255.255.255.0 192.0.2.254",
        "route missing 203.0.113.0 255.255.255.0 192.0.2.254", "route Null0 203.0.113.0 255.255.255.0 192.0.2.254",
        "route outside 203.0.113.0 255.0.255.0 192.0.2.254", "route outside 203.0.113.0 255.255.255.0 2001:db8::1",
        "ipv6 route outside 2001:db8::/32 2001:db8::1", "router bgp 64512", "router ospf 10", "router eigrp 10",
        "policy-route inside route-map POLICY", "pbr route-map POLICY", "route-map POLICY permit 10", "firewall transparent",
        "crypto map VPN 10 match address VPN-ACL", "crypto dynamic-map VPN-DYNAMIC 10", "tunnel-group peer type ipsec-l2l",
        "ip verify reverse-path interface outside", "ip policy route-map POLICY", "mode multiple",
        "same-security-traffic permit inter-interface", "same-security-traffic permit intra-interface",
    }
    for _, form := range routeForms {
        inputs = append(inputs, base+form+"\n")
    }

    natForms := []string{
        "object network WEB\n host 10.0.0.10\n nat (inside,outside) static 198.51.100.10\n",
        "object network WEB\n host 10.0.0.10\n nat (inside,outside) dynamic interface\n",
        "object network WEB\n host 10.0.0.10\n nat (inside,outside) static 198.51.100.10 service tcp 443 8443\n",
        "object network WEB\n host 10.0.0.10\n nat (inside,outside) static 198.51.100.10\nobject network WEB\n host 10.0.0.11\n",
        "object network WEB\n host 10.0.0.10\n fqdn v4 web.invalid\nobject network WEB\n nat (inside,outside) static 198.51.100.10\n",
        "object network WEB\n subnet 10.0.0.0 255.255.255.0\nobject network WEB\n nat (inside,outside) static 198.51.100.10\n",
        "object network WEB\n subnet 10.0.0.0 255.255.255.0\nobject network WEB\n nat (inside,outside) static 198.51.100.0\n",
        "object network WEB\n host 10.0.0.10\n nat (inside,outside) static 198.51.100.10\n nat (inside,outside) static 198.51.100.11\n",
        "object-group network WEB\n network-object host 10.0.0.10\n",
        "static (inside,outside) 198.51.100.10 10.0.0.10 netmask 255.255.255.255",
        "global (outside) 1 198.51.100.10", "nat-control", "nat (inside,outside) source dynamic POOL",
        "object network WEB\n host 10.0.0.10\n nat (any,outside) static 198.51.100.10\n",
    }
    for _, form := range natForms {
        inputs = append(inputs, base+form+"\n")
    }
    return inputs
}
`;
  fs.writeFileSync(path.join(releasePlatform, "firewall", "asa_path_golden_capture_test.go"), captureTest);

  fs.mkdirSync(path.dirname(output), { recursive: true });
  execFileSync(process.env.GO || "go", [
    "test",
    "./firewall",
    "-run",
    "^(TestASAPathInventory.*|TestParseASAConfigIntegratesThePathSample|TestZZZWriteASAPathGolden)$",
    "-count=1",
  ], {
    cwd: releasePlatform,
    env: { ...process.env, ASA_PATH_GOLDEN_OUTPUT: output },
    stdio: "inherit",
  });
  process.stdout.write(`Captured ${JSON.parse(fs.readFileSync(output, "utf8")).length} release cases to ${output}\n`);
} finally {
  fs.rmSync(temporaryDirectory, { recursive: true, force: true });
}
