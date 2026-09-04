import Charts
import SwiftUI

/// The native counterpart of the web client's Analysis destination. It answers the same three
/// questions from the same contract — where money went, when it went, and how the trend moved —
/// using Swift Charts rather than the web client's hand-drawn SVG, because the system framework
/// already carries the platform's rendering, animation, and VoiceOver chart behaviour.
struct SpendingAnalysisView: View {
    let workspace: BudgetWorkspace
    @ObservedObject var model: AppModel

    @State private var preset = AnalysisRangePreset.last6Months
    @State private var granularityOverride: BudgetAnalysisGranularity?

    private var today: String {
        analysisWorkspaceToday(timezone: workspace.timezone)
    }

    /// The preset supplies a bucket width that suits its span; an explicit choice replaces it.
    private var range: BudgetAnalysisRange {
        let base = preset.range(today: today)
        guard let granularityOverride else { return base }
        return BudgetAnalysisRange(
            fromDate: base.fromDate,
            toDate: base.toDate,
            granularity: granularityOverride
        )
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: BudgetTheme.Space.section) {
                controlsCard

                if let analysis = model.spendingAnalysis {
                    if analysis.totals.transactionCount == 0 {
                        BudgetCard {
                            BudgetMessage(
                                title: "Nothing to analyze yet",
                                systemImage: "chart.xyaxis.line",
                                message: "Once this period has posted activity, its trend, categories, and rhythm appear here."
                            )
                        }
                    } else {
                        AnalysisContent(analysis: analysis, workspace: workspace, model: model)
                    }
                } else if model.isLoadingAnalysis {
                    BudgetCard {
                        BudgetLoading("Loading spending analysis…")
                    }
                } else {
                    BudgetCard {
                        BudgetMessage(
                            title: "Analysis unavailable",
                            systemImage: "chart.xyaxis.line",
                            message: .resolved(
                                model.analysisErrorMessage ?? L10n.text("Pull to refresh and try again.")
                            ),
                            action: ("Try again", reload)
                        )
                    }
                }
            }
            .padding(.horizontal, BudgetTheme.Space.screen)
            .padding(.top, 8)
            .padding(.bottom, 32)
        }
        .budgetScreen()
        .navigationTitle("Analysis")
        .navigationBarTitleDisplayMode(.inline)
        .task(id: workspace.id) { await load() }
        .refreshable { await load() }
        .onChange(of: preset) { _, _ in reload() }
        .onChange(of: granularityOverride) { _, _ in reload() }
        .overlay(alignment: .top) {
            if model.isLoadingAnalysis, model.spendingAnalysis != nil {
                BudgetLoading("Updating analysis…")
                    .padding(16)
                    .budgetSurface(radius: BudgetTheme.Radius.small)
                    .padding(.top, 8)
            }
        }
    }

    private var controlsCard: some View {
        BudgetCard {
            VStack(alignment: .leading, spacing: 14) {
                VStack(alignment: .leading, spacing: 5) {
                    BudgetEyebrow("Posted activity only", color: BudgetTheme.forest)
                    Text("Where your money went")
                        .font(.title3.weight(.semibold))
                        .foregroundStyle(BudgetTheme.primaryText)
                    Text("Transfers between your own accounts are never counted as spending.")
                        .font(.footnote)
                        .foregroundStyle(BudgetTheme.secondaryText)
                }

                BudgetHairline()

                VStack(alignment: .leading, spacing: 8) {
                    BudgetEyebrow("Period")
                    // A wheel rather than a segmented control: six presets never fit legibly
                    // across a phone, and truncated labels would defeat the point of naming them.
                    Picker("Period", selection: $preset) {
                        ForEach(AnalysisRangePreset.allCases) { option in
                            Text(option.title).tag(option)
                        }
                    }
                    .pickerStyle(.menu)
                    .tint(BudgetTheme.forest)
                    .accessibilityLabel(L10n.text("Analysis period"))
                }

                VStack(alignment: .leading, spacing: 8) {
                    BudgetEyebrow("Time bucket")
                    Picker("Time bucket", selection: $granularityOverride) {
                        Text("Auto").tag(BudgetAnalysisGranularity?.none)
                        ForEach(BudgetAnalysisGranularity.allCases) { option in
                            Text(option.title).tag(BudgetAnalysisGranularity?.some(option))
                        }
                    }
                    .pickerStyle(.segmented)
                    .accessibilityLabel(L10n.text("Time bucket"))
                }

                if let analysis = model.spendingAnalysis {
                    Text(
                        "\(budgetDisplayDate(analysis.period.fromDate)) – "
                        + "\(budgetDisplayDate(analysis.period.toDate)) · \(analysis.period.timezone)"
                    )
                    .font(.caption)
                    .foregroundStyle(BudgetTheme.tertiaryText)
                    .accessibilityLabel(
                        String(
                            format: L10n.text("Analysis period from %1$@ through %2$@, %3$@"),
                            analysis.period.fromDate,
                            analysis.period.toDate,
                            analysis.period.timezone
                        )
                    )
                }
            }
        }
    }

    private func reload() {
        Task { await load() }
    }

    private func load() async {
        await model.loadSpendingAnalysis(workspaceID: workspace.id, range: range)
    }
}

// MARK: - Content

private struct AnalysisContent: View {
    let analysis: BudgetSpendingAnalysis
    let workspace: BudgetWorkspace
    @ObservedObject var model: AppModel

    private var currency: BudgetCurrency { analysis.period.baseCurrency }

    var body: some View {
        Group {
            AnalysisTotalsSection(analysis: analysis)

            BudgetSection(
                "Spending trend",
                caption: .resolved(String(
                    format: L10n.text("Each point is one %@ of posted activity."),
                    analysis.period.granularity.noun
                ))
            ) {
                BudgetCard {
                    AnalysisTrendChart(analysis: analysis)
                }
            }

            let insights = analysisInsights(
                analysis,
                formatAmount: { currency.formatted(minorUnits: $0) },
                formatCategory: {
                    L10n.categoryName(
                        name: $0.name,
                        kind: $0.kind,
                        predefinedKey: $0.predefinedKey,
                        systemKey: $0.systemKey
                    )
                },
                formatWeekday: { analysisWeekdayName($0) },
                formatBucket: { analysisBucketLabel($0, granularity: analysis.period.granularity) }
            )
            if !insights.isEmpty {
                BudgetSection("What stands out") {
                    VStack(spacing: 10) {
                        ForEach(insights) { insight in
                            AnalysisInsightCard(insight: insight)
                        }
                    }
                }
            }

            AnalysisCategorySection(
                title: "Spending by category",
                analysis: analysis,
                kind: .expense
            )

            AnalysisRhythmSection(analysis: analysis)

            AnalysisPayeeSection(analysis: analysis)

            AnalysisAccountSection(analysis: analysis)

            let income = analysisCategoryNodes(analysis, kind: .income)
            if !income.isEmpty {
                AnalysisCategorySection(
                    title: "Income by category",
                    analysis: analysis,
                    kind: .income
                )
            }

            Text(
                String(
                    format: L10n.text("Comparison window: %1$@ – %2$@"),
                    budgetDisplayDate(analysis.period.comparisonFromDate),
                    budgetDisplayDate(analysis.period.comparisonToDate)
                )
            )
            .font(.caption)
            .foregroundStyle(BudgetTheme.tertiaryText)
            .frame(maxWidth: .infinity, alignment: .trailing)
        }
    }
}

// MARK: - Totals

private struct AnalysisTotalsSection: View {
    let analysis: BudgetSpendingAnalysis

    private var currency: BudgetCurrency { analysis.period.baseCurrency }
    private var totals: BudgetAnalysisTotals { analysis.totals }

    var body: some View {
        VStack(spacing: 10) {
            BudgetCard {
                VStack(alignment: .leading, spacing: 12) {
                    HStack(alignment: .firstTextBaseline) {
                        BudgetEyebrow("Total spent", color: BudgetTheme.spend)
                        Spacer(minLength: 12)
                        BudgetChip(
                            text: .resolved(currency.rawValue),
                            color: BudgetTheme.secondaryText
                        )
                    }
                    Text(currency.formatted(minorUnits: totals.spendingBaseMinor))
                        .font(.budgetHero)
                        .foregroundStyle(BudgetTheme.primaryText)
                        .minimumScaleFactor(0.5)
                        .lineLimit(1)
                    AnalysisDeltaLabel(
                        current: totals.spendingBaseMinor,
                        previous: totals.comparisonSpendingBaseMinor,
                        currency: currency,
                        // Spending going up is not the same kind of news as income going up.
                        favourableWhenFalling: true
                    )
                }
            }

            BudgetCard {
                VStack(spacing: 14) {
                    AnalysisMetricRow(
                        title: "Income",
                        value: currency.formatted(minorUnits: totals.incomeBaseMinor),
                        detail: .resolved(analysisTransactionCount(totals.transactionCount)),
                        current: totals.incomeBaseMinor,
                        previous: totals.comparisonIncomeBaseMinor,
                        currency: currency,
                        favourableWhenFalling: false
                    )
                    BudgetHairline()
                    AnalysisMetricRow(
                        title: "Net",
                        value: currency.formatted(minorUnits: totals.netBaseMinor),
                        detail: savingsDetail,
                        current: totals.netBaseMinor,
                        previous: totals.comparisonNetBaseMinor,
                        currency: currency,
                        favourableWhenFalling: false
                    )
                    BudgetHairline()
                    HStack(spacing: 12) {
                        BudgetStat(
                            title: "Average per day",
                            value: currency.formatted(
                                minorUnits: analysisAveragePerDay(
                                    total: totals.spendingBaseMinor,
                                    dayCount: totals.dayCount
                                )
                            )
                        )
                        BudgetStat(
                            title: "Average per transaction",
                            value: currency.formatted(
                                minorUnits: analysisAveragePerTransaction(
                                    total: totals.spendingBaseMinor,
                                    transactionCount: totals.spendingTransactionCount
                                )
                            )
                        )
                        BudgetStat(
                            title: "Largest charge",
                            value: currency.formatted(minorUnits: totals.largestSpendingBaseMinor)
                        )
                    }
                    Text(
                        String(
                            format: L10n.text("%1$lld of %2$lld days had spending"),
                            totals.spendingDayCount,
                            totals.dayCount
                        )
                    )
                    .font(.caption)
                    .foregroundStyle(BudgetTheme.tertiaryText)
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
            }
        }
    }

    private var savingsDetail: LocalizedStringKey {
        guard let rate = analysisSavingsRate(
            income: totals.incomeBaseMinor,
            spending: totals.spendingBaseMinor
        ) else {
            return "No income in this period"
        }
        return .resolved(String(
            format: L10n.text("%@ of income kept"),
            analysisFormattedPercent(rate, signed: false)
        ))
    }
}

private struct AnalysisMetricRow: View {
    let title: LocalizedStringKey
    let value: String
    let detail: LocalizedStringKey
    let current: Int64
    let previous: Int64
    let currency: BudgetCurrency
    let favourableWhenFalling: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(alignment: .firstTextBaseline) {
                BudgetEyebrow(title)
                Spacer(minLength: 12)
                Text(value)
                    .font(.budgetAmountLarge)
                    .foregroundStyle(BudgetTheme.primaryText)
                    .minimumScaleFactor(0.6)
                    .lineLimit(1)
            }
            HStack(spacing: 10) {
                AnalysisDeltaLabel(
                    current: current,
                    previous: previous,
                    currency: currency,
                    favourableWhenFalling: favourableWhenFalling
                )
                Spacer(minLength: 8)
                Text(detail)
                    .font(.caption)
                    .foregroundStyle(BudgetTheme.tertiaryText)
                    .multilineTextAlignment(.trailing)
            }
        }
    }
}

/// Period-over-period movement. The tone is chosen by what the figure means, not by its sign.
private struct AnalysisDeltaLabel: View {
    let current: Int64
    let previous: Int64
    let currency: BudgetCurrency
    let favourableWhenFalling: Bool

    var body: some View {
        if let ratio = analysisDeltaRatio(current: current, previous: previous) {
            let favourable = favourableWhenFalling ? ratio <= 0 : ratio >= 0
            let color: Color = abs(ratio) < 0.01
                ? BudgetTheme.secondaryText
                : favourable ? BudgetTheme.positive : BudgetTheme.spend
            HStack(spacing: 6) {
                BudgetChip(
                    text: .resolved(analysisFormattedPercent(ratio)),
                    systemImage: ratio > 0 ? "arrow.up.right" : ratio < 0 ? "arrow.down.right" : "equal",
                    color: color
                )
                Text(
                    String(
                        format: L10n.text("vs %@ before"),
                        currency.formatted(minorUnits: previous)
                    )
                )
                .font(.caption2)
                .foregroundStyle(BudgetTheme.tertiaryText)
                .lineLimit(1)
                .minimumScaleFactor(0.8)
            }
            .accessibilityElement(children: .combine)
        } else {
            Text("No comparable activity before this period")
                .font(.caption2)
                .foregroundStyle(BudgetTheme.tertiaryText)
        }
    }
}

// MARK: - Trend

private struct AnalysisTrendChart: View {
    let analysis: BudgetSpendingAnalysis

    @State private var showsIncome = true

    private var currency: BudgetCurrency { analysis.period.baseCurrency }

    private struct Point: Identifiable {
        let id: String
        let label: String
        let date: Date
        let series: String
        let amount: Double
    }

    private var points: [Point] {
        analysis.series.flatMap { bucket -> [Point] in
            guard let date = analysisDate(from: bucket.startDate) else { return [] }
            let label = analysisBucketLabel(
                bucket.startDate,
                granularity: analysis.period.granularity
            )
            var values = [(L10n.text("Spending"), bucket.spendingBaseMinor)]
            if showsIncome {
                values.append((L10n.text("Income"), bucket.incomeBaseMinor))
            }
            return values.map { name, amount in
                Point(
                    id: "\(bucket.startDate)-\(name)",
                    label: label,
                    date: date,
                    series: name,
                    amount: Double(amount) / 100
                )
            }
        }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Toggle("Show income", isOn: $showsIncome)
                .font(.footnote.weight(.medium))
                .tint(BudgetTheme.forest)

            Chart(points) { point in
                if point.series == L10n.text("Spending") {
                    AreaMark(
                        x: .value(L10n.text("Period"), point.date),
                        y: .value(L10n.text("Amount"), point.amount)
                    )
                    .foregroundStyle(
                        LinearGradient(
                            colors: [
                                BudgetTheme.spend.opacity(0.32),
                                BudgetTheme.spend.opacity(0.02),
                            ],
                            startPoint: .top,
                            endPoint: .bottom
                        )
                    )
                    .interpolationMethod(.monotone)
                }
                LineMark(
                    x: .value(L10n.text("Period"), point.date),
                    y: .value(L10n.text("Amount"), point.amount),
                    series: .value(L10n.text("Series"), point.series)
                )
                .foregroundStyle(by: .value(L10n.text("Series"), point.series))
                .interpolationMethod(.monotone)
                .lineStyle(StrokeStyle(lineWidth: 2.5, lineCap: .round, lineJoin: .round))
                .accessibilityLabel("\(point.label), \(point.series)")
                .accessibilityValue(
                    currency.formatted(minorUnits: Int64((point.amount * 100).rounded()))
                )
            }
            .chartForegroundStyleScale([
                L10n.text("Spending"): BudgetTheme.spend,
                L10n.text("Income"): BudgetTheme.positive,
            ])
            .chartLegend(position: .bottom, spacing: 10)
            .chartYAxis {
                AxisMarks(position: .leading) { value in
                    AxisGridLine().foregroundStyle(BudgetTheme.separator)
                    AxisValueLabel {
                        if let amount = value.as(Double.self) {
                            Text(compactAmount(amount))
                                .font(.caption2)
                                .foregroundStyle(BudgetTheme.tertiaryText)
                        }
                    }
                }
            }
            .chartXAxis {
                AxisMarks(values: .automatic(desiredCount: 4)) { value in
                    AxisValueLabel {
                        if let date = value.as(Date.self) {
                            Text(analysisBucketLabel(
                                analysisDateString(date),
                                granularity: analysis.period.granularity
                            ))
                            .font(.caption2)
                            .foregroundStyle(BudgetTheme.tertiaryText)
                        }
                    }
                }
            }
            .frame(height: 220)
            .accessibilityLabel(
                String(
                    format: L10n.text("Spending and income by %@"),
                    analysis.period.granularity.noun
                )
            )
        }
    }

    /// Axis figures have to fit; the exact amount stays available in the chart's audio graph
    /// and in the totals above.
    private func compactAmount(_ amount: Double) -> String {
        amount.formatted(
            .currency(code: currency.rawValue)
                .notation(.compactName)
                .precision(.fractionLength(0...1))
        )
    }
}

// MARK: - Insights

private struct AnalysisInsightCard: View {
    let insight: AnalysisInsight

    private var color: Color {
        switch insight.tone {
        case .neutral: BudgetTheme.secondaryText
        case .positive: BudgetTheme.positive
        case .warning: BudgetTheme.spend
        case .danger: BudgetTheme.over
        }
    }

    var body: some View {
        HStack(alignment: .top, spacing: 12) {
            Capsule()
                .fill(color)
                .frame(width: 3)
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 4) {
                Text(insight.title)
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(BudgetTheme.primaryText)
                Text(insight.detail)
                    .font(.footnote)
                    .foregroundStyle(BudgetTheme.secondaryText)
                    .fixedSize(horizontal: false, vertical: true)
            }
            Spacer(minLength: 0)
        }
        .padding(BudgetTheme.Space.card)
        .budgetSurface(radius: BudgetTheme.Radius.small)
        .accessibilityElement(children: .combine)
    }
}

// MARK: - Categories

private struct AnalysisCategorySection: View {
    let title: LocalizedStringKey
    let analysis: BudgetSpendingAnalysis
    let kind: BudgetCategoryKind

    @State private var expandedCategoryID: String?

    private var nodes: [AnalysisCategoryNode] {
        analysisCategoryNodes(analysis, kind: kind)
    }

    var body: some View {
        BudgetSection(title) {
            if nodes.isEmpty {
                BudgetCard {
                    BudgetMessage(
                        title: "No category activity",
                        systemImage: "tag",
                        message: "Nothing was allocated to a category in this period."
                    )
                }
            } else {
                VStack(spacing: 0) {
                    ForEach(Array(nodes.enumerated()), id: \.element.id) { index, node in
                        AnalysisCategoryRow(
                            node: node,
                            rank: index + 1,
                            analysis: analysis,
                            isExpanded: expandedCategoryID == node.id,
                            onToggle: {
                                expandedCategoryID = expandedCategoryID == node.id ? nil : node.id
                            }
                        )
                        .padding(.horizontal, BudgetTheme.Space.card)
                        .padding(.vertical, 13)
                        if index < nodes.count - 1 {
                            BudgetHairline(leading: BudgetTheme.Space.card)
                        }
                    }
                }
                .budgetSurface()
            }
        }
    }
}

private struct AnalysisCategoryRow: View {
    let node: AnalysisCategoryNode
    let rank: Int
    let analysis: BudgetSpendingAnalysis
    let isExpanded: Bool
    let onToggle: () -> Void

    private var currency: BudgetCurrency { analysis.period.baseCurrency }
    private var palette: CategoryPaletteColor {
        CategoryPresentation.color(for: node.category.colorKey)
    }

    private var name: String {
        L10n.categoryName(
            name: node.category.name,
            kind: node.category.kind,
            predefinedKey: node.category.predefinedKey,
            systemKey: node.category.systemKey
        )
    }

    private var trend: [Int64] {
        analysisCategorySeries(
            analysis,
            categoryIDs: analysisCategorySubtreeIDs(analysis, categoryID: node.category.id)
        )
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(spacing: 12) {
                Text("\(rank)")
                    .font(.caption2.weight(.bold).monospacedDigit())
                    .foregroundStyle(BudgetTheme.tertiaryText)
                    .frame(width: 14, alignment: .leading)
                    .accessibilityHidden(true)
                CategoryAppearanceBadge(
                    iconType: node.category.iconType,
                    iconValue: node.category.iconValue,
                    colorKey: node.category.colorKey,
                    size: 32
                )
                VStack(alignment: .leading, spacing: 2) {
                    Text(name)
                        .font(.subheadline.weight(.medium))
                        .foregroundStyle(BudgetTheme.primaryText)
                    Text(detailLine)
                        .font(.caption2)
                        .foregroundStyle(BudgetTheme.tertiaryText)
                        .lineLimit(1)
                }
                Spacer(minLength: 8)
                VStack(alignment: .trailing, spacing: 2) {
                    Text(currency.formatted(minorUnits: node.amountMinor))
                        .font(.budgetAmount)
                        .foregroundStyle(BudgetTheme.primaryText)
                        .minimumScaleFactor(0.7)
                        .lineLimit(1)
                    Text(analysisFormattedPercent(node.share, signed: false))
                        .font(.caption2.monospacedDigit())
                        .foregroundStyle(BudgetTheme.tertiaryText)
                }
            }

            HStack(spacing: 10) {
                BudgetMeter(progress: node.share, tint: palette.accent)
                AnalysisSparkline(values: trend, color: palette.accent)
                    .frame(width: 54, height: 18)
                Text(deltaText)
                    .font(.caption2.weight(.semibold).monospacedDigit())
                    .foregroundStyle(deltaColor)
                    .frame(minWidth: 44, alignment: .trailing)
            }

            if !node.children.isEmpty {
                Button(action: onToggle) {
                    HStack(spacing: 4) {
                        Text(isExpanded
                            ? LocalizedStringKey("Hide subcategories")
                            : .resolved(String(
                                format: L10n.text("Show %lld subcategories"),
                                Int64(node.children.count)
                            )))
                        Image(systemName: isExpanded ? "chevron.up" : "chevron.down")
                            .font(.caption2.weight(.semibold))
                    }
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(BudgetTheme.forest)
                }
                .buttonStyle(.plain)
                .accessibilityAddTraits(.isButton)

                if isExpanded {
                    VStack(spacing: 6) {
                        ForEach(node.children) { child in
                            HStack(spacing: 8) {
                                CategoryNameLabel(
                                    name: child.name,
                                    kind: child.kind,
                                    predefinedKey: child.predefinedKey,
                                    systemKey: child.systemKey,
                                    iconType: child.iconType,
                                    iconValue: child.iconValue,
                                    colorKey: child.colorKey,
                                    iconSize: 22
                                )
                                .font(.caption)
                                .foregroundStyle(BudgetTheme.secondaryText)
                                Spacer(minLength: 8)
                                Text(currency.formatted(minorUnits: child.rolledUpBaseMinor))
                                    .font(.budgetAmountSmall)
                                    .foregroundStyle(BudgetTheme.secondaryText)
                            }
                        }
                    }
                    .padding(.leading, 26)
                }
            }
        }
        .accessibilityElement(children: node.children.isEmpty ? .combine : .contain)
    }

    private var detailLine: String {
        var parts = [analysisTransactionCount(node.transactionCount)]
        if node.category.largestBaseMinor > 0 {
            parts.append(String(
                format: L10n.text("largest %@"),
                currency.formatted(minorUnits: node.category.largestBaseMinor)
            ))
        }
        return parts.joined(separator: " · ")
    }

    private var deltaText: String {
        guard let ratio = analysisDeltaRatio(
            current: node.amountMinor,
            previous: node.comparisonMinor
        ) else { return L10n.text("New") }
        return analysisFormattedPercent(ratio)
    }

    private var deltaColor: Color {
        guard let ratio = analysisDeltaRatio(
            current: node.amountMinor,
            previous: node.comparisonMinor
        ) else { return BudgetTheme.tertiaryText }
        if ratio > 0.01 { return BudgetTheme.spend }
        if ratio < -0.01 { return BudgetTheme.positive }
        return BudgetTheme.tertiaryText
    }
}

/// A category's own trend, small enough to sit inside a row. Swift Charts would be heavier
/// than this needs to be for a shape with no axes and no labels.
private struct AnalysisSparkline: View {
    let values: [Int64]
    let color: Color

    var body: some View {
        GeometryReader { proxy in
            if values.count > 1 {
                let doubles = values.map(Double.init)
                let minimum = min(doubles.min() ?? 0, 0)
                let maximum = max(doubles.max() ?? 0, 0)
                let span = maximum - minimum
                Path { path in
                    for (index, value) in doubles.enumerated() {
                        let x = proxy.size.width * Double(index) / Double(doubles.count - 1)
                        let ratio = span > 0 ? (value - minimum) / span : 0
                        let y = proxy.size.height - ratio * proxy.size.height
                        if index == 0 {
                            path.move(to: CGPoint(x: x, y: y))
                        } else {
                            path.addLine(to: CGPoint(x: x, y: y))
                        }
                    }
                }
                .stroke(
                    color.opacity(0.8),
                    style: StrokeStyle(lineWidth: 1.5, lineCap: .round, lineJoin: .round)
                )
            }
        }
        .accessibilityHidden(true)
    }
}

// MARK: - Rhythm

private struct AnalysisRhythmSection: View {
    let analysis: BudgetSpendingAnalysis

    private var currency: BudgetCurrency { analysis.period.baseCurrency }
    private var readings: [AnalysisWeekdayReading] { analysisWeekdayReadings(analysis) }

    var body: some View {
        BudgetSection("Spending rhythm", caption: busiestCaption) {
            VStack(spacing: 14) {
                BudgetCard {
                    VStack(spacing: 10) {
                        ForEach(readings) { reading in
                            HStack(spacing: 10) {
                                Text(analysisWeekdayName(reading.weekday, abbreviated: true))
                                    .font(.caption.weight(.semibold))
                                    .foregroundStyle(BudgetTheme.secondaryText)
                                    .frame(width: 38, alignment: .leading)
                                BudgetMeter(progress: reading.share, tint: BudgetTheme.spend)
                                Text(currency.formatted(minorUnits: reading.spendingMinor))
                                    .font(.budgetAmountSmall)
                                    .foregroundStyle(BudgetTheme.secondaryText)
                                    .frame(minWidth: 68, alignment: .trailing)
                                    .minimumScaleFactor(0.7)
                                    .lineLimit(1)
                            }
                            .accessibilityElement(children: .ignore)
                            .accessibilityLabel(
                                String(
                                    format: L10n.text("%1$@: %2$@ across %3$@"),
                                    analysisWeekdayName(reading.weekday),
                                    currency.formatted(minorUnits: reading.spendingMinor),
                                    analysisTransactionCount(reading.transactionCount)
                                )
                            )
                        }
                    }
                }

                BudgetCard {
                    VStack(alignment: .leading, spacing: 10) {
                        BudgetEyebrow("Daily intensity")
                        AnalysisCalendarGrid(analysis: analysis)
                        HStack(spacing: 6) {
                            Spacer(minLength: 0)
                            Text("Quiet")
                                .font(.caption2)
                                .foregroundStyle(BudgetTheme.tertiaryText)
                            LinearGradient(
                                colors: [BudgetTheme.spend.opacity(0.18), BudgetTheme.spend],
                                startPoint: .leading,
                                endPoint: .trailing
                            )
                            .frame(width: 56, height: 7)
                            .clipShape(Capsule())
                            Text("Heavy")
                                .font(.caption2)
                                .foregroundStyle(BudgetTheme.tertiaryText)
                        }
                        .accessibilityHidden(true)
                    }
                }
            }
        }
    }

    private var busiestCaption: LocalizedStringKey {
        guard let busiest = readings.filter({ $0.spendingMinor > 0 })
            .max(by: { $0.spendingMinor < $1.spendingMinor }) else {
            return "No posted spending in this period."
        }
        return .resolved(String(
            format: L10n.text("Most money leaves on %@."),
            analysisWeekdayName(busiest.weekday)
        ))
    }
}

private struct AnalysisCalendarGrid: View {
    let analysis: BudgetSpendingAnalysis

    private var currency: BudgetCurrency { analysis.period.baseCurrency }
    private var weeks: [AnalysisCalendarWeek] { analysisCalendarWeeks(analysis) }

    var body: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(alignment: .top, spacing: 4) {
                VStack(spacing: 3) {
                    ForEach(1...7, id: \.self) { weekday in
                        Text(analysisWeekdayName(weekday, abbreviated: true).prefix(1))
                            .font(.system(size: 8, weight: .semibold))
                            .foregroundStyle(BudgetTheme.tertiaryText)
                            .frame(width: 10, height: 11)
                    }
                }
                .accessibilityHidden(true)

                HStack(spacing: 3) {
                    ForEach(weeks) { week in
                        VStack(spacing: 3) {
                            ForEach(0..<7, id: \.self) { index in
                                cell(week.cells[index])
                            }
                        }
                    }
                }
            }
            .padding(.vertical, 2)
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(L10n.text("Daily spending intensity"))
    }

    @ViewBuilder
    private func cell(_ cell: AnalysisCalendarCell?) -> some View {
        if let cell {
            RoundedRectangle(cornerRadius: 2.5, style: .continuous)
                // The floor keeps a day with activity visible against an empty one, which a
                // purely linear scale would render as indistinguishable.
                .fill(cell.intensity > 0
                    ? BudgetTheme.spend.opacity(0.18 + cell.intensity * 0.82)
                    : BudgetTheme.elevated)
                .frame(width: 11, height: 11)
                .accessibilityElement()
                .accessibilityLabel(
                    String(
                        format: L10n.text("%1$@: %2$@ across %3$@"),
                        budgetDisplayDate(cell.date),
                        currency.formatted(minorUnits: cell.spendingMinor),
                        analysisTransactionCount(cell.transactionCount)
                    )
                )
        } else {
            Color.clear
                .frame(width: 11, height: 11)
                .accessibilityHidden(true)
        }
    }
}

// MARK: - Payees and accounts

private struct AnalysisPayeeSection: View {
    let analysis: BudgetSpendingAnalysis

    private var currency: BudgetCurrency { analysis.period.baseCurrency }
    private var payees: [BudgetAnalysisPayee] {
        Array(analysis.payees.filter { $0.spendingBaseMinor > 0 }.prefix(10))
    }

    var body: some View {
        BudgetSection("Top payees") {
            if payees.isEmpty {
                BudgetCard {
                    BudgetMessage(
                        title: "No payees recorded",
                        systemImage: "storefront",
                        message: "Add a payee when recording a transaction to see who you pay most."
                    )
                }
            } else {
                let peak = payees.map(\.spendingBaseMinor).max() ?? 1
                BudgetRowGroup(items: payees) { payee in
                    VStack(alignment: .leading, spacing: 8) {
                        HStack(alignment: .firstTextBaseline, spacing: 10) {
                            VStack(alignment: .leading, spacing: 2) {
                                Text(payee.payee)
                                    .font(.subheadline.weight(.medium))
                                    .foregroundStyle(BudgetTheme.primaryText)
                                Text(
                                    "\(analysisTransactionCount(payee.transactionCount)) · "
                                    + String(
                                        format: L10n.text("last %@"),
                                        budgetDisplayDate(payee.lastDate)
                                    )
                                )
                                .font(.caption2)
                                .foregroundStyle(BudgetTheme.tertiaryText)
                            }
                            Spacer(minLength: 8)
                            Text(currency.formatted(minorUnits: payee.spendingBaseMinor))
                                .font(.budgetAmount)
                                .foregroundStyle(BudgetTheme.primaryText)
                                .minimumScaleFactor(0.7)
                                .lineLimit(1)
                        }
                        BudgetMeter(
                            progress: Double(payee.spendingBaseMinor) / Double(max(peak, 1)),
                            tint: BudgetTheme.spend
                        )
                    }
                    .accessibilityElement(children: .combine)
                }
            }
        }
    }
}

private struct AnalysisAccountSection: View {
    let analysis: BudgetSpendingAnalysis

    private var currency: BudgetCurrency { analysis.period.baseCurrency }
    private var accounts: [BudgetAnalysisAccount] {
        analysis.accounts.sorted { $0.outflowBaseMinor > $1.outflowBaseMinor }
    }

    var body: some View {
        BudgetSection(
            "Account activity",
            caption: "Excludes transfers between your own accounts."
        ) {
            if accounts.isEmpty {
                BudgetCard {
                    BudgetMessage(
                        title: "No account activity",
                        systemImage: "building.columns",
                        message: "Nothing moved through an account in this period."
                    )
                }
            } else {
                let peak = accounts.map(\.outflowBaseMinor).max() ?? 1
                BudgetRowGroup(items: accounts) { account in
                    VStack(alignment: .leading, spacing: 8) {
                        HStack(alignment: .firstTextBaseline, spacing: 10) {
                            VStack(alignment: .leading, spacing: 2) {
                                Text(account.name)
                                    .font(.subheadline.weight(.medium))
                                    .foregroundStyle(BudgetTheme.primaryText)
                                Text(accountDetail(account))
                                    .font(.caption2)
                                    .foregroundStyle(BudgetTheme.tertiaryText)
                            }
                            Spacer(minLength: 8)
                            VStack(alignment: .trailing, spacing: 2) {
                                Text(currency.formatted(minorUnits: account.outflowBaseMinor))
                                    .font(.budgetAmount)
                                    .foregroundStyle(BudgetTheme.primaryText)
                                    .minimumScaleFactor(0.7)
                                    .lineLimit(1)
                                Text(String(
                                    format: L10n.text("%@ in"),
                                    currency.formatted(minorUnits: account.inflowBaseMinor)
                                ))
                                .font(.caption2)
                                .foregroundStyle(BudgetTheme.tertiaryText)
                            }
                        }
                        BudgetMeter(
                            progress: Double(account.outflowBaseMinor) / Double(max(peak, 1)),
                            tint: BudgetTheme.spend
                        )
                    }
                    .accessibilityElement(children: .combine)
                }
            }
        }
    }

    private func accountDetail(_ account: BudgetAnalysisAccount) -> String {
        var parts = [account.type.title]
        if account.archivedAt != nil {
            parts.append(L10n.text("Archived"))
        }
        return parts.joined(separator: " · ")
    }
}
