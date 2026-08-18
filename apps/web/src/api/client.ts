import type { components } from "./generated/schema";

export type SessionResponse = components["schemas"]["SessionResponse"];
export type AuthResponse = components["schemas"]["AuthResponse"];
export type LoginRequest = components["schemas"]["LoginRequest"];
export type RegisterRequest = components["schemas"]["RegisterRequest"];
export type Account = components["schemas"]["Account"];
export type AccountWriteRequest = components["schemas"]["AccountWriteRequest"];
export type Category = components["schemas"]["Category"];
export type CategoryWriteRequest = components["schemas"]["CategoryWriteRequest"];
export type Transaction = components["schemas"]["Transaction"];
export type TransactionWriteRequest = components["schemas"]["TransactionWriteRequest"];
export type FinancialProjection = components["schemas"]["FinancialProjection"];
export type MonthlyBudget = components["schemas"]["MonthlyBudget"];
export type MonthlyBudgetWriteRequest = components["schemas"]["MonthlyBudgetWriteRequest"];
export type FinancialProjectionRange = { fromDate: string; toDate: string };
export const sessionQueryKey = ["session"] as const;
export const accountsQueryKey = (workspaceId: string) => ["workspaces", workspaceId, "accounts"] as const;
export const categoriesQueryKey = (workspaceId: string) =>
  ["workspaces", workspaceId, "categories"] as const;
export const transactionsQueryKey = (workspaceId: string) =>
  ["workspaces", workspaceId, "transactions"] as const;
export const financialProjectionQueryPrefix = (workspaceId: string) =>
  ["workspaces", workspaceId, "financial-projection"] as const;
export const financialProjectionQueryKey = (
  workspaceId: string,
  range?: FinancialProjectionRange,
) => [...financialProjectionQueryPrefix(workspaceId), range ?? "month-to-date"] as const;
export const monthlyBudgetQueryPrefix = (workspaceId: string) =>
  ["workspaces", workspaceId, "budgets"] as const;
export const monthlyBudgetQueryKey = (workspaceId: string, month: string) =>
  [...monthlyBudgetQueryPrefix(workspaceId), month] as const;

export class APIError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
  }
}

export async function requestJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    credentials: "include",
    headers: {
      Accept: "application/json",
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
      ...init?.headers,
    },
  });
  if (!response.ok) {
    let message = "The request could not be completed.";
    try {
      const body = (await response.json()) as components["schemas"]["ErrorResponse"];
      message = body.error.message;
    } catch {
      // Keep the generic message when an intermediary returns a non-JSON error.
    }
    throw new APIError(response.status, message);
  }
  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
}

export function getSession(): Promise<SessionResponse> {
  return requestJSON<SessionResponse>("/v1/session");
}

export function login(input: LoginRequest): Promise<AuthResponse> {
  return requestJSON<AuthResponse>("/v1/auth/login", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function register(input: RegisterRequest): Promise<AuthResponse> {
  return requestJSON<AuthResponse>("/v1/auth/register", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function logout(): Promise<void> {
  return requestJSON<void>("/v1/auth/logout", { method: "POST" });
}

export type ExchangeRate = components["schemas"]["ExchangeRate"];

export const exchangeRatesQueryKey = (workspaceId: string) =>
  ["workspaces", workspaceId, "exchange-rates"] as const;

/**
 * Conversion is an optional extra: the server returns 503 when rate fetching is disabled or
 * the provider is unreachable. That is a normal configuration, not an error, so it resolves
 * to an empty list and the UI simply offers no converted figure.
 */
export async function listExchangeRates(workspaceId: string): Promise<ExchangeRate[]> {
  try {
    const response = await requestJSON<components["schemas"]["ExchangeRateListResponse"]>(
      `/v1/workspaces/${encodeURIComponent(workspaceId)}/exchange-rates`,
    );
    return response.rates;
  } catch (error) {
    if (error instanceof APIError && error.status === 503) return [];
    throw error;
  }
}

export async function listAccounts(workspaceId: string): Promise<Account[]> {
  const response = await requestJSON<components["schemas"]["AccountListResponse"]>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/accounts`,
  );
  return response.accounts;
}

export function createAccount(workspaceId: string, input: AccountWriteRequest): Promise<Account> {
  return requestJSON<Account>(`/v1/workspaces/${encodeURIComponent(workspaceId)}/accounts`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateAccount(
  workspaceId: string,
  accountId: string,
  input: AccountWriteRequest,
): Promise<Account> {
  return requestJSON<Account>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/accounts/${encodeURIComponent(accountId)}`,
    { method: "PUT", body: JSON.stringify(input) },
  );
}

export function archiveAccount(workspaceId: string, accountId: string): Promise<void> {
  return requestJSON<void>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/accounts/${encodeURIComponent(accountId)}`,
    { method: "DELETE" },
  );
}

export async function listCategories(workspaceId: string): Promise<Category[]> {
  const response = await requestJSON<components["schemas"]["CategoryListResponse"]>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/categories`,
  );
  return response.categories;
}

export function createCategory(workspaceId: string, input: CategoryWriteRequest): Promise<Category> {
  return requestJSON<Category>(`/v1/workspaces/${encodeURIComponent(workspaceId)}/categories`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateCategory(
  workspaceId: string,
  categoryId: string,
  input: CategoryWriteRequest,
): Promise<Category> {
  return requestJSON<Category>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/categories/${encodeURIComponent(categoryId)}`,
    { method: "PUT", body: JSON.stringify(input) },
  );
}

export function archiveCategory(workspaceId: string, categoryId: string): Promise<void> {
  return requestJSON<void>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/categories/${encodeURIComponent(categoryId)}`,
    { method: "DELETE" },
  );
}

export async function listTransactions(workspaceId: string): Promise<Transaction[]> {
  const response = await requestJSON<components["schemas"]["TransactionListResponse"]>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/transactions`,
  );
  return response.transactions;
}

export function createTransaction(
  workspaceId: string,
  input: TransactionWriteRequest,
): Promise<Transaction> {
  return requestJSON<Transaction>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/transactions`,
    { method: "POST", body: JSON.stringify(input) },
  );
}

export function updateTransaction(
  workspaceId: string,
  transactionId: string,
  input: TransactionWriteRequest,
): Promise<Transaction> {
  return requestJSON<Transaction>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/transactions/${encodeURIComponent(transactionId)}`,
    { method: "PUT", body: JSON.stringify(input) },
  );
}

export function deleteTransaction(workspaceId: string, transactionId: string): Promise<void> {
  return requestJSON<void>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/transactions/${encodeURIComponent(transactionId)}`,
    { method: "DELETE" },
  );
}

export function getFinancialProjection(
  workspaceId: string,
  range?: FinancialProjectionRange,
): Promise<FinancialProjection> {
  const query = range
    ? `?${new URLSearchParams({ from_date: range.fromDate, to_date: range.toDate })}`
    : "";
  return requestJSON<FinancialProjection>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/financial-projection${query}`,
  );
}

export function getMonthlyBudget(workspaceId: string, month: string): Promise<MonthlyBudget> {
  const query = new URLSearchParams({ month });
  return requestJSON<MonthlyBudget>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/budgets?${query}`,
  );
}

export function replaceMonthlyBudget(
  workspaceId: string,
  month: string,
  input: MonthlyBudgetWriteRequest,
): Promise<MonthlyBudget> {
  return requestJSON<MonthlyBudget>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/budgets/${encodeURIComponent(month)}`,
    { method: "PUT", body: JSON.stringify(input) },
  );
}

export type WorkspaceMember = components["schemas"]["WorkspaceMember"];
export type WorkspaceInvitation = components["schemas"]["WorkspaceInvitation"];
export type CreateWorkspaceInvitationRequest =
  components["schemas"]["CreateWorkspaceInvitationRequest"];
export type CreateWorkspaceInvitationResponse =
  components["schemas"]["CreateWorkspaceInvitationResponse"];
export type WorkspaceMembershipAcceptance =
  components["schemas"]["WorkspaceMembershipAcceptance"];

export const membersQueryKey = (workspaceId: string) =>
  ["workspaces", workspaceId, "members"] as const;
export const invitationsQueryKey = (workspaceId: string) =>
  ["workspaces", workspaceId, "invitations"] as const;

function workspacePath(workspaceId: string, suffix: string): string {
  return `/v1/workspaces/${encodeURIComponent(workspaceId)}${suffix}`;
}

export async function listWorkspaceMembers(workspaceId: string): Promise<WorkspaceMember[]> {
  const response = await requestJSON<components["schemas"]["WorkspaceMemberListResponse"]>(
    workspacePath(workspaceId, "/members"),
  );
  return response.members;
}

export function updateWorkspaceMemberRole(
  workspaceId: string,
  userId: string,
  role: WorkspaceMember["role"],
): Promise<WorkspaceMember> {
  return requestJSON<WorkspaceMember>(
    workspacePath(workspaceId, `/members/${encodeURIComponent(userId)}`),
    { method: "PATCH", body: JSON.stringify({ role }) },
  );
}

export function removeWorkspaceMember(workspaceId: string, userId: string): Promise<void> {
  return requestJSON<void>(
    workspacePath(workspaceId, `/members/${encodeURIComponent(userId)}`),
    { method: "DELETE" },
  );
}

/**
 * Invitations are visible only to owners and admins, so a viewer or member receives 403 here.
 * Callers gate on canListInvitations rather than relying on the failure.
 */
export async function listWorkspaceInvitations(
  workspaceId: string,
): Promise<WorkspaceInvitation[]> {
  const response = await requestJSON<components["schemas"]["WorkspaceInvitationListResponse"]>(
    workspacePath(workspaceId, "/invitations"),
  );
  return response.invitations;
}

/** The response discloses the acceptance token once. Treat it as a credential. */
export function createWorkspaceInvitation(
  workspaceId: string,
  input: CreateWorkspaceInvitationRequest,
): Promise<CreateWorkspaceInvitationResponse> {
  return requestJSON<CreateWorkspaceInvitationResponse>(
    workspacePath(workspaceId, "/invitations"),
    { method: "POST", body: JSON.stringify(input) },
  );
}

export function revokeWorkspaceInvitation(
  workspaceId: string,
  invitationId: string,
): Promise<void> {
  return requestJSON<void>(
    workspacePath(workspaceId, `/invitations/${encodeURIComponent(invitationId)}`),
    { method: "DELETE" },
  );
}

/** The token travels in the body, never the URL, so it stays out of logs and history. */
export function acceptWorkspaceInvitation(token: string): Promise<WorkspaceMembershipAcceptance> {
  return requestJSON<WorkspaceMembershipAcceptance>("/v1/invitations/accept", {
    method: "POST",
    body: JSON.stringify({ token }),
  });
}
