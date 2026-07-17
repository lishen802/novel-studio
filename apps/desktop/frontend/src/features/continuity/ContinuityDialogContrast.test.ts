import { describe, expect, it } from "vitest";
import styles from "../../app/styles.css?raw";

function cssVariable(name: string) {
  const match = styles.match(new RegExp(`${name}:\\s*(#[0-9a-fA-F]{6})`));
  expect(
    match,
    `${name} should be declared as a six-digit hex color`,
  ).not.toBeNull();
  return match![1];
}

function relativeLuminance(hex: string) {
  const channels = hex
    .slice(1)
    .match(/.{2}/g)!
    .map((value) => Number.parseInt(value, 16) / 255)
    .map((value) =>
      value <= 0.04045
        ? value / 12.92
        : Math.pow((value + 0.055) / 1.055, 2.4),
    );
  return 0.2126 * channels[0] + 0.7152 * channels[1] + 0.0722 * channels[2];
}

function contrastRatio(foreground: string, background: string) {
  const lighter = Math.max(
    relativeLuminance(foreground),
    relativeLuminance(background),
  );
  const darker = Math.min(
    relativeLuminance(foreground),
    relativeLuminance(background),
  );
  return (lighter + 0.05) / (darker + 0.05);
}

describe("continuity dialog contrast", () => {
  it.each([
    [
      "eyebrow",
      "--continuity-dialog-accent-text",
      "--continuity-dialog-surface",
    ],
    ["label", "--continuity-dialog-label", "--continuity-dialog-surface"],
    [
      "supporting text",
      "--continuity-dialog-muted-text",
      "--continuity-dialog-surface",
    ],
    [
      "field text",
      "--continuity-dialog-field-text",
      "--continuity-dialog-field-surface",
    ],
    [
      "placeholder",
      "--continuity-dialog-placeholder",
      "--continuity-dialog-field-surface",
    ],
    [
      "secondary action",
      "--continuity-dialog-secondary-text",
      "--continuity-dialog-secondary-surface",
    ],
    [
      "primary action",
      "--continuity-dialog-primary-text",
      "--continuity-dialog-primary-surface",
    ],
  ])(
    "keeps %s at WCAG AA normal-text contrast",
    (_, foreground, background) => {
      expect(
        contrastRatio(cssVariable(foreground), cssVariable(background)),
      ).toBeGreaterThanOrEqual(4.5);
    },
  );

  it("applies the accessible palette to labels, fields and dialog actions", () => {
    expect(styles).toMatch(
      /\.continuity-dialog label\s*\{[^}]*color:\s*var\(--continuity-dialog-label\)/s,
    );
    expect(styles).toMatch(
      /\.continuity-dialog input,[^}]*\.continuity-dialog textarea\s*\{[^}]*color:\s*var\(--continuity-dialog-field-text\)[^}]*background:\s*var\(--continuity-dialog-field-surface\)/s,
    );
    expect(styles).toMatch(
      /\.continuity-dialog \.secondary-action\s*\{[^}]*color:\s*var\(--continuity-dialog-secondary-text\)[^}]*background:\s*var\(--continuity-dialog-secondary-surface\)/s,
    );
  });
});
