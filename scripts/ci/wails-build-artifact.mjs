import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import {
  lstatSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { dirname, join, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

const PRODUCT = "NovelStudio";
const MACH_O_MAGICS = new Set([
  "cafebabe",
  "bebafeca",
  "cefaedfe",
  "cffaedfe",
  "feedface",
  "feedfacf",
]);

const platformSpecs = {
  darwin: {
    artifactName: "NovelStudio.app",
    entrypoint: ["Contents", "MacOS", "NovelStudio"],
    requiredFiles: [
      ["Contents", "Info.plist"],
      ["Contents", "MacOS", "NovelStudio"],
    ],
    kind: "app_bundle",
  },
  windows: {
    artifactName: "NovelStudio.exe",
    entrypoint: [],
    requiredFiles: [[]],
    kind: "pe_executable",
  },
};

function slashPath(value) {
  return value.split(sep).join("/");
}

function requireString(value, name) {
  if (typeof value !== "string" || value.trim() === "") {
    throw new Error(`missing build metadata: ${name}`);
  }
  return value.trim();
}

function digest(filePath) {
  return createHash("sha256").update(readFileSync(filePath)).digest("hex");
}

function directoryMetrics(root) {
  let fileCount = 0;
  let sizeBytes = 0;

  function visit(current) {
    for (const entry of readdirSync(current, { withFileTypes: true })) {
      const fullPath = join(current, entry.name);
      if (entry.isDirectory()) {
        visit(fullPath);
        continue;
      }
      if (entry.isSymbolicLink()) {
        lstatSync(fullPath);
        continue;
      }
      if (entry.isFile()) {
        fileCount += 1;
        sizeBytes += statSync(fullPath).size;
      }
    }
  }

  visit(root);
  return { fileCount, sizeBytes };
}

function assertExecutableFormat(platform, entrypointPath) {
  const bytes = readFileSync(entrypointPath);
  if (platform === "windows") {
    if (bytes.length < 2 || bytes[0] !== 0x4d || bytes[1] !== 0x5a) {
      throw new Error("Windows build entrypoint is not a PE/MZ executable");
    }
    return;
  }

  const magic = bytes.subarray(0, 4).toString("hex");
  if (!MACH_O_MAGICS.has(magic)) {
    throw new Error("macOS build entrypoint is not a Mach-O executable");
  }
}

function allowlistedMetadata(metadata) {
  return {
    source: {
      commit: requireString(metadata.commit, "commit"),
    },
    ci: {
      run_id: requireString(metadata.runId, "runId"),
      run_attempt: requireString(metadata.runAttempt, "runAttempt"),
    },
    runner: {
      label: requireString(metadata.runnerLabel, "runnerLabel"),
      os: requireString(metadata.runnerOS, "runnerOS"),
      arch: requireString(metadata.runnerArch, "runnerArch"),
    },
    toolchain: {
      go: requireString(metadata.goVersion, "goVersion"),
      node: requireString(metadata.nodeVersion, "nodeVersion"),
      wails: requireString(metadata.wailsVersion, "wailsVersion"),
    },
  };
}

export function inspectWailsBuild({ platform, buildDir, metadata }) {
  const spec = platformSpecs[platform];
  if (!spec) {
    throw new Error(`unsupported Wails build platform: ${platform}`);
  }

  const absoluteBuildDir = resolve(buildDir);
  const artifactPath = join(absoluteBuildDir, spec.artifactName);
  let artifactStat;
  try {
    artifactStat = statSync(artifactPath);
  } catch {
    const label = platform === "darwin" ? "macOS app bundle" : "Windows executable";
    throw new Error(`missing ${label}: ${spec.artifactName}`);
  }

  if (platform === "darwin" && !artifactStat.isDirectory()) {
    throw new Error("macOS Wails artifact must be an app bundle directory");
  }
  if (platform === "windows" && !artifactStat.isFile()) {
    throw new Error("Windows Wails artifact must be an executable file");
  }

  for (const parts of spec.requiredFiles) {
    const requiredPath = join(artifactPath, ...parts);
    let requiredStat;
    try {
      requiredStat = statSync(requiredPath);
    } catch {
      throw new Error(
        `missing required Wails output: ${slashPath(relative(absoluteBuildDir, requiredPath))}`,
      );
    }
    if (!requiredStat.isFile() || requiredStat.size === 0) {
      throw new Error(
        `empty required Wails output: ${slashPath(relative(absoluteBuildDir, requiredPath))}`,
      );
    }
  }

  const entrypointPath =
    platform === "darwin"
      ? join(artifactPath, ...spec.entrypoint)
      : artifactPath;
  assertExecutableFormat(platform, entrypointPath);

  const metrics =
    platform === "darwin"
      ? directoryMetrics(artifactPath)
      : { fileCount: 1, sizeBytes: artifactStat.size };
  const allowlisted = allowlistedMetadata(metadata);

  return {
    schema_version: 1,
    product: PRODUCT,
    platform,
    artifact: {
      kind: spec.kind,
      path: slashPath(relative(absoluteBuildDir, artifactPath)),
      entrypoint: slashPath(relative(absoluteBuildDir, entrypointPath)),
      entrypoint_size_bytes: statSync(entrypointPath).size,
      entrypoint_sha256: digest(entrypointPath),
      file_count: metrics.fileCount,
      size_bytes: metrics.sizeBytes,
    },
    ...allowlisted,
  };
}

export function writeBuildManifest({
  platform,
  buildDir,
  output,
  metadata,
}) {
  const manifest = inspectWailsBuild({ platform, buildDir, metadata });
  mkdirSync(dirname(resolve(output)), { recursive: true });
  writeFileSync(resolve(output), `${JSON.stringify(manifest, null, 2)}\n`);
  return manifest;
}

function argument(name) {
  const index = process.argv.indexOf(`--${name}`);
  if (index === -1 || index + 1 >= process.argv.length) {
    throw new Error(`missing required argument: --${name}`);
  }
  return process.argv[index + 1];
}

function commandVersion(command, args) {
  return execFileSync(command, args, { encoding: "utf8" }).trim();
}

function metadataFromEnvironment() {
  return {
    commit: process.env.GITHUB_SHA || "local",
    runId: process.env.GITHUB_RUN_ID || "local",
    runAttempt: process.env.GITHUB_RUN_ATTEMPT || "1",
    runnerLabel: process.env.BUILD_RUNNER_LABEL || "local",
    runnerOS: process.env.RUNNER_OS || process.platform,
    runnerArch: process.env.RUNNER_ARCH || process.arch,
    goVersion: commandVersion("go", ["version"]),
    nodeVersion: process.version,
    wailsVersion: commandVersion("wails", ["version"]),
  };
}

function run() {
  const manifest = writeBuildManifest({
    platform: argument("platform"),
    buildDir: argument("build-dir"),
    output: argument("output"),
    metadata: metadataFromEnvironment(),
  });
  console.log(
    `Validated ${manifest.platform} ${manifest.artifact.path} ` +
      `(${manifest.artifact.size_bytes} bytes, ` +
      `${manifest.artifact.entrypoint_sha256})`,
  );
}

const currentFile = fileURLToPath(import.meta.url);
if (process.argv[1] && resolve(process.argv[1]) === currentFile) {
  try {
    run();
  } catch (error) {
    console.error(`Wails build artifact validation failed: ${error.message}`);
    process.exitCode = 1;
  }
}
