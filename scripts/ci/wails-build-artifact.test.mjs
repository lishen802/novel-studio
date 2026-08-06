import assert from "node:assert/strict";
import {
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import {
  inspectWailsBuild,
  writeBuildManifest,
} from "./wails-build-artifact.mjs";

const roots = [];

test.afterEach(() => {
  for (const root of roots.splice(0)) {
    rmSync(root, { force: true, recursive: true });
  }
});

function temporaryBuildDir() {
  const root = mkdtempSync(join(tmpdir(), "novel-studio-wails-"));
  roots.push(root);
  return root;
}

function metadata(overrides = {}) {
  return {
    commit: "0123456789abcdef",
    runId: "101",
    runAttempt: "2",
    runnerLabel: "fixture-runner",
    runnerOS: "fixture-os",
    runnerArch: "fixture-arch",
    goVersion: "go version go1.25.12 fixture",
    nodeVersion: "v22.22.0",
    wailsVersion: "v2.13.0",
    ...overrides,
  };
}

test("inspects a macOS app bundle without persisting absolute paths", () => {
  const buildDir = temporaryBuildDir();
  const app = join(buildDir, "NovelStudio.app", "Contents");
  mkdirSync(join(app, "MacOS"), { recursive: true });
  writeFileSync(join(app, "Info.plist"), "<plist><dict/></plist>");
  writeFileSync(
    join(app, "MacOS", "NovelStudio"),
    Buffer.from([0xcf, 0xfa, 0xed, 0xfe, 0x00]),
  );

  const manifest = inspectWailsBuild({
    platform: "darwin",
    buildDir,
    metadata: metadata({
      runnerLabel: "macos-15",
      runnerOS: "macOS",
      runnerArch: "ARM64",
    }),
  });

  assert.equal(manifest.platform, "darwin");
  assert.equal(manifest.artifact.kind, "app_bundle");
  assert.equal(manifest.artifact.path, "NovelStudio.app");
  assert.equal(
    manifest.artifact.entrypoint,
    "NovelStudio.app/Contents/MacOS/NovelStudio",
  );
  assert.equal(manifest.runner.label, "macos-15");
  assert.match(manifest.artifact.entrypoint_sha256, /^[a-f0-9]{64}$/);
  assert.doesNotMatch(JSON.stringify(manifest), new RegExp(buildDir));
});

test("inspects a Windows PE executable and records only allowlisted metadata", () => {
  const buildDir = temporaryBuildDir();
  writeFileSync(
    join(buildDir, "NovelStudio.exe"),
    Buffer.from([0x4d, 0x5a, 0x00, 0x00]),
  );

  const manifest = inspectWailsBuild({
    platform: "windows",
    buildDir,
    metadata: metadata({
      runnerLabel: "windows-2025",
      runnerOS: "Windows",
      runnerArch: "X64",
      ignoredSecret: "must-not-be-persisted",
    }),
  });

  assert.equal(manifest.artifact.kind, "pe_executable");
  assert.equal(manifest.artifact.path, "NovelStudio.exe");
  assert.equal(manifest.artifact.entrypoint, "NovelStudio.exe");
  assert.equal(manifest.runner.label, "windows-2025");
  assert.doesNotMatch(JSON.stringify(manifest), /must-not-be-persisted/);
});

test("rejects missing or wrong-format Wails output", () => {
  const missingDir = temporaryBuildDir();
  assert.throws(
    () =>
      inspectWailsBuild({
        platform: "windows",
        buildDir: missingDir,
        metadata: metadata(),
      }),
    /missing Windows executable/i,
  );

  const invalidDir = temporaryBuildDir();
  writeFileSync(join(invalidDir, "NovelStudio.exe"), "not-a-pe-file");
  assert.throws(
    () =>
      inspectWailsBuild({
        platform: "windows",
        buildDir: invalidDir,
        metadata: metadata(),
      }),
    /PE\/MZ/i,
  );
});

test("writes a stable JSON manifest next to the build output", () => {
  const buildDir = temporaryBuildDir();
  writeFileSync(
    join(buildDir, "NovelStudio.exe"),
    Buffer.from([0x4d, 0x5a, 0x00, 0x00]),
  );
  const output = join(buildDir, "build-manifest.json");

  const written = writeBuildManifest({
    platform: "windows",
    buildDir,
    output,
    metadata: metadata(),
  });
  const parsed = JSON.parse(readFileSync(output, "utf8"));

  assert.deepEqual(parsed, written);
  assert.equal(parsed.schema_version, 1);
  assert.equal(parsed.product, "NovelStudio");
  assert.equal(readFileSync(output, "utf8").endsWith("\n"), true);
});
