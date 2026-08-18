import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import {
  type Account,
  type AccountWriteRequest,
  type ExchangeRate,
  type SessionResponse,
  accountsQueryKey,
  archiveAccount,
  createAccount,
  exchangeRatesQueryKey,
  financialProjectionQueryPrefix,
  listAccounts,
  listExchangeRates,
  updateAccount,
} from "../../api/client";
import { MutationError } from "../../components/MutationError";
import { ResourceState } from "../../components/ResourceState";
import {
  type Currency,
  SUPPORTED_CURRENCIES,
  convertMinor,
  currencyLabel,
  formatMoney,
} from "../../lib/currency";

type Workspace = SessionResponse["workspaces"][number];

export function AccountsPanel({ workspace, canManage }: { workspace: Workspace; canManage: boolean }) {
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: accountsQueryKey(workspace.id),
    queryFn: () => listAccounts(workspace.id),
  });
  const rates = useQuery({
    queryKey: exchangeRatesQueryKey(workspace.id),
    queryFn: () => listExchangeRates(workspace.id),
  });
  const [displayCurrency, setDisplayCurrency] = useState<Currency>(workspace.base_currency);
  const [editing, setEditing] = useState<Account>();
  const [name, setName] = useState("");
  const [type, setType] = useState<AccountWriteRequest["type"]>("bank");
  const [currency, setCurrency] = useState<Currency>(workspace.base_currency);
  const [institutionName, setInstitutionName] = useState("");
  const save = useMutation({
    mutationFn: (input: AccountWriteRequest) =>
      editing
        ? updateAccount(workspace.id, editing.id, input)
        : createAccount(workspace.id, input),
    onSuccess: async () => {
      reset();
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: accountsQueryKey(workspace.id) }),
        queryClient.invalidateQueries({ queryKey: financialProjectionQueryPrefix(workspace.id) }),
      ]);
    },
  });
  const archive = useMutation({
    mutationFn: (accountId: string) => archiveAccount(workspace.id, accountId),
    onSuccess: () => Promise.all([
      queryClient.invalidateQueries({ queryKey: accountsQueryKey(workspace.id) }),
      queryClient.invalidateQueries({ queryKey: financialProjectionQueryPrefix(workspace.id) }),
    ]),
  });

  function reset() {
    setEditing(undefined);
    setName("");
    setType("bank");
    setCurrency(workspace.base_currency);
    setInstitutionName("");
  }

  function edit(account: Account) {
    setEditing(account);
    setName(account.name);
    setType(account.type);
    setCurrency(account.currency);
    setInstitutionName(account.institution_name ?? "");
  }

  function submit(event: FormEvent) {
    event.preventDefault();
    save.mutate({
      name,
      type,
      currency,
      ...(institutionName.trim() ? { institution_name: institutionName.trim() } : {}),
    });
  }

  return (
    <section className="setup-panel" aria-labelledby="accounts-heading">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Where money lives</p>
          <h2 id="accounts-heading">Accounts</h2>
        </div>
        <span>{query.data?.length ?? 0} active</span>
      </div>
      <ResourceState query={query} empty="Create your first account to begin." />
      <BaseCurrencyTotal
        accounts={query.data ?? []}
        baseCurrency={workspace.base_currency}
        displayCurrency={displayCurrency}
        onDisplayCurrencyChange={setDisplayCurrency}
        rates={rates.data ?? []}
      />
      <div className="resource-list">
        {query.data?.map((account) => (
          <article className="resource-row" key={account.id}>
            <div>
              <strong>{account.name}</strong>
              <small>
                {account.type.replaceAll("_", " ")} · {account.currency}
                {account.institution_name ? ` · ${account.institution_name}` : ""}
              </small>
            </div>
            <div className="resource-actions">
              <span>{formatMoney(account.balance_minor, account.currency)}</span>
              {canManage ? (
                <>
                  <button className="text-button" type="button" onClick={() => edit(account)}>
                    Edit
                  </button>
                  <button
                    className="text-button danger"
                    type="button"
                    onClick={() => {
                      if (window.confirm(`Archive ${account.name}?`)) archive.mutate(account.id);
                    }}
                  >
                    Archive
                  </button>
                </>
              ) : null}
            </div>
          </article>
        ))}
      </div>
      <MutationError mutation={archive} />
      {canManage ? (
        <form className="resource-form" onSubmit={submit}>
          <h3>{editing ? `Edit ${editing.name}` : "Add account"}</h3>
          <label>
            Name
            <input required maxLength={100} value={name} onChange={(event) => setName(event.target.value)} />
          </label>
          <div className="form-columns">
            <label>
              Type
              <select value={type} onChange={(event) => setType(event.target.value as typeof type)}>
                <option value="bank">Bank</option>
                <option value="cash">Cash</option>
                <option value="credit_card">Credit card</option>
                <option value="savings">Savings</option>
                <option value="investment">Investment</option>
                <option value="other">Other</option>
              </select>
            </label>
            <label>
              Currency
              <select
                required
                value={currency}
                onChange={(event) => setCurrency(event.target.value as Currency)}
              >
                {SUPPORTED_CURRENCIES.map((code) => (
                  <option key={code} value={code}>
                    {code} · {currencyLabel(code)}
                  </option>
                ))}
              </select>
            </label>
          </div>
          <label>
            Institution (optional)
            <input maxLength={100} value={institutionName} onChange={(event) => setInstitutionName(event.target.value)} />
          </label>
          <MutationError mutation={save} />
          <div className="form-actions">
            <button disabled={save.isPending} type="submit">
              {editing ? "Save account" : "Add account"}
            </button>
            {editing ? (
              <button className="secondary-button" type="button" onClick={reset}>
                Cancel
              </button>
            ) : null}
          </div>
        </form>
      ) : (
        <p className="permission-note">Viewer access is read-only.</p>
      )}
    </section>
  );
}

function BaseCurrencyTotal({
  accounts,
  baseCurrency,
  displayCurrency,
  onDisplayCurrencyChange,
  rates,
}: {
  accounts: Account[];
  baseCurrency: Currency;
  displayCurrency: Currency;
  onDisplayCurrencyChange: (currency: Currency) => void;
  rates: ExchangeRate[];
}) {
  const inBaseCurrency = accounts.filter(
    (account) => account.currency === baseCurrency && !account.archived_at,
  );
  if (inBaseCurrency.length === 0) return null;

  const excluded = accounts.filter(
    (account) => account.currency !== baseCurrency && !account.archived_at,
  ).length;
  const total = inBaseCurrency.reduce((sum, account) => sum + account.balance_minor, 0);

  const selected = rates.find((rate) => rate.quote_currency === displayCurrency);
  const converted = selected ? convertMinor(total, selected.rate) : null;

  return (
    <div className="currency-total">
      <div>
        <strong>{formatMoney(total, baseCurrency)}</strong>
        {converted !== null && selected ? (
          <small>
            ≈ {formatMoney(converted, displayCurrency)} at the rate published{" "}
            {selected.rate_date}
          </small>
        ) : null}
        {excluded > 0 ? (
          <small>
            {excluded} account{excluded === 1 ? "" : "s"} in another currency{" "}
            {excluded === 1 ? "is" : "are"} not included in this total.
          </small>
        ) : null}
      </div>
      {rates.length > 0 ? (
        <label>
          Show in
          <select
            value={displayCurrency}
            onChange={(event) => onDisplayCurrencyChange(event.target.value as Currency)}
          >
            <option value={baseCurrency}>{baseCurrency}</option>
            {rates.map((rate) => (
              <option key={rate.quote_currency} value={rate.quote_currency}>
                {rate.quote_currency}
              </option>
            ))}
          </select>
        </label>
      ) : null}
    </div>
  );
}
