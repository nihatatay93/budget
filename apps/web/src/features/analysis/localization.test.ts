/// <reference types="vite/client" />
import { describe, expect, it } from "vitest";

import { t } from "../../lib/i18n";

/*
 * The analysis destination introduces a large amount of prose. A missing Turkish entry does
 * not fail a type check or a render test: t() falls back to the English key, so the page
 * silently turns bilingual. This walks the literals in the source instead.
 */

// Read through the bundler rather than the filesystem: the same resolution the app uses, and
// no Node types in a config that otherwise describes a browser build.
const sources = Object.values(import.meta.glob<string>(
  ["./AnalysisDashboard.tsx", "../../lib/analysis.ts", "../../components/Charts.tsx"],
  { query: "?raw", import: "default", eager: true },
));

/** Matches t("literal"…) and t(`literal`…); interpolated keys are not statically checkable. */
const TRANSLATION_CALL = /\bt\(\s*(?:"((?:[^"\\]|\\.)*)"|`([^`$\\]*)`)/g;

function translationKeys(): string[] {
  const keys = new Set<string>();
  for (const source of sources) {
    for (const match of source.matchAll(TRANSLATION_CALL)) {
      const key = (match[1] ?? match[2]).replace(/\\"/g, '"');
      if (key) keys.add(key);
    }
  }
  return [...keys];
}

/**
 * Turkish uses these unchanged: "Net" is the same word, and the payee line is punctuation
 * around two interpolations. Listing them keeps the check strict everywhere else.
 */
const identicalInTurkish = new Set(["Net", "{payee}: {amount}"]);

describe("analysis localization", () => {
  it("finds the translation calls it is meant to be checking", () => {
    // A regex that silently matches nothing would make every assertion below vacuous.
    expect(translationKeys().length).toBeGreaterThan(40);
  });

  it("translates every analysis string into Turkish", () => {
    const untranslated = translationKeys()
      .filter((key) => !identicalInTurkish.has(key))
      .filter((key) => t(key, "tr") === key);
    expect(untranslated, `missing Turkish translations:\n${untranslated.join("\n")}`)
      .toEqual([]);
  });

  it("keeps every interpolation placeholder in the Turkish translation", () => {
    const mismatched: string[] = [];
    for (const key of translationKeys()) {
      const placeholders = [...key.matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort();
      if (placeholders.length === 0) continue;
      const translated = t(key, "tr");
      const translatedPlaceholders = [...translated.matchAll(/\{(\w+)\}/g)]
        .map((match) => match[1]).sort();
      // A dropped placeholder loses a number; an added one renders a literal "{amount}".
      if (placeholders.join() !== translatedPlaceholders.join()) {
        mismatched.push(`${key} -> ${translated}`);
      }
    }
    expect(mismatched, `placeholder mismatch:\n${mismatched.join("\n")}`).toEqual([]);
  });
});
