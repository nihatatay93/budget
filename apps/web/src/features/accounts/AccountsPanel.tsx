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
  spendingAnalysisQueryPrefix,
  listAccounts,
  listExchangeRates,
  updateAccount,
} from "../../api/client";
import { MutationError } from "../../components/MutationError";
import { AppIcon } from "../../components/ExperiencePrimitives";
import {
  EmptyState,
  InlineNotice,
  LoadingState,
  ModalDialog,
  MoneyAmount,
  StatusBadge,
  ToastRegion,
  type ToastMessage,
} from "../../components/Presentation";
import {
  type Currency,
  SUPPORTED_CURRENCIES,
  convertMinor,
  currencyLabel,
  formatMoney,
} from "../../lib/currency";
import { t } from "../../lib/i18n";

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
  const [editorOpen, setEditorOpen] = useState(false);
  const [archiving, setArchiving] = useState<Account>();
  const [toasts, setToasts] = useState<ToastMessage[]>([]);
  const save = useMutation({
    mutationFn: (input: AccountWriteRequest) =>
      editing
        ? updateAccount(workspace.id, editing.id, input)
        : createAccount(workspace.id, input),
    onSuccess: async (account) => {
      setToasts([{
        id: `account-${account.id}-saved`,
        title: editing ? t("Account updated") : t("Account created"),
        tone: "positive",
      }]);
      reset();
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: accountsQueryKey(workspace.id) }),
        queryClient.invalidateQueries({ queryKey: financialProjectionQueryPrefix(workspace.id) }),
        queryClient.invalidateQueries({ queryKey: spendingAnalysisQueryPrefix(workspace.id) }),
      ]);
    },
  });
  const archive = useMutation({
    mutationFn: (accountId: string) => archiveAccount(workspace.id, accountId),
    onSuccess: async () => {
      setToasts([{
        id: `account-archive-${archiving?.id ?? "complete"}`,
        title: t("Account archived"),
        description: t("Its historical entries remain part of the ledger."),
        tone: "positive",
      }]);
      setArchiving(undefined);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: accountsQueryKey(workspace.id) }),
        queryClient.invalidateQueries({ queryKey: financialProjectionQueryPrefix(workspace.id) }),
        queryClient.invalidateQueries({ queryKey: spendingAnalysisQueryPrefix(workspace.id) }),
      ]);
    },
  });

  function reset() {
    setEditing(undefined);
    setName("");
    setType("bank");
    setCurrency(workspace.base_currency);
    setInstitutionName("");
    setEditorOpen(false);
    save.reset();
  }

  function edit(account: Account) {
    setEditing(account);
    setName(account.name);
    setType(account.type);
    setCurrency(account.currency);
    setInstitutionName(account.institution_name ?? "");
    setEditorOpen(true);
  }

  function create() {
    reset();
    setEditorOpen(true);
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

  function confirmArchive(account: Account) {
    archive.reset();
    setArchiving(account);
  }

  const accounts = query.data ?? [];
  const activeAccounts = accounts.filter((account) => !account.archived_at);
  const archivedAccounts = accounts.filter((account) => account.archived_at);

  return (
    <section className="accounts-workspace" aria-labelledby="accounts-heading">
      <div className="resource-destination-heading">
        <div>
          <p className="eyebrow">{t("Where money lives")}</p>
          <h2 id="accounts-heading">{t("Accounts")}</h2>
          <p>{t("Balances come from posted entries; pending activity is reflected in projections.")}</p>
        </div>
        {canManage ? <button onClick={create} type="button">{t("Add account")}</button> : null}
      </div>
      {query.isPending ? <LoadingState label={t("Loading accounts")} rows={4} /> : null}
      {query.isError ? (
        <InlineNotice
          action={<button className="secondary-button" onClick={() => void query.refetch()} type="button">{t("Try again")}</button>}
          title={t("Accounts could not be loaded")}
          tone="danger"
        >
          <p>{query.error.message}</p>
        </InlineNotice>
      ) : null}
      <BaseCurrencyTotal
        accounts={accounts}
        baseCurrency={workspace.base_currency}
        displayCurrency={displayCurrency}
        onDisplayCurrencyChange={setDisplayCurrency}
        rates={rates.data ?? []}
      />
      {!query.isPending && !query.isError && activeAccounts.length === 0 ? (
        <EmptyState
          action={canManage ? <button onClick={create} type="button">{t("Create first account")}</button> : undefined}
          description={t("Accounts are required before transactions can affect a balance.")}
          icon="accounts"
          title={t("No active accounts")}
        />
      ) : null}
      {activeAccounts.length > 0 ? (
        <div className="account-card-grid">
          {activeAccounts.map((account) => (
            <AccountCard account={account} canManage={canManage} key={account.id} onArchive={confirmArchive} onEdit={edit} />
          ))}
        </div>
      ) : null}
      {archivedAccounts.length > 0 ? (
        <details className="archived-resource-group">
          <summary>{t(
            archivedAccounts.length === 1 ? "{count} archived account" : "{count} archived accounts",
            { count: archivedAccounts.length },
          )}</summary>
          <div className="account-card-grid account-card-grid-archived">
            {archivedAccounts.map((account) => (
              <AccountCard account={account} canManage={false} key={account.id} onArchive={confirmArchive} onEdit={edit} />
            ))}
          </div>
        </details>
      ) : null}
      {!canManage && !query.isPending ? (
        <InlineNotice title={t("Read-only accounts")}><p>{t("Viewer access can review balances but cannot change account settings.")}</p></InlineNotice>
      ) : null}
      {canManage ? (
        <ModalDialog
          description={t("Account currency is locked after ledger history exists. Balances are always derived from entries.")}
          footer={(
            <>
              <button className="secondary-button" onClick={reset} type="button">{t("Cancel")}</button>
              <button disabled={save.isPending} form="account-editor" type="submit">
                {save.isPending ? t("Saving…") : editing ? t("Save account") : t("Add account")}
              </button>
            </>
          )}
          onClose={reset}
          open={editorOpen}
          placement="drawer"
          title={editing ? t("Edit {name}", { name: editing.name }) : t("Add account")}
        >
          <form className="resource-form resource-editor-form" id="account-editor" onSubmit={submit}>
          <label>
            {t("Name")}
            <input required maxLength={100} value={name} onChange={(event) => setName(event.target.value)} />
          </label>
          <div className="form-columns">
            <label>
              {t("Type")}
              <select value={type} onChange={(event) => setType(event.target.value as typeof type)}>
                {(["bank", "cash", "credit_card", "savings", "investment", "other"] as const).map((accountType) => <option key={accountType} value={accountType}>{t(`account.type.${accountType}`)}</option>)}
              </select>
            </label>
            <label>
              {t("Currency")}
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
            {t("Institution (optional)")}
            <input maxLength={100} value={institutionName} onChange={(event) => setInstitutionName(event.target.value)} />
          </label>
          <MutationError mutation={save} />
          </form>
        </ModalDialog>
      ) : null}
      <ModalDialog
        description={t("Archiving removes this account from active workflows while preserving every historical entry and report.")}
        footer={(
          <>
            <button className="secondary-button" onClick={() => setArchiving(undefined)} type="button">{t("Cancel")}</button>
            <button
              className="danger-button"
              disabled={archive.isPending}
              onClick={() => archiving && archive.mutate(archiving.id)}
              type="button"
            >
              {archive.isPending ? t("Archiving…") : t("Archive account")}
            </button>
          </>
        )}
        onClose={() => setArchiving(undefined)}
        open={Boolean(archiving)}
        title={t("Archive {name}?", { name: archiving?.name ?? t("account") })}
      >
        <InlineNotice title={t("Historical balances stay intact")} tone="warning">
          <p>{t("You can no longer select the archived account for new transactions.")}</p>
        </InlineNotice>
        <MutationError mutation={archive} />
      </ModalDialog>
      <ToastRegion messages={toasts} onDismiss={(id) => setToasts((current) => current.filter((toast) => toast.id !== id))} />
    </section>
  );
}

function AccountCard({ account, canManage, onArchive, onEdit }: {
  account: Account;
  canManage: boolean;
  onArchive: (account: Account) => void;
  onEdit: (account: Account) => void;
}) {
  return (
    <article className={`account-card${account.archived_at ? " account-card-archived" : ""}`}>
      <div className="account-card-heading">
        <span aria-hidden="true" className="resource-icon"><AppIcon name="accounts" size={18} /></span>
        <div>
          <strong>{account.name}</strong>
          <small>{account.institution_name || t(`account.type.${account.type}`)}</small>
        </div>
        <StatusBadge>{account.currency}</StatusBadge>
      </div>
      <div className="account-card-balance">
        <span>{account.archived_at ? t("Historical balance") : t("Posted balance")}</span>
        <strong><MoneyAmount amount={account.balance_minor} currency={account.currency} emphasis="hero" /></strong>
      </div>
      <div className="account-card-footer">
        <span>{t(`account.type.${account.type}`)}</span>
        {canManage ? (
          <div>
            <button className="text-button" onClick={() => onEdit(account)} type="button">{t("Edit")}</button>
            <button className="text-button danger" onClick={() => onArchive(account)} type="button">{t("Archive")}</button>
          </div>
        ) : null}
      </div>
    </article>
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
        <span>{t("Base-currency total")}</span>
        <strong>{formatMoney(total, baseCurrency)}</strong>
        {converted !== null && selected ? (
          <small>
            {t("≈ {amount} at the rate published {date}", { amount: formatMoney(converted, displayCurrency), date: selected.rate_date })}
          </small>
        ) : null}
        {excluded > 0 ? (
          <small>
            {t("{count} account(s) in another currency are not included in this total.", { count: excluded })}
          </small>
        ) : null}
      </div>
      {rates.length > 0 ? (
        <label>
          {t("Show in")}
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
