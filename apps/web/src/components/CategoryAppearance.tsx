import type { CSSProperties } from "react";

import { AppIcon, type AppIconName } from "./ExperiencePrimitives";
import { t } from "../lib/i18n";

export type CategoryIconType = "emoji" | "system";
export type CategoryColorKey =
  | "green" | "mint" | "blue" | "cyan" | "purple"
  | "pink" | "red" | "orange" | "amber" | "slate";

/**
 * The only web mapping from persisted semantic category icon keys to UI icons.
 * AppIcon's own drawing implementation deliberately stays separate from this contract.
 */
export const categorySystemIcons = [
  "home", "shopping-cart", "utensils", "car", "receipt", "shopping-bag", "heart",
  "gamepad", "repeat", "plane", "graduation-cap", "sparkles", "gift", "ellipsis",
  "wallet", "laptop", "trending-up", "building", "refund", "wallet-more",
] as const satisfies readonly AppIconName[];

export type CategorySystemIconKey = typeof categorySystemIcons[number];

const categoryIconMap: Record<CategorySystemIconKey, AppIconName> = {
  home: "home", "shopping-cart": "shopping-cart", utensils: "utensils", car: "car",
  receipt: "receipt", "shopping-bag": "shopping-bag", heart: "heart", gamepad: "gamepad",
  repeat: "repeat", plane: "plane", "graduation-cap": "graduation-cap", sparkles: "sparkles",
  gift: "gift", ellipsis: "ellipsis", wallet: "wallet", laptop: "laptop",
  "trending-up": "trending-up", building: "building", refund: "refund", "wallet-more": "wallet-more",
};

export const categoryColorKeys = [
  "green", "mint", "blue", "cyan", "purple", "pink", "red", "orange", "amber", "slate",
] as const satisfies readonly CategoryColorKey[];

const categoryColors: Record<CategoryColorKey, { accent: string; soft: string; ink: string }> = {
  green: { accent: "#287a54", soft: "#e2f0e7", ink: "#145238" },
  mint: { accent: "#418a79", soft: "#e2f1ec", ink: "#245f53" },
  blue: { accent: "#39759a", soft: "#e4eff5", ink: "#24536f" },
  cyan: { accent: "#3c8592", soft: "#e2f1f2", ink: "#225d67" },
  purple: { accent: "#70618f", soft: "#ede9f4", ink: "#4f416c" },
  pink: { accent: "#a16478", soft: "#f5e8ed", ink: "#743d52" },
  red: { accent: "#a34e48", soft: "#f5e8e5", ink: "#76332e" },
  orange: { accent: "#b86f3f", soft: "#f7eadf", ink: "#7c4422" },
  amber: { accent: "#a77b28", soft: "#f7efd9", ink: "#6d4c10" },
  slate: { accent: "#66776c", soft: "#e9eeea", ink: "#405047" },
};

/**
 * The accent for a category, for marks that need a literal color rather than a CSS variable.
 * SVG fills and strokes cannot resolve the custom property set on an ancestor element.
 */
export function categoryAccentColor(colorKey?: string): string {
  return (categoryColors[(colorKey ?? "slate") as CategoryColorKey] ?? categoryColors.slate).accent;
}

export function categoryColorStyle(colorKey?: string): CSSProperties {
  const color = categoryColors[(colorKey ?? "slate") as CategoryColorKey] ?? categoryColors.slate;
  return {
    "--category-accent": color.accent,
    "--category-soft": color.soft,
    "--category-ink": color.ink,
  } as CSSProperties;
}

export function categoryIconLabel(iconKey: string): string {
  return t(`category.icon.${iconKey}`);
}

export function isSingleGrapheme(value: string): boolean {
  const normalized = value.trim();
  if (!normalized) return false;
  if (typeof Intl.Segmenter === "function") {
    return [...new Intl.Segmenter(undefined, { granularity: "grapheme" }).segment(normalized)].length === 1;
  }
  return Array.from(normalized).length === 1;
}

export function CategoryAppearance({
  colorKey,
  iconType,
  iconValue,
  label,
  size = 18,
}: {
  colorKey?: string;
  iconType?: CategoryIconType;
  iconValue?: string;
  label?: string;
  size?: number;
}) {
  // Compatibility responses may only contain the deprecated icon field. New API responses
  // always provide iconType, so an unknown system value still falls back to ellipsis.
  const effectiveIconType = iconType ?? (iconValue && !(iconValue in categoryIconMap) ? "emoji" : "system");
  const systemIcon = effectiveIconType !== "emoji" ? categoryIconMap[iconValue as CategorySystemIconKey] : undefined;
  return (
    <span
      aria-label={label}
      aria-hidden={label ? undefined : true}
      className="category-appearance-icon"
      role={label ? "img" : undefined}
      style={categoryColorStyle(colorKey)}
      title={label}
    >
      {effectiveIconType === "emoji" && iconValue ? <span className="category-emoji">{iconValue}</span> : null}
      {effectiveIconType !== "emoji" ? <AppIcon name={systemIcon ?? "ellipsis"} size={size} /> : null}
    </span>
  );
}

export function CategoryLabel({
  colorKey,
  iconType,
  iconValue,
  name,
  size,
}: {
  colorKey?: string;
  iconType?: CategoryIconType;
  iconValue?: string;
  name: string;
  size?: number;
}) {
  return <span className="category-label"><CategoryAppearance colorKey={colorKey} iconType={iconType} iconValue={iconValue} size={size} /><span>{name}</span></span>;
}
