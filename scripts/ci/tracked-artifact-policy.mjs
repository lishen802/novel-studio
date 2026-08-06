import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { basename, extname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const forbiddenDatabaseSuffixes = [
  ".db",
  ".db-shm",
  ".db-wal",
  ".sqlite",
  ".sqlite3",
];

const forbiddenSigningSuffixes = [
  ".key",
  ".p12",
  ".pem",
  ".pfx",
];

const scannedTextExtensions = new Set([
  ".go",
  ".js",
  ".json",
  ".md",
  ".mjs",
  ".sql",
  ".toml",
  ".ts",
  ".tsx",
  ".txt",
  ".yaml",
  ".yml",
]);

export function pathViolation(filePath) {
  const normalized = filePath.replaceAll("\\", "/");
  const lower = normalized.toLowerCase();
  const segments = lower.split("/");

  if (segments.some((segment) => segment.endsWith(".zjproject"))) {
    return "tracked project package";
  }
  if (forbiddenDatabaseSuffixes.some((suffix) => lower.endsWith(suffix))) {
    return "tracked project database";
  }
  if (forbiddenSigningSuffixes.some((suffix) => lower.endsWith(suffix))) {
    return "tracked private signing material";
  }
  return "";
}

export function contentViolation(content) {
  if (/-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----/.test(content)) {
    return "tracked private key";
  }
  if (/\bsk-proj-[A-Za-z0-9_-]{20,}\b/.test(content)) {
    return "tracked OpenAI-shaped credential";
  }
  if (/\bsk-ant-[A-Za-z0-9_-]{20,}\b/.test(content)) {
    return "tracked Anthropic-shaped credential";
  }
  return "";
}

function shouldScanText(filePath) {
  return (
    scannedTextExtensions.has(extname(filePath).toLowerCase()) ||
    basename(filePath) === ".env.example"
  );
}

export function inspectRepository(cwd) {
  const tracked = execFileSync("git", ["ls-files", "-z"], {
    cwd,
    encoding: "utf8",
  })
    .split("\0")
    .filter(Boolean);

  const violations = [];
  for (const filePath of tracked) {
    const pathReason = pathViolation(filePath);
    if (pathReason) {
      violations.push(`${filePath}: ${pathReason}`);
    }

    if (!shouldScanText(filePath)) {
      continue;
    }
    const contentReason = contentViolation(
      readFileSync(resolve(cwd, filePath), "utf8"),
    );
    if (contentReason) {
      violations.push(`${filePath}: ${contentReason}`);
    }
  }
  return { trackedCount: tracked.length, violations };
}

function run() {
  const { trackedCount, violations } = inspectRepository(process.cwd());
  if (violations.length > 0) {
    console.error("Tracked artifact policy failed:");
    for (const violation of violations) {
      console.error(`- ${violation}`);
    }
    process.exitCode = 1;
    return;
  }
  console.log(`Tracked artifact policy passed (${trackedCount} files checked).`);
}

const currentFile = fileURLToPath(import.meta.url);
if (process.argv[1] && resolve(process.argv[1]) === currentFile) {
  run();
}
