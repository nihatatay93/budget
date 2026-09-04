import { describe, expect, it } from "vitest";

import type { components } from "../api/generated/schema";
import { analysisGranularities, granularityLabel } from "./analysis";
import { SUPPORTED_CURRENCIES, currencyLabel } from "./currency";
import { ASSIGNABLE_ROLES, INVITATION_ROLES, canInvite } from "./workspace";

/*
 * The client keeps hand-written copies of values the contract also names, so the interface
 * can enumerate them without parsing the schema at runtime. A copy that drifts renders a
 * control the server rejects, or silently omits one it accepts.
 *
 * These assertions compare against types generated from api/openapi.yaml, so regenerating
 * the schema after a contract change is what fails the build here.
 */

type ContractCurrency = components["schemas"]["Currency"];
type ContractRole = components["schemas"]["WorkspaceRole"];
type ContractInvitationRole = components["schemas"]["WorkspaceInvitationRole"];
type ContractGranularity = components["schemas"]["AnalysisGranularity"];

describe("contract mirrors", () => {
  it("covers every contract currency with a label", () => {
    // Record<ContractCurrency, …> fails to compile if a currency is missing, so this checks
    // the reverse direction: nothing extra is listed.
    const contract: Record<ContractCurrency, true> = { TRY: true, USD: true, EUR: true };
    expect([...SUPPORTED_CURRENCIES].sort()).toEqual(Object.keys(contract).sort());
    for (const currency of SUPPORTED_CURRENCIES) {
      expect(currencyLabel(currency)).toBeTruthy();
    }
  });

  it("mirrors every workspace role", () => {
    const contract: Record<ContractRole, true> = {
      owner: true, admin: true, member: true, viewer: true,
    };
    expect([...ASSIGNABLE_ROLES].sort()).toEqual(Object.keys(contract).sort());
  });

  it("offers every analysis granularity the contract accepts, and no other", () => {
    const contract: Record<ContractGranularity, true> = { day: true, week: true, month: true };
    expect([...analysisGranularities].sort()).toEqual(Object.keys(contract).sort());
    for (const granularity of analysisGranularities) {
      // A missing label would render a bucket control with the raw contract value on it.
      expect(granularityLabel(granularity)).not.toBe(`analysis.granularity.${granularity}`);
    }
  });

  it("mirrors invitation roles and excludes owner", () => {
    const contract: Record<ContractInvitationRole, true> = {
      admin: true, member: true, viewer: true,
    };
    expect([...INVITATION_ROLES].sort()).toEqual(Object.keys(contract).sort());
    expect(INVITATION_ROLES).not.toContain("owner");
    // The contract cannot express an owner invitation, so the policy must not offer one.
    expect(canInvite("owner", "owner")).toBe(false);
  });
});
