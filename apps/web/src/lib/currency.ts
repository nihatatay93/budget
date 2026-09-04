import type { components } from "../api/generated/schema";
import { t } from "./i18n";

export type Currency = components["schemas"]["Currency"];

// Record<Currency, string> is the guard: adding a currency to the OpenAPI enum fails the
// build here until it is given a label, so the dropdown can never silently omit one.
const CURRENCY_LABEL_KEYS: Record<Currency, string> = {
  TRY: "currency.try",
  USD: "currency.usd",
  EUR: "currency.eur",
};

export const SUPPORTED_CURRENCIES = Object.keys(CURRENCY_LABEL_KEYS) as Currency[];

export function currencyLabel(currency: Currency): string {
  return t(CURRENCY_LABEL_KEYS[currency]);
}

// Every supported currency uses two minor-unit decimal places, which is what makes this a
// plain division. See docs/decisions/0005-supported-currencies-and-display-conversion.md.
const MINOR_UNITS_PER_MAJOR = 100;

/**
 * Converts minor units by an exact decimal rate, rounding once with ties away from zero.
 *
 * The rate arrives as a decimal string and is applied with BigInt rather than Number so the
 * published precision is not lost to binary floating point, matching the server's rule in
 * docs/decisions/0005-supported-currencies-and-display-conversion.md.
 */
export function convertMinor(amountMinor: number, rate: string): number | null {
  // OpenAPI maps int64 to number in TypeScript. Refuse to display a conversion if either
  // side cannot be represented exactly instead of presenting a plausible but wrong amount.
  if (!Number.isSafeInteger(amountMinor)) return null;
  const match = /^(\d+)(?:\.(\d+))?$/.exec(rate.trim());
  if (!match) return null;
  const [, whole, fraction = ""] = match;
  const scale = BigInt(10) ** BigInt(fraction.length);
  const numerator = BigInt(whole + fraction);
  if (numerator === BigInt(0)) return null;

  const amount = BigInt(Math.trunc(amountMinor));
  const product = amount * numerator;
  const sign = product < BigInt(0) ? BigInt(-1) : BigInt(1);
  const magnitude = product * sign;
  // Add half the divisor before truncating so exact halves round away from zero.
  const rounded = (magnitude * BigInt(2) + scale) / (scale * BigInt(2));
  const result = Number(rounded * sign);
  return Number.isSafeInteger(result) ? result : null;
}

export function formatMoney(amountMinor: number, currency: Currency): string {
  if (!Number.isSafeInteger(amountMinor)) return t("Amount unavailable");
  return new Intl.NumberFormat(undefined, {
    style: "currency",
    currency,
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(amountMinor / MINOR_UNITS_PER_MAJOR);
}

/** Parses an exact two-decimal major-unit input without passing through floating point. */
export function parseMajorAmount(value: string): number | null {
  const match = /^([+-]?)(?:(\d+)(?:\.(\d{0,2}))?|\.(\d{1,2}))$/.exec(value.trim());
  if (!match) return null;
  const [, sign, whole = "0", fractionA, fractionB] = match;
  const fraction = (fractionA ?? fractionB ?? "").padEnd(2, "0");
  const magnitude = BigInt(whole) * BigInt(MINOR_UNITS_PER_MAJOR) + BigInt(fraction || "0");
  const minor = Number(sign === "-" ? -magnitude : magnitude);
  return Number.isSafeInteger(minor) ? minor : null;
}

export function majorAmountInput(amountMinor: number): string {
  if (!Number.isSafeInteger(amountMinor)) return "";
  const sign = amountMinor < 0 ? "-" : "";
  const magnitude = Math.abs(amountMinor);
  return `${sign}${Math.floor(magnitude / MINOR_UNITS_PER_MAJOR)}.${String(magnitude % MINOR_UNITS_PER_MAJOR).padStart(2, "0")}`;
}
