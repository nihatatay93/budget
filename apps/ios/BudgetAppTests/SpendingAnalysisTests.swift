import Foundation
import Testing
@testable import Budget

private let parentID = "0198b7ae-5e93-72da-b7aa-cd015d4bb77a"
private let childID = "0198b7ae-5e93-72da-b7aa-cd015d4bb77b"
private let otherID = "0198b7ae-5e93-72da-b7aa-cd015d4bb77c"

private func analysisCategory(
    id: String,
    parentID: String? = nil,
    name: String,
    kind: BudgetCategoryKind = .expense,
    rolledUp: Int64,
    comparisonRolledUp: Int64 = 0,
    direct: Int64 = 0,
    transactionCount: Int64 = 0,
    rolledUpTransactionCount: Int64 = 0,
    largest: Int64 = 0
) -> BudgetAnalysisCategory {
    BudgetAnalysisCategory(
        id: id,
        parentID: parentID,
        name: name,
        kind: kind,
        systemKey: nil,
        predefinedKey: nil,
        iconType: "system",
        iconValue: "ellipsis",
        colorKey: "slate",
        archivedAt: nil,
        directBaseMinor: direct,
        rolledUpBaseMinor: rolledUp,
        comparisonDirectBaseMinor: 0,
        comparisonRolledUpBaseMinor: comparisonRolledUp,
        transactionCount: transactionCount,
        rolledUpTransactionCount: rolledUpTransactionCount,
        largestBaseMinor: largest,
        firstDate: nil,
        lastDate: nil
    )
}

private func analysisFixture(
    fromDate: String = "2026-08-01",
    toDate: String = "2026-08-14",
    categorySeries: [BudgetAnalysisCategoryPoint]? = nil,
    days: [BudgetAnalysisDay]? = nil,
    weekdays: [BudgetAnalysisWeekday]? = nil,
    totals: BudgetAnalysisTotals? = nil
) -> BudgetSpendingAnalysis {
    BudgetSpendingAnalysis(
        period: BudgetAnalysisPeriod(
            fromDate: fromDate,
            toDate: toDate,
            comparisonFromDate: "2026-07-18",
            comparisonToDate: "2026-07-31",
            granularity: .week,
            timezone: "Europe/Istanbul",
            baseCurrency: .turkishLira
        ),
        totals: totals ?? BudgetAnalysisTotals(
            incomeBaseMinor: 500_000,
            spendingBaseMinor: 300_000,
            netBaseMinor: 200_000,
            comparisonIncomeBaseMinor: 500_000,
            comparisonSpendingBaseMinor: 200_000,
            comparisonNetBaseMinor: 300_000,
            transactionCount: 20,
            spendingTransactionCount: 16,
            largestSpendingBaseMinor: 90_000,
            spendingDayCount: 7,
            dayCount: 14
        ),
        series: [
            BudgetAnalysisBucket(
                startDate: "2026-08-01",
                endDate: "2026-08-02",
                incomeBaseMinor: 500_000,
                spendingBaseMinor: 100_000,
                netBaseMinor: 400_000,
                transactionCount: 6
            ),
            BudgetAnalysisBucket(
                startDate: "2026-08-03",
                endDate: "2026-08-09",
                incomeBaseMinor: 0,
                spendingBaseMinor: 180_000,
                netBaseMinor: -180_000,
                transactionCount: 10
            ),
            BudgetAnalysisBucket(
                startDate: "2026-08-10",
                endDate: "2026-08-14",
                incomeBaseMinor: 0,
                spendingBaseMinor: 20_000,
                netBaseMinor: -20_000,
                transactionCount: 4
            ),
        ],
        categories: [
            analysisCategory(
                id: parentID, name: "Food", rolledUp: 240_000,
                comparisonRolledUp: 120_000, direct: 40_000,
                transactionCount: 4, rolledUpTransactionCount: 14, largest: 90_000
            ),
            analysisCategory(
                id: childID, parentID: parentID, name: "Restaurants", rolledUp: 200_000,
                comparisonRolledUp: 80_000, direct: 200_000,
                transactionCount: 10, rolledUpTransactionCount: 10
            ),
            analysisCategory(
                id: otherID, name: "Transport", rolledUp: 60_000,
                comparisonRolledUp: 80_000, direct: 60_000,
                transactionCount: 2, rolledUpTransactionCount: 2
            ),
        ],
        categorySeries: categorySeries ?? [
            BudgetAnalysisCategoryPoint(
                categoryID: parentID, startDate: "2026-08-03", baseMinor: 40_000
            ),
            BudgetAnalysisCategoryPoint(
                categoryID: childID, startDate: "2026-08-03", baseMinor: 140_000
            ),
            BudgetAnalysisCategoryPoint(
                categoryID: childID, startDate: "2026-08-10", baseMinor: 60_000
            ),
        ],
        weekdays: weekdays ?? [
            BudgetAnalysisWeekday(
                weekday: 1, incomeBaseMinor: 0, spendingBaseMinor: 40_000, transactionCount: 3
            ),
            BudgetAnalysisWeekday(
                weekday: 6, incomeBaseMinor: 0, spendingBaseMinor: 200_000, transactionCount: 8
            ),
        ],
        days: days ?? [
            BudgetAnalysisDay(
                date: "2026-08-01", incomeBaseMinor: 500_000,
                spendingBaseMinor: 100_000, transactionCount: 6
            ),
            BudgetAnalysisDay(
                date: "2026-08-08", incomeBaseMinor: 0,
                spendingBaseMinor: 200_000, transactionCount: 8
            ),
        ],
        payees: [],
        accounts: []
    )
}

@Suite("Spending analysis dates")
struct SpendingAnalysisDateTests {
    /// 22:30 UTC is already the next day in Istanbul, which is the day the server would use.
    @Test func readsTodayInTheWorkspaceTimezoneRatherThanTheDevices() throws {
        var utc = Calendar(identifier: .gregorian)
        utc.timeZone = try #require(TimeZone(secondsFromGMT: 0))
        let instant = try #require(utc.date(
            from: DateComponents(year: 2026, month: 8, day: 25, hour: 22, minute: 30)
        ))

        #expect(analysisWorkspaceToday(instant, timezone: "Europe/Istanbul") == "2026-08-26")
        #expect(analysisWorkspaceToday(instant, timezone: "America/Los_Angeles") == "2026-08-25")
    }

    @Test func numbersWeekdaysTheWayTheContractDoes() {
        #expect(analysisISOWeekday("2026-08-03") == 1)
        #expect(analysisISOWeekday("2026-08-09") == 7)
    }

    @Test func addsDaysAcrossMonthBoundaries() {
        #expect(analysisAddingDays("2026-08-31", 1) == "2026-09-01")
        #expect(analysisAddingDays("2026-03-01", -1) == "2026-02-28")
    }

    /// Presets must produce the same windows the web client asks for, or the same period
    /// would read differently on the two clients.
    @Test func anchorsPresetsOnTheWorkspaceDayWithAReadableBucketWidth() {
        let today = "2026-08-25"

        let thisMonth = AnalysisRangePreset.thisMonth.range(today: today)
        #expect(thisMonth.fromDate == "2026-08-01")
        #expect(thisMonth.toDate == today)
        #expect(thisMonth.granularity == .day)

        let lastMonth = AnalysisRangePreset.lastMonth.range(today: today)
        #expect(lastMonth.fromDate == "2026-07-01")
        #expect(lastMonth.toDate == "2026-07-31")

        let year = AnalysisRangePreset.last12Months.range(today: today)
        #expect(year.fromDate == "2025-09-01")
        #expect(year.granularity == .month)

        #expect(AnalysisRangePreset.yearToDate.range(today: today).fromDate == "2026-01-01")
    }

    /// February is where a naive "subtract 30 days per month" would drift.
    @Test func shiftsMonthsAcrossAShortFebruary() {
        let range = AnalysisRangePreset.lastMonth.range(today: "2026-03-15")

        #expect(range.fromDate == "2026-02-01")
        #expect(range.toDate == "2026-02-28")
    }
}

@Suite("Spending analysis readings")
struct SpendingAnalysisReadingTests {
    @Test func reportsNoRatioWhenThereIsNothingToCompareAgainst() {
        #expect(analysisDeltaRatio(current: 100, previous: 0) == nil)
        #expect(analysisDeltaRatio(current: 150, previous: 100) == 0.5)
        #expect(analysisDeltaRatio(current: 50, previous: 100) == -0.5)
    }

    @Test func declinesToReportASavingsRateWithoutIncome() {
        #expect(analysisSavingsRate(income: 0, spending: 5_000) == nil)
        #expect(analysisSavingsRate(income: 10_000, spending: 2_500) == 0.75)
    }

    @Test func averagesSpendingAcrossTheWholeWindowNotOnlyItsActiveDays() {
        #expect(analysisAveragePerDay(total: 300_000, dayCount: 14) == 21_429)
        #expect(analysisAveragePerDay(total: 300_000, dayCount: 0) == 0)
        #expect(analysisAveragePerTransaction(total: 300_000, transactionCount: 0) == 0)
    }

    /// "1 transactions" is the giveaway that a count was assembled without a plural rule.
    @Test func usesTheSingularFormForASingleTransaction() {
        #expect(analysisTransactionCount(1) == "1 transaction")
        #expect(analysisTransactionCount(0) == "0 transactions")
        #expect(analysisTransactionCount(14) == "14 transactions")
    }
}

@Suite("Spending analysis categories")
struct SpendingAnalysisCategoryTests {
    @Test func ranksTopLevelCategoriesByTheirRolledUpTotals() {
        let nodes = analysisCategoryNodes(analysisFixture(), kind: .expense)

        #expect(nodes.map(\.id) == [parentID, otherID])
        #expect(nodes[0].amountMinor == 240_000)
        #expect(nodes[0].transactionCount == 14)
        #expect(nodes[0].children.map(\.id) == [childID])
    }

    @Test func computesSharesAgainstTheRankedTotalSoTheySumToOne() {
        let nodes = analysisCategoryNodes(analysisFixture(), kind: .expense)
        let total = nodes.reduce(0.0) { $0 + $1.share }

        #expect(abs(total - 1) < 0.0001)
        #expect(abs(nodes[0].share - 240_000.0 / 300_000.0) < 0.0001)
    }

    @Test func omitsCategoriesWithNoActivityInTheWindow() {
        #expect(analysisCategoryNodes(analysisFixture(), kind: .income).isEmpty)
    }

    @Test func includesASubtreeSoAParentTrendMatchesItsRolledUpTotal() {
        let analysis = analysisFixture()
        let ids = analysisCategorySubtreeIDs(analysis, categoryID: parentID)
        #expect(ids == [parentID, childID])

        let values = analysisCategorySeries(analysis, categoryIDs: ids)
        #expect(values == [0, 180_000, 60_000])
        #expect(values.reduce(0, +) == 240_000)
    }

    /// Month and week anchors precede a window that starts mid-bucket. Dropping those points
    /// would silently understate the first bucket.
    @Test func foldsActivityAnchoredBeforeTheWindowIntoTheOpeningBucket() {
        let analysis = analysisFixture(categorySeries: [
            BudgetAnalysisCategoryPoint(
                categoryID: childID, startDate: "2026-07-27", baseMinor: 25_000
            ),
            BudgetAnalysisCategoryPoint(
                categoryID: childID, startDate: "2026-08-03", baseMinor: 140_000
            ),
        ])

        #expect(analysisCategorySeries(analysis, categoryIDs: [childID]) == [25_000, 140_000, 0])
    }
}

@Suite("Spending analysis rhythm")
struct SpendingAnalysisRhythmTests {
    @Test func returnsAllSevenWeekdaysSoQuietOnesStayVisible() {
        let readings = analysisWeekdayReadings(analysisFixture())

        #expect(readings.count == 7)
        #expect(readings.map(\.weekday) == [1, 2, 3, 4, 5, 6, 7])
        #expect(readings[5].share == 1)
        #expect(abs(readings[0].share - 0.2) < 0.0001)
        #expect(readings[1].spendingMinor == 0)
    }

    /// 2026-08-01 is a Saturday, so the first week carries five leading gaps and the grid
    /// stays aligned to weekdays instead of sliding by the window's start day.
    @Test func alignsTheCalendarToMondayAndLeavesDaysOutsideTheWindowEmpty() {
        let weeks = analysisCalendarWeeks(analysisFixture())

        #expect(weeks.first?.cells.prefix(5).allSatisfy { $0 == nil } == true)
        #expect(weeks.first?.cells[5]?.date == "2026-08-01")
        // The window ends on Friday 2026-08-14, so the final weekend has no days.
        #expect(weeks.last?.cells.suffix(2).allSatisfy { $0 == nil } == true)
        #expect(weeks.last?.cells[4]?.date == "2026-08-14")

        let dates = weeks.flatMap { $0.cells.compactMap { $0?.date } }
        #expect(dates.count == 14)
        #expect(dates.first == "2026-08-01")
        #expect(dates.last == "2026-08-14")
    }

    @Test func scalesIntensityAgainstTheHeaviestDayInTheWindow() {
        let cells = analysisCalendarWeeks(analysisFixture()).flatMap { $0.cells.compactMap { $0 } }

        #expect(cells.first { $0.date == "2026-08-08" }?.intensity == 1)
        #expect(abs((cells.first { $0.date == "2026-08-01" }?.intensity ?? 0) - 0.5) < 0.0001)
        #expect(cells.first { $0.date == "2026-08-02" }?.intensity == 0)
    }
}

@Suite("Spending analysis insights")
struct SpendingAnalysisInsightTests {
    private func insights(_ analysis: BudgetSpendingAnalysis) -> [AnalysisInsight] {
        analysisInsights(
            analysis,
            formatAmount: { "\($0 / 100) TRY" },
            formatCategory: { $0.name },
            formatWeekday: { "day \($0)" },
            formatBucket: { $0 }
        )
    }

    @Test func statesTheMovementsTheWindowActuallyProves() {
        let byID = Dictionary(
            insights(analysisFixture()).map { ($0.id, $0) },
            uniquingKeysWith: { first, _ in first }
        )

        #expect(byID["momentum"]?.tone == .warning)
        #expect(byID["momentum"]?.detail.contains("50") == true)
        #expect(byID["mover"]?.title.contains("Food") == true)
        #expect(byID["concentration"]?.title.contains("Food") == true)
        #expect(byID["rhythm"]?.title.contains("day 6") == true)
        #expect(byID["peak"]?.title.contains("2026-08-03") == true)
        #expect(byID["savings"]?.tone == .positive)
    }

    @Test func staysQuietAboutMomentumWhenTheChangeIsNegligible() {
        let totals = BudgetAnalysisTotals(
            incomeBaseMinor: 500_000,
            spendingBaseMinor: 300_000,
            netBaseMinor: 200_000,
            comparisonIncomeBaseMinor: 500_000,
            comparisonSpendingBaseMinor: 300_000,
            comparisonNetBaseMinor: 200_000,
            transactionCount: 20,
            spendingTransactionCount: 16,
            largestSpendingBaseMinor: 90_000,
            spendingDayCount: 7,
            dayCount: 14
        )

        let ids = insights(analysisFixture(totals: totals)).map(\.id)
        #expect(!ids.contains("momentum"))
    }

    @Test func reportsOverspendingRatherThanANegativeSavingsRate() {
        let totals = BudgetAnalysisTotals(
            incomeBaseMinor: 500_000,
            spendingBaseMinor: 600_000,
            netBaseMinor: -100_000,
            comparisonIncomeBaseMinor: 500_000,
            comparisonSpendingBaseMinor: 200_000,
            comparisonNetBaseMinor: 300_000,
            transactionCount: 20,
            spendingTransactionCount: 16,
            largestSpendingBaseMinor: 90_000,
            spendingDayCount: 7,
            dayCount: 14
        )

        let savings = insights(analysisFixture(totals: totals)).first { $0.id == "savings" }
        #expect(savings?.tone == .danger)
        #expect(savings?.title == "You spent more than you earned")
    }
}
