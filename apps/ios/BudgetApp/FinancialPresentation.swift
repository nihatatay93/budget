import Foundation

enum BudgetUsageState: Equatable {
    case noTarget
    case onTrack
    case overspent
    case refundCredit
}

struct BudgetUsagePresentation: Equatable {
    let progress: Double
    let state: BudgetUsageState

    init(planned: Int64, used: Int64, remaining: Int64) {
        if planned <= 0 {
            progress = 0
            state = .noTarget
        } else {
            progress = min(1, max(0, Double(used) / Double(planned)))
            if remaining < 0 {
                state = .overspent
            } else if used < 0 {
                state = .refundCredit
            } else {
                state = .onTrack
            }
        }
    }
}

struct AccountCurrencySummary: Identifiable, Equatable {
    let currency: BudgetCurrency
    let accountCount: Int
    let balanceMinor: Int64?

    var id: BudgetCurrency { currency }
}

func accountCurrencySummaries(_ accounts: [BudgetAccount]) -> [AccountCurrencySummary] {
    Dictionary(grouping: accounts.filter { $0.archivedAt == nil }, by: \.currency)
        .map { currency, accounts in
            var total: Int64 = 0
            var overflowed = false
            for account in accounts {
                let addition = total.addingReportingOverflow(account.balanceMinor)
                if addition.overflow {
                    overflowed = true
                    break
                }
                total = addition.partialValue
            }
            return AccountCurrencySummary(
                currency: currency,
                accountCount: accounts.count,
                balanceMinor: overflowed ? nil : total
            )
        }
        .sorted { $0.currency.rawValue < $1.currency.rawValue }
}
