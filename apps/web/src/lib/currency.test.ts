import { describe, expect, it } from "vitest";

import { convertMinor, formatMoney, majorAmountInput, parseMajorAmount } from "./currency";

describe("convertMinor", () => {
  it("converts the ADR 0005 example without drift", () => {
    // 50,000.00 TRY at the published TRY->USD rate.
    expect(convertMinor(5000000, "0.02088")).toBe(104400);
  });

  it("keeps precision across the full 12-decimal rate range", () => {
    // The smallest rate the server will serve, applied to an amount large enough to survive.
    expect(convertMinor(1_000_000_000_000, "0.000000000001")).toBe(1);
    // A rate whose binary float representation is inexact must still land exactly.
    expect(convertMinor(100, "1.1")).toBe(110);
  });

  it("rounds exact halves away from zero in both directions", () => {
    expect(convertMinor(1, "0.5")).toBe(1);
    expect(convertMinor(-1, "0.5")).toBe(-1);
    expect(convertMinor(1, "0.49")).toBe(0);
  });

  it("handles negative balances", () => {
    expect(convertMinor(-35000, "0.02088")).toBe(-731);
  });

  it("rejects a rate it cannot parse", () => {
    for (const rate of ["", "abc", "-1.5", "0", "1e-5"]) {
      expect(convertMinor(1000, rate)).toBeNull();
    }
  });

  it("refuses values that JavaScript cannot represent exactly", () => {
    expect(convertMinor(Number.MAX_SAFE_INTEGER + 1, "1")).toBeNull();
    expect(convertMinor(Number.MAX_SAFE_INTEGER, "2")).toBeNull();
  });
});

describe("major-unit input", () => {
  it("parses signed amounts exactly into minor units", () => {
    expect(parseMajorAmount("-12.50")).toBe(-1250);
    expect(parseMajorAmount("3.4")).toBe(340);
    expect(parseMajorAmount(".05")).toBe(5);
    expect(parseMajorAmount("1.234")).toBeNull();
    expect(parseMajorAmount("not money")).toBeNull();
  });

  it("formats minor units for editing without floating-point arithmetic", () => {
    expect(majorAmountInput(-5)).toBe("-0.05");
    expect(majorAmountInput(123456)).toBe("1234.56");
  });
});

describe("formatMoney", () => {
  it("renders minor units as a major-unit amount", () => {
    expect(formatMoney(5000000, "USD")).toContain("50,000.00");
    expect(formatMoney(0, "EUR")).toContain("0.00");
  });

  it("does not format a minor-unit value JavaScript cannot represent exactly", () => {
    expect(formatMoney(Number.MAX_SAFE_INTEGER + 1, "USD")).toBe("Amount unavailable");
  });
});
