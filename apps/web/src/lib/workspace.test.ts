import { describe, expect, it } from "vitest";

import {
  assignableRoles,
  canChangeRole,
  canInvite,
  canListInvitations,
  canManageWorkspace,
  canRemoveMember,
  invitableRoles,
} from "./workspace";

describe("canManageWorkspace", () => {
  it("allows the managing roles", () => {
    for (const role of ["owner", "admin", "member"]) {
      expect(canManageWorkspace({ role })).toBe(true);
    }
  });

  it("denies viewers", () => {
    expect(canManageWorkspace({ role: "viewer" })).toBe(false);
  });

  // Failing open would render edit controls that every server request then rejects.
  it("denies an unrecognised role", () => {
    for (const role of ["", "guest", "Owner", "admin "]) {
      expect(canManageWorkspace({ role })).toBe(false);
    }
  });
});

// These mirror the tables in internal/workspace/collaboration_test.go. If the server policy
// changes and this does not, the interface starts offering actions the server rejects.
describe("collaboration permission mirror", () => {
  it("restricts pending invitations to owners and admins", () => {
    expect(canListInvitations("owner")).toBe(true);
    expect(canListInvitations("admin")).toBe(true);
    for (const role of ["member", "viewer", "", "auditor"]) {
      expect(canListInvitations(role)).toBe(false);
    }
  });

  it("lets an owner invite any non-owner role", () => {
    expect(invitableRoles("owner")).toEqual(["admin", "member", "viewer"]);
    expect(canInvite("owner", "owner")).toBe(false);
  });

  it("stops an admin inviting a peer or a superior", () => {
    expect(invitableRoles("admin")).toEqual(["member", "viewer"]);
    expect(canInvite("admin", "admin")).toBe(false);
    expect(canInvite("admin", "owner")).toBe(false);
  });

  it("gives members and viewers no invitation rights", () => {
    for (const role of ["member", "viewer", "", "auditor"]) {
      expect(invitableRoles(role)).toEqual([]);
    }
  });

  it("lets an owner set any role, including transferring ownership", () => {
    expect(canChangeRole("owner", "member", "admin")).toBe(true);
    expect(canChangeRole("owner", "member", "owner")).toBe(true);
    expect(canChangeRole("owner", "owner", "member")).toBe(true);
    expect(assignableRoles("owner", "member")).toEqual(["owner", "admin", "member", "viewer"]);
  });

  it("confines an admin to moving members and viewers", () => {
    expect(canChangeRole("admin", "member", "viewer")).toBe(true);
    expect(canChangeRole("admin", "viewer", "member")).toBe(true);
    expect(canChangeRole("admin", "member", "admin")).toBe(false);
    expect(canChangeRole("admin", "admin", "member")).toBe(false);
    expect(canChangeRole("admin", "owner", "member")).toBe(false);
    expect(assignableRoles("admin", "member")).toEqual(["member", "viewer"]);
  });

  it("rejects a role it does not recognise", () => {
    expect(canChangeRole("owner", "member", "auditor")).toBe(false);
    expect(canChangeRole("owner", "member", "")).toBe(false);
  });

  it("lets anyone leave, whatever their role", () => {
    expect(canRemoveMember("me", "me", "viewer", "viewer")).toBe(true);
    expect(canRemoveMember("me", "me", "owner", "owner")).toBe(true);
  });

  it("confines removal rights to the documented policy", () => {
    expect(canRemoveMember("actor", "target", "owner", "admin")).toBe(true);
    expect(canRemoveMember("actor", "target", "admin", "member")).toBe(true);
    expect(canRemoveMember("actor", "target", "admin", "viewer")).toBe(true);
    expect(canRemoveMember("actor", "target", "admin", "admin")).toBe(false);
    expect(canRemoveMember("actor", "target", "admin", "owner")).toBe(false);
    expect(canRemoveMember("actor", "target", "member", "viewer")).toBe(false);
    expect(canRemoveMember("actor", "target", "viewer", "member")).toBe(false);
  });
});
