import Foundation

func workspaceMonthStart(_ date: Date = Date(), timezone: String) -> Date {
    var calendar = Calendar(identifier: .gregorian)
    calendar.timeZone = TimeZone(identifier: timezone) ?? .current
    let components = calendar.dateComponents([.year, .month], from: date)
    return calendar.date(from: components) ?? date
}

func workspaceMonthKey(_ date: Date = Date(), timezone: String) -> String {
    let formatter = DateFormatter()
    formatter.calendar = Calendar(identifier: .gregorian)
    formatter.locale = Locale(identifier: "en_US_POSIX")
    formatter.timeZone = TimeZone(identifier: timezone) ?? .current
    formatter.dateFormat = "yyyy-MM"
    return formatter.string(from: date)
}

func budgetMonthLabel(_ month: String) -> String {
    let parser = DateFormatter()
    parser.calendar = Calendar(identifier: .gregorian)
    parser.locale = Locale(identifier: "en_US_POSIX")
    parser.timeZone = TimeZone(secondsFromGMT: 0)
    parser.dateFormat = "yyyy-MM"
    let formatter = DateFormatter()
    formatter.calendar = Calendar(identifier: .gregorian)
    formatter.locale = .current
    formatter.timeZone = TimeZone(secondsFromGMT: 0)
    formatter.dateFormat = "LLLL yyyy"
    return parser.date(from: month).map(formatter.string) ?? month
}
