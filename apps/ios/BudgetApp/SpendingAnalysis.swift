import Foundation

/// Presentation logic for the spending-analysis screen, kept apart from the view so the
/// arithmetic behind every reading is testable without rendering anything.
///
/// The server has already resolved the window in the workspace timezone and oriented every
/// amount for reading. What is left here is what the phone must decide: which window to ask
/// for, how a category tree ranks, and which few sentences the numbers already prove. These
/// rules deliberately match the web client's, so the same period reads the same on both.

// MARK: - Dates

private let secondsPerDay: TimeInterval = 86_400

/// Analysis dates travel as calendar days, not instants. Interpreting them in the device's
/// timezone would shift a boundary by a day for anyone east or west of the workspace, so
/// every conversion here uses a fixed UTC calendar.
private var analysisCalendar: Calendar {
    var calendar = Calendar(identifier: .gregorian)
    calendar.timeZone = TimeZone(secondsFromGMT: 0) ?? .current
    calendar.locale = Locale(identifier: "en_US_POSIX")
    return calendar
}

func analysisDate(from value: String) -> Date? {
    let parts = value.split(separator: "-").compactMap { Int($0) }
    guard parts.count == 3 else { return nil }
    return analysisCalendar.date(
        from: DateComponents(year: parts[0], month: parts[1], day: parts[2])
    )
}

func analysisDateString(_ date: Date) -> String {
    let components = analysisCalendar.dateComponents([.year, .month, .day], from: date)
    return String(
        format: "%04d-%02d-%02d",
        components.year ?? 0,
        components.month ?? 0,
        components.day ?? 0
    )
}

func analysisAddingDays(_ value: String, _ days: Int) -> String {
    guard let date = analysisDate(from: value) else { return value }
    return analysisDateString(date.addingTimeInterval(TimeInterval(days) * secondsPerDay))
}

/// ISO weekday, where 1 is Monday and 7 is Sunday, matching the analysis contract.
func analysisISOWeekday(_ value: String) -> Int {
    guard let date = analysisDate(from: value) else { return 1 }
    // Foundation numbers Sunday as 1; the contract numbers Monday as 1.
    let weekday = analysisCalendar.component(.weekday, from: date)
    return weekday == 1 ? 7 : weekday - 1
}

/// Today in the workspace timezone, so a preset means the same day the server means.
func analysisWorkspaceToday(_ date: Date = Date(), timezone: String) -> String {
    var calendar = Calendar(identifier: .gregorian)
    calendar.timeZone = TimeZone(identifier: timezone) ?? .current
    let components = calendar.dateComponents([.year, .month, .day], from: date)
    return String(
        format: "%04d-%02d-%02d",
        components.year ?? 0,
        components.month ?? 0,
        components.day ?? 0
    )
}

private func analysisStartOfMonth(_ value: String) -> String {
    String(value.prefix(7)) + "-01"
}

private func analysisShiftingMonths(_ value: String, _ months: Int) -> String {
    guard let date = analysisDate(from: analysisStartOfMonth(value)),
          let shifted = analysisCalendar.date(byAdding: .month, value: months, to: date) else {
        return value
    }
    return analysisDateString(shifted)
}

private func analysisEndOfMonth(_ monthStart: String) -> String {
    analysisAddingDays(analysisShiftingMonths(monthStart, 1), -1)
}

// MARK: - Range presets

/// Inclusive windows anchored on the workspace's today. Each carries the bucket width that
/// reads well at that span, so switching presets never leaves an unreadable chart.
enum AnalysisRangePreset: String, CaseIterable, Identifiable, Sendable {
    case thisMonth
    case lastMonth
    case last3Months
    case last6Months
    case yearToDate
    case last12Months

    var id: Self { self }

    var title: String { L10n.text("analysis.range.\(rawValue)") }

    func range(today: String) -> BudgetAnalysisRange {
        let monthStart = analysisStartOfMonth(today)
        switch self {
        case .thisMonth:
            return BudgetAnalysisRange(fromDate: monthStart, toDate: today, granularity: .day)
        case .lastMonth:
            let previous = analysisShiftingMonths(monthStart, -1)
            return BudgetAnalysisRange(
                fromDate: previous,
                toDate: analysisEndOfMonth(previous),
                granularity: .day
            )
        case .last3Months:
            return BudgetAnalysisRange(
                fromDate: analysisShiftingMonths(monthStart, -2),
                toDate: today,
                granularity: .week
            )
        case .last6Months:
            return BudgetAnalysisRange(
                fromDate: analysisShiftingMonths(monthStart, -5),
                toDate: today,
                granularity: .month
            )
        case .yearToDate:
            return BudgetAnalysisRange(
                fromDate: "\(today.prefix(4))-01-01",
                toDate: today,
                granularity: .month
            )
        case .last12Months:
            return BudgetAnalysisRange(
                fromDate: analysisShiftingMonths(monthStart, -11),
                toDate: today,
                granularity: .month
            )
        }
    }
}

// MARK: - Derived readings

/// Relative movement against the comparison window. A previous window of zero has no ratio to
/// report: growth from nothing is unbounded, and a percentage would invent precision.
func analysisDeltaRatio(current: Int64, previous: Int64) -> Double? {
    guard previous != 0 else { return nil }
    return Double(current - previous) / Double(previous.magnitude)
}

func analysisFormattedPercent(_ ratio: Double, signed: Bool = true) -> String {
    let magnitude = abs(ratio)
    var style = FloatingPointFormatStyle<Double>.Percent.percent
        .precision(.fractionLength(magnitude >= 1 ? 0 : 1))
    if signed {
        style = style.sign(strategy: .always(includingZero: false))
    }
    return ratio.formatted(style)
}

/// Money spent per day of the window, which makes windows of different lengths comparable.
func analysisAveragePerDay(total: Int64, dayCount: Int64) -> Int64 {
    guard dayCount > 0 else { return 0 }
    return Int64((Double(total) / Double(dayCount)).rounded())
}

func analysisAveragePerTransaction(total: Int64, transactionCount: Int64) -> Int64 {
    guard transactionCount > 0 else { return 0 }
    return Int64((Double(total) / Double(transactionCount)).rounded())
}

/// The share of income that was not spent. Reported only when income exists, because a
/// savings rate against no income is not a meaningful figure.
func analysisSavingsRate(income: Int64, spending: Int64) -> Double? {
    guard income > 0 else { return nil }
    return Double(income - spending) / Double(income)
}

/// Counts read as prose, and "1 transactions" is the giveaway that they were not.
func analysisTransactionCount(_ count: Int64) -> String {
    let key = count == 1 ? "%lld transaction" : "%lld transactions"
    return String(format: L10n.text(key), count)
}

// MARK: - Categories

struct AnalysisCategoryNode: Identifiable, Sendable {
    let category: BudgetAnalysisCategory
    let children: [BudgetAnalysisCategory]
    let amountMinor: Int64
    let comparisonMinor: Int64
    let transactionCount: Int64
    let share: Double

    var id: String { category.id }
}

/// Top-level categories carry rolled-up totals, so a workspace that organizes spending into
/// subcategories still reads as a small number of meaningful groups. Children ride along for
/// progressive disclosure rather than competing with their parent in the ranking.
func analysisCategoryNodes(
    _ analysis: BudgetSpendingAnalysis,
    kind: BudgetCategoryKind
) -> [AnalysisCategoryNode] {
    let inKind = analysis.categories.filter { $0.kind == kind }
    let ranked = inKind
        .filter { $0.parentID == nil && $0.rolledUpBaseMinor != 0 }
        .sorted { $0.rolledUpBaseMinor > $1.rolledUpBaseMinor }
    let total = ranked.reduce(Int64(0)) { $0 + max($1.rolledUpBaseMinor, 0) }
    return ranked.map { category in
        AnalysisCategoryNode(
            category: category,
            children: inKind
                .filter { $0.parentID == category.id && $0.rolledUpBaseMinor != 0 }
                .sorted { $0.rolledUpBaseMinor > $1.rolledUpBaseMinor },
            amountMinor: category.rolledUpBaseMinor,
            comparisonMinor: category.comparisonRolledUpBaseMinor,
            transactionCount: category.rolledUpTransactionCount,
            share: total > 0 ? Double(max(category.rolledUpBaseMinor, 0)) / Double(total) : 0
        )
    }
}

/// Every category in one subtree, so a parent's trend includes the children it rolls up.
func analysisCategorySubtreeIDs(
    _ analysis: BudgetSpendingAnalysis,
    categoryID: String
) -> [String] {
    var ids = [categoryID]
    var index = 0
    while index < ids.count {
        let parent = ids[index]
        for candidate in analysis.categories
        where candidate.parentID == parent && !ids.contains(candidate.id) {
            ids.append(candidate.id)
        }
        index += 1
    }
    return ids
}

/// Bucketed activity for a set of categories, aligned to the analysis series so a category
/// trend shares the main chart's axis.
func analysisCategorySeries(
    _ analysis: BudgetSpendingAnalysis,
    categoryIDs: [String]
) -> [Int64] {
    let wanted = Set(categoryIDs)
    var byDate: [String: Int64] = [:]
    for point in analysis.categorySeries where wanted.contains(point.categoryID) {
        byDate[point.startDate, default: 0] += point.baseMinor
    }
    return analysis.series.enumerated().map { index, bucket in
        var total = byDate[bucket.startDate] ?? 0
        // Points are anchored to the bucket the server truncated to, which can precede a
        // clamped first bucket start. Fold anything earlier into the opening bucket.
        if index == 0 {
            for (date, amount) in byDate where date < bucket.startDate {
                total += amount
            }
        }
        return total
    }
}

// MARK: - Rhythm

struct AnalysisWeekdayReading: Identifiable, Sendable {
    let weekday: Int
    let spendingMinor: Int64
    let transactionCount: Int64
    let share: Double

    var id: Int { weekday }
}

/// All seven days, including the quiet ones, so the shape of a week is visible at a glance.
func analysisWeekdayReadings(_ analysis: BudgetSpendingAnalysis) -> [AnalysisWeekdayReading] {
    let byWeekday = Dictionary(
        analysis.weekdays.map { ($0.weekday, $0) },
        uniquingKeysWith: { first, _ in first }
    )
    let spending = (1...7).map { max(byWeekday[$0]?.spendingBaseMinor ?? 0, 0) }
    let peak = spending.max() ?? 0
    return (1...7).map { weekday in
        let amount = spending[weekday - 1]
        return AnalysisWeekdayReading(
            weekday: weekday,
            spendingMinor: amount,
            transactionCount: byWeekday[weekday]?.transactionCount ?? 0,
            share: peak > 0 ? Double(amount) / Double(peak) : 0
        )
    }
}

struct AnalysisCalendarCell: Identifiable, Sendable {
    let date: String
    let spendingMinor: Int64
    let transactionCount: Int64
    let intensity: Double

    var id: String { date }
}

struct AnalysisCalendarWeek: Identifiable, Sendable {
    let start: String
    /// Seven slots, Monday first. A nil slot is a day outside the analysis window.
    let cells: [AnalysisCalendarCell?]

    var id: String { start }
}

/// A Monday-aligned grid over the window. Leading and trailing gaps stay empty so the rows
/// line up with weekdays instead of sliding by the window's start day.
func analysisCalendarWeeks(_ analysis: BudgetSpendingAnalysis) -> [AnalysisCalendarWeek] {
    let byDate = Dictionary(
        analysis.days.map { ($0.date, $0) },
        uniquingKeysWith: { first, _ in first }
    )
    let peak = analysis.days.map { max($0.spendingBaseMinor, 0) }.max() ?? 0
    let from = analysis.period.fromDate
    let to = analysis.period.toDate
    var weeks: [AnalysisCalendarWeek] = []
    var cursor = analysisAddingDays(from, -(analysisISOWeekday(from) - 1))
    // A malformed window would otherwise spin forever; the guard keeps a bad response from
    // hanging the screen rather than merely drawing nothing.
    while cursor <= to, weeks.count < 400 {
        let start = cursor
        let cells: [AnalysisCalendarCell?] = (0..<7).map { offset in
            let date = analysisAddingDays(start, offset)
            guard date >= from, date <= to else { return nil }
            let day = byDate[date]
            let amount = max(day?.spendingBaseMinor ?? 0, 0)
            return AnalysisCalendarCell(
                date: date,
                spendingMinor: amount,
                transactionCount: day?.transactionCount ?? 0,
                intensity: peak > 0 ? Double(amount) / Double(peak) : 0
            )
        }
        weeks.append(AnalysisCalendarWeek(start: start, cells: cells))
        cursor = analysisAddingDays(start, 7)
    }
    return weeks
}

// MARK: - Insights

enum AnalysisInsightTone: Sendable {
    case neutral
    case positive
    case warning
    case danger
}

struct AnalysisInsight: Identifiable, Sendable {
    let id: String
    let tone: AnalysisInsightTone
    let title: String
    let detail: String
}

/// Reading a chart is work. These sentences state the few things the numbers already prove,
/// so nothing here is a projection or a recommendation — only what the window contains.
func analysisInsights(
    _ analysis: BudgetSpendingAnalysis,
    formatAmount: (Int64) -> String,
    formatCategory: (BudgetAnalysisCategory) -> String,
    formatWeekday: (Int) -> String,
    formatBucket: (String) -> String
) -> [AnalysisInsight] {
    var insights: [AnalysisInsight] = []
    let totals = analysis.totals

    if let spendingDelta = analysisDeltaRatio(
        current: totals.spendingBaseMinor,
        previous: totals.comparisonSpendingBaseMinor
    ), abs(spendingDelta) >= 0.05 {
        let rising = spendingDelta > 0
        insights.append(AnalysisInsight(
            id: "momentum",
            tone: rising ? .warning : .positive,
            title: L10n.text(rising ? "Spending is up" : "Spending is down"),
            detail: String(
                format: L10n.text(
                    rising
                        ? "You spent %1$@ more than the previous %2$lld days, %3$@ in total."
                        : "You spent %1$@ less than the previous %2$lld days, %3$@ in total."
                ),
                analysisFormattedPercent(abs(spendingDelta), signed: false),
                totals.dayCount,
                formatAmount(totals.spendingBaseMinor)
            )
        ))
    }

    let expenses = analysisCategoryNodes(analysis, kind: .expense)
    let movers = expenses
        .filter { $0.comparisonMinor > 0 }
        .compactMap { node -> (node: AnalysisCategoryNode, ratio: Double)? in
            guard let ratio = analysisDeltaRatio(
                current: node.amountMinor,
                previous: node.comparisonMinor
            ), abs(ratio) >= 0.15 else { return nil }
            return (node, ratio)
        }
        .sorted { abs($0.ratio * Double($0.node.amountMinor)) > abs($1.ratio * Double($1.node.amountMinor)) }
    if let mover = movers.first {
        let rising = mover.ratio > 0
        insights.append(AnalysisInsight(
            id: "mover",
            tone: rising ? .warning : .positive,
            title: String(
                format: L10n.text(rising ? "%@ grew the most" : "%@ fell the most"),
                formatCategory(mover.node.category)
            ),
            detail: String(
                format: L10n.text("%1$@ this period against %2$@ in the previous one."),
                formatAmount(mover.node.amountMinor),
                formatAmount(mover.node.comparisonMinor)
            )
        ))
    }

    if let leader = expenses.first, leader.share > 0 {
        insights.append(AnalysisInsight(
            id: "concentration",
            tone: leader.share >= 0.4 ? .warning : .neutral,
            title: String(
                format: L10n.text("%@ leads your spending"),
                formatCategory(leader.category)
            ),
            detail: String(
                format: L10n.text("%1$@ of everything you spent, across %2$@."),
                analysisFormattedPercent(leader.share, signed: false),
                analysisTransactionCount(leader.transactionCount)
            )
        ))
    }

    let busiest = analysisWeekdayReadings(analysis)
        .filter { $0.spendingMinor > 0 }
        .max { $0.spendingMinor < $1.spendingMinor }
    if let busiest {
        insights.append(AnalysisInsight(
            id: "rhythm",
            tone: .neutral,
            title: String(
                format: L10n.text("%@ is your heaviest day"),
                formatWeekday(busiest.weekday)
            ),
            detail: String(
                format: L10n.text("%1$@ spent on %2$@s in this period."),
                formatAmount(busiest.spendingMinor),
                formatWeekday(busiest.weekday)
            )
        ))
    }

    if let peak = analysis.series.max(by: { $0.spendingBaseMinor < $1.spendingBaseMinor }),
       peak.spendingBaseMinor > 0 {
        insights.append(AnalysisInsight(
            id: "peak",
            tone: .neutral,
            title: String(format: L10n.text("%@ was the peak"), formatBucket(peak.startDate)),
            detail: String(
                format: L10n.text("%1$@ across %2$@."),
                formatAmount(peak.spendingBaseMinor),
                analysisTransactionCount(peak.transactionCount)
            )
        ))
    }

    if let rate = analysisSavingsRate(
        income: totals.incomeBaseMinor,
        spending: totals.spendingBaseMinor
    ) {
        insights.append(AnalysisInsight(
            id: "savings",
            tone: rate >= 0.2 ? .positive : rate >= 0 ? .neutral : .danger,
            title: rate >= 0
                ? String(
                    format: L10n.text("You kept %@ of income"),
                    analysisFormattedPercent(rate, signed: false)
                )
                : L10n.text("You spent more than you earned"),
            detail: String(
                format: L10n.text("%1$@ earned against %2$@ spent."),
                formatAmount(totals.incomeBaseMinor),
                formatAmount(totals.spendingBaseMinor)
            )
        ))
    }

    return insights
}

// MARK: - Formatting

/// Analysis dates are calendar days, so every label is rendered in the same fixed zone they
/// were parsed in. Formatting them in the device's zone would move a boundary date by a day.
private func analysisFormatted(_ date: Date, _ style: Date.FormatStyle) -> String {
    var zoned = style
    zoned.timeZone = TimeZone(secondsFromGMT: 0) ?? .current
    return date.formatted(zoned)
}

/// A bucket reads by its width: a day needs its date, a month only needs its name.
func analysisBucketLabel(_ startDate: String, granularity: BudgetAnalysisGranularity) -> String {
    guard let date = analysisDate(from: startDate) else { return startDate }
    switch granularity {
    case .month:
        // A two-digit year here would read as a day: "Aug 26" for August 2026.
        return analysisFormatted(date, .dateTime.year(.defaultDigits).month(.abbreviated))
    case .day, .week:
        return analysisFormatted(date, .dateTime.month(.abbreviated).day())
    }
}

/// 2024-01-01 was a Monday, which turns an ISO weekday into a date the formatter can name.
func analysisWeekdayName(_ weekday: Int, abbreviated: Bool = false) -> String {
    guard let date = analysisCalendar.date(
        from: DateComponents(year: 2024, month: 1, day: weekday)
    ) else { return "" }
    return analysisFormatted(date, .dateTime.weekday(abbreviated ? .abbreviated : .wide))
}
