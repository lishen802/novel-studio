import assert from "node:assert/strict";
import test from "node:test";

import {
  contentViolation,
  pathViolation,
} from "./tracked-artifact-policy.mjs";

test("rejects tracked project packages, databases, and private signing material", () => {
  assert.match(
    pathViolation("fixtures/demo.zjproject/project.db"),
    /project package/i,
  );
  assert.match(pathViolation("tmp/project.db"), /database/i);
  assert.match(pathViolation("tmp/project.db-wal"), /database/i);
  assert.match(pathViolation("signing/release.p12"), /private signing/i);
  assert.match(pathViolation("signing/notarization.key"), /private signing/i);
});

test("allows source, migration, documentation, and generated fixture specifications", () => {
  assert.equal(pathViolation("internal/persistence/sqlite/migrations/00014.sql"), "");
  assert.equal(pathViolation("fixtures/longform/novel_500ch/spec.json"), "");
  assert.equal(pathViolation("docs/testing/foundation-acceptance.md"), "");
});

test("rejects private keys and live-shaped provider credentials", () => {
  const privateKeyHeader = ["-----BEGIN", "PRIVATE KEY-----"].join(" ");
  const openAIKey = ["sk", "proj", "A".repeat(32)].join("-");
  const anthropicKey = ["sk", "ant", "B".repeat(32)].join("-");

  assert.match(contentViolation(privateKeyHeader), /private key/i);
  assert.match(contentViolation(openAIKey), /OpenAI/i);
  assert.match(contentViolation(anthropicKey), /Anthropic/i);
});

test("allows documentation labels and short fake secrets", () => {
  assert.equal(contentViolation("OPENAI_API_KEY"), "");
  assert.equal(contentViolation("sk-test"), "");
  assert.equal(contentViolation("fake-secret"), "");
});
