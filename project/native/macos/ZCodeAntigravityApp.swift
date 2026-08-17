import AppKit
import Foundation
import SwiftUI

private let appVersion = "0.4.7-test"
private let quotaRefreshInterval: TimeInterval = 5 * 60

private extension Notification.Name {
    static let selectBridgeProvider = Notification.Name("ZCodeSelectBridgeProvider")
    static let refreshBridgeData = Notification.Name("ZCodeRefreshBridgeData")
}

struct NativeConnection: Decodable {
    let baseURL: String
    let session: String
    let dashboardURL: String
}

struct DashboardItem: Decodable {
    let ok: Bool
    let label: String
    let detail: String?
    let running: Bool?
}

struct ProviderAccounts: Decodable {
    let antigravity: Int
    let xai: Int
}

struct OperationState: Decodable {
    let running: Bool
    let name: String?
    let message: String?
    let error: String?
}

struct DashboardStatus: Decodable {
    let version: String
    let gateway: DashboardItem
    let proxy: DashboardItem
    let tun: DashboardItem
    let zcode: DashboardItem
    let providerAccounts: ProviderAccounts
    let selectedProvider: String
    let models: [String]
    let operation: OperationState
}

struct QuotaReport: Decodable {
    let fetchedAt: String?
    let provider: String
    let source: String
    let stale: Bool
    let accounts: [QuotaAccount]
    let warning: String?
}

struct QuotaAccount: Decodable, Identifiable {
    var id: String { account + (plan ?? "") + status }
    let account: String
    let plan: String?
    let status: String
    let statusMessage: String?
    let groups: [QuotaGroup]?
    let credits: CreditInfo?
    let error: String?
}

struct QuotaGroup: Decodable, Identifiable {
    var id: String { name }
    let name: String
    let description: String?
    let buckets: [QuotaBucket]
}

struct QuotaBucket: Decodable, Identifiable {
    var id: String { name + window }
    let name: String
    let description: String?
    let window: String
    let remainingPercent: Double?
    let resetTime: String?
    let disabled: Bool
}

struct CreditInfo: Decodable {
    let available: Bool
    let amount: Double
    let creditType: String
    let upstreamLabel: String
}

struct UsageSample: Decodable {
    let timestamp: String
    let provider: String
    let model: String
    let outputTokens: Int64
    let nonReasoningTokens: Int64
    let reasoningTokens: Int64
    let totalTokens: Int64
    let latencyMs: Int64
    let ttftMs: Int64
    let generationMs: Int64
    let outputTokensPerSecond: Double
    let speedBasis: String
}

struct UsageAggregate: Decodable {
    let requests: Int
    let outputTokens: Int64
    let reasoningTokens: Int64
    let averageTokensPerSecond: Double
    let trackedFrom: String?
}

struct UsageReport: Decodable {
    let provider: String
    let available: Bool
    let latest: UsageSample?
    let total: UsageAggregate
    let warning: String?
}

struct ConnectorResponse: Decodable {
    let provider: String
    let baseURL: String
    let model: String
    let connectors: [AgentConnector]
}

struct AgentConnector: Decodable, Identifiable {
    let id: String
    let name: String
    let description: String
    let model: String
    let snippets: [String: String]
}

struct APIError: Decodable {
    let error: String
}

enum BridgeError: LocalizedError {
    case message(String)

    var errorDescription: String? {
        switch self {
        case .message(let value): return value
        }
    }
}

final class NativeHost: @unchecked Sendable {
    static let shared = NativeHost()

    private let lock = NSLock()
    private var process: Process?
    private var inputPipe: Pipe?

    func start() async throws -> NativeConnection {
        try await withCheckedThrowingContinuation { continuation in
            DispatchQueue.global(qos: .userInitiated).async {
                do {
                    continuation.resume(returning: try self.startBlocking())
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }

    private func startBlocking() throws -> NativeConnection {
        guard let executableDirectory = Bundle.main.executableURL?.deletingLastPathComponent() else {
            throw BridgeError.message("无法定位 App 可执行目录")
        }
        let coreURL = executableDirectory.appendingPathComponent("ZCode-Antigravity-Core")
        guard FileManager.default.isExecutableFile(atPath: coreURL.path) else {
            throw BridgeError.message("原生客户端缺少后台核心：\(coreURL.path)")
        }

        let child = Process()
        let stdout = Pipe()
        let stdin = Pipe()
        child.executableURL = coreURL
        child.arguments = ["native-host"]
        child.currentDirectoryURL = executableDirectory
        child.standardOutput = stdout
        child.standardError = FileHandle.nullDevice
        child.standardInput = stdin
        child.environment = childEnvironment()
        try child.run()

        lock.lock()
        process = child
        inputPipe = stdin
        lock.unlock()

        let reader = stdout.fileHandleForReading
        var line = Data()
        while child.isRunning {
            let next = reader.readData(ofLength: 1)
            if next.isEmpty { break }
            if next.first == 0x0A { break }
            line.append(next)
            if line.count > 32 * 1024 {
                throw BridgeError.message("后台连接信息异常")
            }
        }
        guard !line.isEmpty else {
            throw BridgeError.message("后台核心启动失败")
        }
        let connection: NativeConnection
        do {
            connection = try JSONDecoder().decode(NativeConnection.self, from: line)
        } catch {
            throw BridgeError.message("后台连接信息无法解析：\(error.localizedDescription)")
        }

        DispatchQueue.global(qos: .utility).async {
            while child.isRunning {
                let data = reader.readData(ofLength: 4096)
                if data.isEmpty { break }
            }
        }
        return connection
    }

    private func childEnvironment() -> [String: String] {
        var environment = ProcessInfo.processInfo.environment
        let envURL = Bundle.main.bundleURL.deletingLastPathComponent().appendingPathComponent(".env")
        guard let contents = try? String(contentsOf: envURL, encoding: .utf8) else {
            return environment
        }
        for rawLine in contents.split(whereSeparator: \.isNewline) {
            let line = rawLine.trimmingCharacters(in: .whitespaces)
            guard !line.isEmpty, !line.hasPrefix("#"), let separator = line.firstIndex(of: "=") else { continue }
            let key = line[..<separator].trimmingCharacters(in: .whitespaces)
            var value = line[line.index(after: separator)...].trimmingCharacters(in: .whitespaces)
            if value.count >= 2, (value.hasPrefix("\"") && value.hasSuffix("\"") || value.hasPrefix("'") && value.hasSuffix("'")) {
                value.removeFirst()
                value.removeLast()
            }
            if !key.isEmpty { environment[key] = value }
        }
        return environment
    }

    func stop() {
        lock.lock()
        let child = process
        let input = inputPipe
        process = nil
        inputPipe = nil
        lock.unlock()

        try? input?.fileHandleForWriting.close()
        guard let child, child.isRunning else { return }
        DispatchQueue.global(qos: .utility).asyncAfter(deadline: .now() + 1.5) {
            if child.isRunning { child.terminate() }
        }
    }
}

@MainActor
final class BridgeModel: ObservableObject {
    @Published var status: DashboardStatus?
    @Published var quota: QuotaReport?
    @Published var connectors: ConnectorResponse?
    @Published var usage: UsageReport?
    @Published var provider = "antigravity"
    @Published var providerSwitching = false
    @Published var loading = true
    @Published var message = "正在启动本地安全核心…"
    @Published var errorMessage: String?
    @Published var quotaError: String?
    @Published var lastQuotaRefresh: Date?

    private var connection: NativeConnection?
    private var pollingTask: Task<Void, Never>?
    private var lastQuotaRefreshAttempt: Date?
    private var quotaCache: [String: QuotaReport] = [:]
    private var usageCache: [String: UsageReport] = [:]
    private var notificationTokens: [NSObjectProtocol] = []

    init() {
        notificationTokens.append(NotificationCenter.default.addObserver(forName: .selectBridgeProvider, object: nil, queue: .main) { [weak self] note in
            guard let provider = note.object as? String else { return }
            Task { @MainActor in await self?.selectProvider(provider) }
        })
        notificationTokens.append(NotificationCenter.default.addObserver(forName: .refreshBridgeData, object: nil, queue: .main) { [weak self] _ in
            Task { @MainActor in await self?.refreshAll() }
        })
    }

    deinit {
        for token in notificationTokens { NotificationCenter.default.removeObserver(token) }
        pollingTask?.cancel()
    }

    func start() {
        guard connection == nil else { return }
        loading = true
        Task { [self] in
            do {
                connection = try await NativeHost.shared.start()
                message = "本地核心已连接"
                await refreshAll()
                pollingTask = Task { [weak self] in
                    while !Task.isCancelled {
                        try? await Task.sleep(nanoseconds: 5_000_000_000)
                        await self?.refreshAll(showLoading: false, forceQuota: false)
                    }
                }
            } catch {
                loading = false
                errorMessage = error.localizedDescription
                message = "本地核心启动失败"
            }
        }
    }

    func stop() {
        pollingTask?.cancel()
        NativeHost.shared.stop()
    }

    func refreshAll(showLoading: Bool = true, forceQuota: Bool = true) async {
        guard connection != nil else { return }
        if showLoading { loading = true }
        do {
            let previousProvider = provider
            let operationWasRunning = status?.operation.running == true
            let latest: DashboardStatus = try await request(path: "/api/status")
            status = latest
            if !providerSwitching { provider = latest.selectedProvider }
            errorMessage = nil
            connectors = await optionalRequest(path: "/api/connectors")
            let usageProvider = provider
            if let latestUsage: UsageReport = await optionalRequest(path: "/api/usage?provider=\(usageProvider)") {
                usageCache[usageProvider] = latestUsage
                if provider == usageProvider { usage = latestUsage }
            }
            let providerChanged = previousProvider != provider
            let operationFinished = operationWasRunning && !latest.operation.running
            let quotaDue = lastQuotaRefreshAttempt.map { Date().timeIntervalSince($0) >= quotaRefreshInterval } ?? true
            if !latest.operation.running && (forceQuota || providerChanged || operationFinished || quotaDue) {
                lastQuotaRefreshAttempt = Date()
                do {
                    let latestQuota: QuotaReport = try await request(path: "/api/quota?provider=\(provider)")
                    quota = latestQuota
                    quotaCache[provider] = latestQuota
                    quotaError = nil
                    lastQuotaRefresh = Date()
                } catch {
                    quotaError = error.localizedDescription
                }
            }
            message = latest.operation.message ?? (latest.gateway.ok ? "网关在线" : "等待接入")
            StatusBarController.shared.update(provider: provider, quota: quota, usage: usage, status: latest)
        } catch {
            errorMessage = error.localizedDescription
            message = "状态刷新失败"
            StatusBarController.shared.update(provider: provider, quota: quota, usage: usage, status: status)
        }
        loading = false
    }

    func selectProvider(_ value: String) async {
        let normalized = value == "xai" || value == "grok" ? "xai" : "antigravity"
        guard !providerSwitching, normalized != provider else { return }
        let previous = provider
        providerSwitching = true
        provider = normalized
        quota = quotaCache[normalized]
        usage = usageCache[normalized]
        if let latestUsage: UsageReport = await optionalRequest(path: "/api/usage?provider=\(normalized)") {
            usageCache[normalized] = latestUsage
            if provider == normalized { usage = latestUsage }
        }
        message = normalized == "xai" ? "正在切换到 Grok / xAI…" : "正在切换到 Antigravity…"
        do {
            let _: [String: String] = try await request(path: "/api/provider", method: "POST", body: ["provider": normalized])
            quotaError = nil
            lastQuotaRefresh = nil
            lastQuotaRefreshAttempt = nil
            connectors = nil
            await refreshAll(showLoading: false, forceQuota: true)
        } catch {
            provider = previous
            quota = quotaCache[previous]
            usage = usageCache[previous]
            errorMessage = error.localizedDescription
        }
        providerSwitching = false
    }

    func runAction(_ action: String) async {
        do {
            let _: [String: String] = try await request(path: "/api/action", method: "POST", body: ["action": action])
            message = actionMessage(action)
            errorMessage = nil
            await refreshAll(showLoading: false, forceQuota: false)
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func actionMessage(_ action: String) -> String {
        switch action {
        case "setup": return "正在接入所选提供商…"
        case "login": return "正在打开 Google OAuth…"
        case "login-grok": return "正在打开 xAI 设备授权…"
        case "sync": return "正在重新同步模型…"
        case "stop": return "正在停止本地网关…"
        default: return "正在处理…"
        }
    }

    private func optionalRequest<T: Decodable>(path: String) async -> T? {
        try? await request(path: path)
    }

    private func request<T: Decodable>(path: String, method: String = "GET", body: [String: String]? = nil) async throws -> T {
        guard let connection, let url = URL(string: connection.baseURL + path) else {
            throw BridgeError.message("本地核心尚未连接")
        }
        var request = URLRequest(url: url)
        request.httpMethod = method
        request.cachePolicy = .reloadIgnoringLocalCacheData
        request.timeoutInterval = 40
        request.setValue(connection.session, forHTTPHeaderField: "X-ZCAB-Session")
        if let body {
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
            request.httpBody = try JSONEncoder().encode(body)
        }
        let (data, response) = try await URLSession.shared.data(for: request)
        guard let http = response as? HTTPURLResponse else {
            throw BridgeError.message("本地核心返回了无效响应")
        }
        guard (200..<300).contains(http.statusCode) else {
            if let apiError = try? JSONDecoder().decode(APIError.self, from: data) {
                throw BridgeError.message(apiError.error)
            }
            throw BridgeError.message("本地请求失败（HTTP \(http.statusCode)）")
        }
        return try JSONDecoder().decode(T.self, from: data)
    }
}

@MainActor
final class NavigationModel: ObservableObject {
    static let shared = NavigationModel()
    @Published var section = "usage"
}

@MainActor
final class StatusWidgetModel: ObservableObject {
    @Published var provider = "antigravity"
    @Published var providerName = "Antigravity"
    @Published var accountCount = 0
    @Published var fiveHourPercent: Double?
    @Published var fiveHourReset = "等待同步"
    @Published var weeklyPercent: Double?
    @Published var weeklyReset = "等待同步"
    @Published var outputText = "—"
    @Published var speedText = "等待首次响应"
}

private struct StatusPopoverView: View {
    @ObservedObject var model: StatusWidgetModel

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(spacing: 12) {
                ZStack {
                    RoundedRectangle(cornerRadius: 11)
                        .fill(LinearGradient(colors: [CodexUPalette.accentLight, CodexUPalette.accent], startPoint: .topLeading, endPoint: .bottomTrailing))
                    Text("ZA").font(.caption.bold()).foregroundStyle(.white)
                }
                .frame(width: 38, height: 38)
                VStack(alignment: .leading, spacing: 1) {
                    Text("ZCode Antigravity").font(.headline)
                    Text("额度与 Token 小组件").font(.caption).foregroundStyle(.secondary)
                }
                Spacer()
                Button {
                    NotificationCenter.default.post(name: .refreshBridgeData, object: nil)
                } label: {
                    Image(systemName: "arrow.clockwise").frame(width: 30, height: 30)
                }
                .buttonStyle(.plain)
                .background(.thinMaterial, in: Circle())
            }

            HStack(spacing: 7) {
                widgetProviderButton("Antigravity", provider: "antigravity")
                widgetProviderButton("Grok / xAI", provider: "xai")
            }
            .padding(4)
            .background(.thinMaterial, in: RoundedRectangle(cornerRadius: 14))
            .overlay(RoundedRectangle(cornerRadius: 14).stroke(CodexUPalette.border))

            VStack(alignment: .leading, spacing: 14) {
                HStack {
                    VStack(alignment: .leading, spacing: 2) {
                        Text(model.providerName).font(.title3.weight(.semibold))
                        Text("\(model.accountCount) 个账号 · 本机数据").font(.caption).foregroundStyle(.secondary)
                    }
                    Spacer()
                    Text(model.accountCount > 0 ? "可用" : "待登录")
                        .font(.caption.weight(.bold))
                        .foregroundStyle(model.accountCount > 0 ? .green : .orange)
                        .padding(.horizontal, 10)
                        .padding(.vertical, 5)
                        .background((model.accountCount > 0 ? Color.green : Color.orange).opacity(0.1), in: Capsule())
                }
                HStack(spacing: 16) {
                    WidgetQuotaColumn(title: "5 小时剩余", percent: model.fiveHourPercent, reset: model.fiveHourReset, tint: CodexUPalette.accent)
                    WidgetQuotaColumn(title: "本周剩余", percent: model.weeklyPercent, reset: model.weeklyReset, tint: CodexUPalette.secondary)
                    VStack(alignment: .leading, spacing: 5) {
                        Text("最近输出").font(.caption).foregroundStyle(.secondary)
                        Text(model.outputText).font(.title3.monospacedDigit().weight(.semibold))
                        Text(model.speedText).font(.caption2).foregroundStyle(.secondary).lineLimit(1)
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
            }
            .padding(16)
            .background(CodexUPalette.accent.opacity(0.08), in: RoundedRectangle(cornerRadius: 16))
            .overlay(RoundedRectangle(cornerRadius: 16).stroke(CodexUPalette.accent.opacity(0.35)))

            HStack(spacing: 8) {
                widgetAction("打开主界面", icon: "macwindow") { StatusBarController.shared.openWindow() }
                widgetAction("刷新", icon: "arrow.clockwise") { NotificationCenter.default.post(name: .refreshBridgeData, object: nil) }
                widgetAction("退出", icon: "power") { NSApp.terminate(nil) }
            }
        }
        .padding(18)
        .frame(width: 430)
        .background(.ultraThinMaterial)
    }

    private func widgetProviderButton(_ title: String, provider: String) -> some View {
        Button {
            NotificationCenter.default.post(name: .selectBridgeProvider, object: provider)
        } label: {
            Text(title).font(.callout.weight(.medium)).frame(maxWidth: .infinity).padding(.vertical, 8)
        }
        .buttonStyle(.plain)
        .foregroundStyle(model.provider == provider ? Color.white : Color.secondary)
        .background(model.provider == provider ? CodexUPalette.accent : Color.clear, in: RoundedRectangle(cornerRadius: 11))
    }

    private func widgetAction(_ title: String, icon: String, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Label(title, systemImage: icon).frame(maxWidth: .infinity).padding(.vertical, 9)
        }
        .buttonStyle(.plain)
        .background(.thinMaterial, in: RoundedRectangle(cornerRadius: 11))
        .overlay(RoundedRectangle(cornerRadius: 11).stroke(CodexUPalette.border))
    }
}

private struct WidgetQuotaColumn: View {
    let title: String
    let percent: Double?
    let reset: String
    let tint: Color

    var body: some View {
        VStack(alignment: .leading, spacing: 5) {
            Text(title).font(.caption).foregroundStyle(.secondary)
            Text(percent.map { String(format: "%.0f%%", $0) } ?? "—")
                .font(.title3.monospacedDigit().weight(.semibold))
            ProgressView(value: max(0, min(100, percent ?? 0)), total: 100).tint(tint)
            Text(reset).font(.caption2.monospacedDigit()).foregroundStyle(.secondary).lineLimit(1)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

@MainActor
final class StatusBarController: NSObject {
    static let shared = StatusBarController()

    private var statusItem: NSStatusItem?
    private let popover = NSPopover()
    private let widgetModel = StatusWidgetModel()

    func install() {
        guard statusItem == nil else { return }
        let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        item.button?.image = NSImage(systemSymbolName: "gauge.with.dots.needle.67percent", accessibilityDescription: "ZCode 额度")
        item.button?.image?.isTemplate = true
        item.button?.toolTip = "ZCode Antigravity 额度"
        item.button?.target = self
        item.button?.action = #selector(togglePopover)
        item.button?.sendAction(on: [.leftMouseUp, .rightMouseUp])
        popover.behavior = .transient
        popover.animates = true
        popover.contentSize = NSSize(width: 430, height: 330)
        popover.contentViewController = NSHostingController(rootView: StatusPopoverView(model: widgetModel))
        statusItem = item
    }

    func update(provider: String, quota: QuotaReport?, usage: UsageReport?, status: DashboardStatus?) {
        DispatchQueue.main.async {
            let name = provider == "xai" ? "Grok / xAI" : "Antigravity"
            let count = provider == "xai" ? status?.providerAccounts.xai ?? 0 : status?.providerAccounts.antigravity ?? 0
            let five = self.quotaWindow(quota, kind: "five")
            let week = self.quotaWindow(quota, kind: "week")
            self.widgetModel.provider = provider
            self.widgetModel.providerName = name
            self.widgetModel.accountCount = count
            self.widgetModel.fiveHourPercent = five?.remainingPercent
            self.widgetModel.fiveHourReset = self.quotaResetText(five)
            self.widgetModel.weeklyPercent = week?.remainingPercent
            self.widgetModel.weeklyReset = self.quotaResetText(week)
            if let latest = usage?.latest {
                let speedLabel = latest.speedBasis == "generation" ? "生成速度" : "有效吞吐"
                self.widgetModel.outputText = "\(self.formatInteger(latest.outputTokens)) tok"
                self.widgetModel.speedText = String(format: "%@ %.1f tok/s", speedLabel, latest.outputTokensPerSecond)
            } else {
                self.widgetModel.outputText = "—"
                self.widgetModel.speedText = "等待首次响应"
            }
            var buttonParts: [String] = []
            if let remaining = five?.remainingPercent { buttonParts.append(String(format: "5h %.0f%%", remaining)) }
            if let remaining = week?.remainingPercent { buttonParts.append(String(format: "周 %.0f%%", remaining)) }
            self.statusItem?.button?.title = buttonParts.isEmpty ? "" : " " + buttonParts.joined(separator: " · ")
            self.statusItem?.button?.toolTip = "\(name) · \(count) 个账号\n\(self.quotaMenuTitle("5 小时", bucket: five))\n\(self.quotaMenuTitle("本周", bucket: week))"
        }
    }

    @objc private func togglePopover() {
        guard let button = statusItem?.button else { return }
        if popover.isShown {
            popover.performClose(nil)
        } else {
            popover.show(relativeTo: button.bounds, of: button, preferredEdge: .minY)
            NSApp.activate(ignoringOtherApps: true)
        }
    }

    private func quotaWindow(_ quota: QuotaReport?, kind: String) -> QuotaBucket? {
        quota?.accounts
            .flatMap { $0.groups ?? [] }
            .flatMap { group in
                group.buckets.filter { bucket in
                    let search = "\(group.name) \(bucket.name) \(bucket.window)".lowercased()
                    if kind == "five" {
                        return search.contains("5小时") || search.contains("5 小时") || search.contains("5-hour") || search.contains("5 hour") || search.contains("5h")
                    }
                    return search.contains("week") || search.contains("周") || search.contains("7-day") || search.contains("7 day") || search.contains("7天")
                }
            }
            .filter { $0.remainingPercent != nil }
            .min { ($0.remainingPercent ?? 101) < ($1.remainingPercent ?? 101) }
    }

    private func quotaMenuTitle(_ label: String, bucket: QuotaBucket?) -> String {
        guard let bucket, let remaining = bucket.remainingPercent else { return "\(label)剩余：当前提供商未提供" }
        let reset = bucket.resetTime.map { " · 重置 " + String($0.prefix(16)).replacingOccurrences(of: "T", with: " ") } ?? ""
        return String(format: "%@剩余：%.0f%%%@", label, remaining, reset)
    }

    private func quotaResetText(_ bucket: QuotaBucket?) -> String {
        guard let raw = bucket?.resetTime else { return "等待同步" }
        return String(raw.prefix(16)).replacingOccurrences(of: "T", with: " ")
    }

    private func formatInteger(_ value: Int64) -> String {
        let formatter = NumberFormatter()
        formatter.numberStyle = .decimal
        formatter.maximumFractionDigits = 0
        return formatter.string(from: NSNumber(value: value)) ?? "\(value)"
    }

    func openWindow() {
        popover.performClose(nil)
        NSApp.activate(ignoringOtherApps: true)
        NSApp.windows.first(where: { $0.canBecomeKey })?.makeKeyAndOrderFront(nil)
    }
}

final class AppDelegate: NSObject, NSApplicationDelegate {
    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApp.setActivationPolicy(.regular)
        StatusBarController.shared.install()
        NSApp.activate(ignoringOtherApps: true)
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { false }

    func applicationWillTerminate(_ notification: Notification) {
        NativeHost.shared.stop()
    }

    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
        if !flag {
            sender.windows.first(where: { $0.canBecomeKey })?.makeKeyAndOrderFront(nil)
        }
        return true
    }
}

@main
struct ZCodeAntigravityApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
    @StateObject private var model = BridgeModel()

    var body: some Scene {
        WindowGroup("ZCode · Antigravity 控制中心") {
            DashboardView()
                .environmentObject(model)
                .frame(minWidth: 1020, minHeight: 720)
                .onAppear { model.start() }
        }
        .windowStyle(.titleBar)
        .commands {
            CommandGroup(after: .appInfo) {
                Button("刷新状态") { Task { await model.refreshAll() } }
                    .keyboardShortcut("r", modifiers: [.command])
            }
        }
    }
}

private enum CodexUPalette {
    static let accent = Color(red: 40 / 255, green: 102 / 255, blue: 247 / 255)
    static let accentLight = Color(red: 123 / 255, green: 160 / 255, blue: 255 / 255)
    static let secondary = Color(red: 139 / 255, green: 109 / 255, blue: 255 / 255)
    static let tertiary = Color(red: 255 / 255, green: 159 / 255, blue: 10 / 255)
    static let border = Color(red: 148 / 255, green: 163 / 255, blue: 184 / 255).opacity(0.32)
    static let pageBackground = Color(nsColor: .windowBackgroundColor)
}

private struct ThemeModeButton: View {
    let icon: String
    let value: String
    @Binding var selection: String

    var body: some View {
        Button { selection = value } label: {
            Image(systemName: icon)
                .font(.system(size: 13, weight: .medium))
                .frame(width: 30, height: 30)
        }
        .buttonStyle(.plain)
        .foregroundStyle(selection == value ? Color.white : Color.secondary)
        .background(selection == value ? CodexUPalette.accent : Color.clear, in: Circle())
    }
}

struct DashboardView: View {
    @EnvironmentObject private var model: BridgeModel
    @ObservedObject private var navigation = NavigationModel.shared
    @AppStorage("zcode.theme") private var theme = "system"

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                topToolbar
                contextRow
                providerTabs
                statusStrip
                if navigation.section == "connectors" { connectorPanel } else { usageAndActions }
            }
            .padding(22)
            .frame(maxWidth: 1280, alignment: .leading)
        }
        .background(CodexUPalette.pageBackground.ignoresSafeArea())
        .preferredColorScheme(theme == "light" ? .light : (theme == "dark" ? .dark : nil))
        .animation(.easeInOut(duration: 0.18), value: model.provider)
        .animation(.easeInOut(duration: 0.18), value: navigation.section)
    }

    private var topToolbar: some View {
        HStack(spacing: 14) {
            HStack(spacing: 12) {
                ZStack {
                    RoundedRectangle(cornerRadius: 14).fill(
                        LinearGradient(colors: [CodexUPalette.accentLight, CodexUPalette.accent], startPoint: .topLeading, endPoint: .bottomTrailing)
                    )
                    Text("ZA").font(.system(size: 17, weight: .bold, design: .rounded)).foregroundStyle(.white)
                }
                .frame(width: 48, height: 48)
                .overlay(RoundedRectangle(cornerRadius: 14).stroke(.white.opacity(0.7), lineWidth: 1))
                VStack(alignment: .leading, spacing: 2) {
                    Text("ZCode Antigravity").font(.system(size: 20, weight: .semibold, design: .rounded))
                    Text("Updated " + lastUpdateText).font(.caption).foregroundStyle(.secondary)
                }
            }
            Spacer()
            HStack(spacing: 4) {
                ThemeModeButton(icon: "sun.max", value: "light", selection: $theme)
                ThemeModeButton(icon: "moon", value: "dark", selection: $theme)
                ThemeModeButton(icon: "display", value: "system", selection: $theme)
            }
            .padding(3)
            .background(.thinMaterial, in: Capsule())
            .overlay(Capsule().stroke(CodexUPalette.border))
            Button { Task { await model.refreshAll() } } label: {
                Image(systemName: "arrow.clockwise")
                    .font(.system(size: 15, weight: .semibold))
                    .frame(width: 38, height: 38)
            }
            .buttonStyle(.plain)
            .background(.thinMaterial, in: Circle())
            .overlay(Circle().stroke(CodexUPalette.border))
            .rotationEffect(.degrees(model.loading ? 360 : 0))
            .animation(model.loading ? .linear(duration: 0.8).repeatForever(autoreverses: false) : .default, value: model.loading)
        }
        .padding(.horizontal, 20)
        .padding(.vertical, 14)
        .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 18))
        .overlay(RoundedRectangle(cornerRadius: 18).stroke(CodexUPalette.border))
        .shadow(color: .black.opacity(0.07), radius: 18, y: 8)
    }

    private var contextRow: some View {
        HStack(spacing: 12) {
            Label("Local only", systemImage: "waveform.path.ecg")
                .foregroundStyle(.orange)
                .font(.caption.weight(.medium))
                .padding(.horizontal, 12)
                .padding(.vertical, 7)
                .background(.thinMaterial, in: Capsule())
                .overlay(Capsule().stroke(CodexUPalette.border))
            Text("\(model.provider == "xai" ? "Grok / xAI" : "Antigravity") · \(selectedProviderCount) 个账号")
                .font(.callout)
                .foregroundStyle(.secondary)
            Spacer()
            Label("额度每 5 分钟 · Token 每 5 秒", systemImage: "clock.arrow.circlepath")
                .font(.caption)
                .foregroundStyle(.secondary)
                .padding(.horizontal, 12)
                .padding(.vertical, 7)
                .background(.thinMaterial, in: Capsule())
                .overlay(Capsule().stroke(CodexUPalette.border))
        }
        .padding(.horizontal, 6)
    }

    private var providerTabs: some View {
        HStack(spacing: 7) {
            ProviderChoiceButton(
                title: "Antigravity",
                subtitle: "Google · Gemini",
                count: model.status?.providerAccounts.antigravity ?? 0,
                selected: model.provider != "xai" && navigation.section == "usage",
                switching: model.providerSwitching && model.provider != "xai"
            ) {
                navigation.section = "usage"
                Task { await model.selectProvider("antigravity") }
            }
            ProviderChoiceButton(
                title: "Grok / xAI",
                subtitle: "Grok Build",
                count: model.status?.providerAccounts.xai ?? 0,
                selected: model.provider == "xai" && navigation.section == "usage",
                switching: model.providerSwitching && model.provider == "xai"
            ) {
                navigation.section = "usage"
                Task { await model.selectProvider("xai") }
            }
            Button {
                navigation.section = "connectors"
            } label: {
                Label("Agent 接入", systemImage: "point.3.connected.trianglepath.dotted")
                    .font(.callout.weight(.medium))
                    .padding(.horizontal, 16)
                    .frame(minHeight: 42)
            }
            .buttonStyle(.plain)
            .foregroundStyle(navigation.section == "connectors" ? Color.white : Color.secondary)
            .background(navigation.section == "connectors" ? CodexUPalette.accent : Color.clear, in: RoundedRectangle(cornerRadius: 13))
            Spacer()
            if let error = model.errorMessage {
                Label(error, systemImage: "exclamationmark.triangle.fill").font(.caption)
                    .foregroundStyle(.red)
            } else {
                Label(model.message, systemImage: "checkmark.circle").font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
        .padding(5)
        .background(.thinMaterial, in: RoundedRectangle(cornerRadius: 18))
        .overlay(RoundedRectangle(cornerRadius: 18).stroke(CodexUPalette.border))
        .disabled(model.providerSwitching)
    }

    private var selectedProviderCount: Int {
        model.provider == "xai" ? model.status?.providerAccounts.xai ?? 0 : model.status?.providerAccounts.antigravity ?? 0
    }

    private var lastUpdateText: String {
        guard let date = model.lastQuotaRefresh else { return "等待首次刷新" }
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "zh_CN")
        formatter.dateFormat = "HH:mm:ss"
        return formatter.string(from: date)
    }

    private var statusStrip: some View {
        HStack(spacing: 10) {
            StatusTile(name: "TUN", item: model.status?.tun)
            StatusTile(name: "PROXY", item: model.status?.proxy)
            StatusTile(name: "BRIDGE", item: model.status?.gateway)
            StatusTile(name: "ZCODE", item: model.status?.zcode)
        }
    }

    private var usageAndActions: some View {
        HStack(alignment: .top, spacing: 18) {
            NativeCard {
                VStack(alignment: .leading, spacing: 16) {
                    CardTitle(kicker: model.provider == "xai" ? "GROK USAGE" : "ANTIGRAVITY USAGE", title: model.provider == "xai" ? "Grok 共享额度" : "Gemini 模型额度", icon: "chart.bar.fill")
                    performanceSummary
                    quotaContent
                }
            }
            .frame(maxWidth: .infinity)
            NativeCard {
                VStack(alignment: .leading, spacing: 12) {
                    CardTitle(kicker: "LOCAL ACTIONS", title: "接入控制", icon: "bolt.shield.fill")
                    ActionButton(title: "一键接入 ZCode", subtitle: "启动网关并同步当前账号", primary: true) { await model.runAction("setup") }
                    ActionButton(title: "登录 Antigravity", subtitle: "Google OAuth") { await model.runAction("login") }
                    ActionButton(title: "登录 Grok / xAI", subtitle: "xAI 设备授权") { await model.runAction("login-grok") }
                    ActionButton(title: "修复并重新同步", subtitle: "重新校验模型与 Provider") { await model.runAction("sync") }
                    ActionButton(title: "打开 ZCode", subtitle: "接入完成后开始使用") { await model.runAction("open-zcode") }
                    ActionButton(title: "停止本地网关", subtitle: "不会删除账号或聊天", destructive: true) { await model.runAction("stop") }
                }
            }
            .frame(width: 330)
        }
    }

    private var performanceSummary: some View {
        VStack(alignment: .leading, spacing: 9) {
            LazyVGrid(columns: [GridItem(.adaptive(minimum: 132), spacing: 9)], spacing: 9) {
                QuotaSummaryTile(
                    title: "最近输出",
                    value: model.usage?.latest.map { "\(formatToken($0.outputTokens)) tok" } ?? "—",
                    icon: "text.line.last.and.arrowtriangle.forward",
                    tint: .blue
                )
                QuotaSummaryTile(
                    title: model.usage?.latest?.speedBasis == "generation" ? "生成速度" : "有效吞吐",
                    value: model.usage?.latest.map { String(format: "%.1f tok/s", $0.outputTokensPerSecond) } ?? "—",
                    icon: "speedometer",
                    tint: .purple
                )
                QuotaSummaryTile(
                    title: "推理 Token",
                    value: model.usage?.latest.map { formatToken($0.reasoningTokens) } ?? "—",
                    icon: "brain.head.profile",
                    tint: .orange
                )
                QuotaSummaryTile(
                    title: "本地累计输出",
                    value: model.usage?.available == true ? "\(formatToken(model.usage?.total.outputTokens ?? 0)) tok" : "—",
                    icon: "sum",
                    tint: .green
                )
            }
            if let latest = model.usage?.latest {
                let timing = latest.speedBasis == "generation"
                    ? String(format: "生成 %.1fs · 首字节 %.1fs", Double(latest.generationMs) / 1000, Double(latest.ttftMs) / 1000)
                    : String(format: "完整调用 %.1fs · 当前显示有效吞吐", Double(latest.latencyMs) / 1000)
                Text("\(latest.model) · \(timing) · 本地 \(model.usage?.total.requests ?? 0) 次平均 \(String(format: "%.1f", model.usage?.total.averageTokensPerSecond ?? 0)) tok/s")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                    .lineLimit(2)
            } else {
                Text("Token 统计已启用；完成一次模型响应后显示真实输出量与速度，不保存提示词或回复。")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            }
        }
    }

    private func formatToken(_ value: Int64) -> String {
        let formatter = NumberFormatter()
        formatter.numberStyle = .decimal
        formatter.maximumFractionDigits = 0
        return formatter.string(from: NSNumber(value: value)) ?? "\(value)"
    }

    @ViewBuilder
    private var quotaContent: some View {
        if let report = model.quota, !report.accounts.isEmpty {
            HStack(spacing: 10) {
                QuotaSummaryTile(
                    title: "账号",
                    value: "\(report.accounts.count)",
                    icon: "person.2.fill",
                    tint: .blue
                )
                QuotaSummaryTile(
                    title: "最低剩余",
                    value: lowestQuota(in: report).map { String(format: "%.0f%%", $0) } ?? "—",
                    icon: "gauge.with.dots.needle.33percent",
                    tint: quotaColor(lowestQuota(in: report))
                )
                QuotaSummaryTile(
                    title: "最近刷新",
                    value: refreshTimeText,
                    icon: "clock.fill",
                    tint: .indigo
                )
            }
            if let quotaError = model.quotaError {
                Label("本次刷新失败，继续显示上次成功数据：\(quotaError)", systemImage: "exclamationmark.triangle.fill")
                    .font(.caption)
                    .foregroundStyle(.orange)
                    .textSelection(.enabled)
            }
            ForEach(report.accounts) { account in
                VStack(alignment: .leading, spacing: 12) {
                    HStack {
                        VStack(alignment: .leading, spacing: 2) {
                            Text(account.account).font(.headline)
                            Text(account.plan ?? account.status).font(.caption).foregroundStyle(.secondary)
                        }
                        Spacer()
                        Text(account.status)
                            .font(.caption2.weight(.semibold))
                            .foregroundStyle(account.status == "active" ? Color.green : Color.secondary)
                            .padding(.horizontal, 8)
                            .padding(.vertical, 4)
                            .background((account.status == "active" ? Color.green : Color.secondary).opacity(0.1), in: Capsule())
                        if let credits = account.credits, credits.available {
                            Text(String(format: "$%.2f %@", credits.amount, credits.creditType))
                                .font(.caption.monospacedDigit()).padding(.horizontal, 8).padding(.vertical, 4)
                                .background(Color.accentColor.opacity(0.1), in: Capsule())
                        }
                    }
                    if let error = account.error, !error.isEmpty {
                        Label(error, systemImage: "exclamationmark.circle").foregroundStyle(.red).font(.callout)
                    }
                    ForEach(account.groups ?? []) { group in
                        VStack(alignment: .leading, spacing: 9) {
                            HStack(alignment: .firstTextBaseline) {
                                Text(group.name).font(.subheadline.weight(.semibold))
                                if let description = group.description, !description.isEmpty {
                                    Text(description).font(.caption2).foregroundStyle(.secondary).lineLimit(1)
                                }
                            }
                            ForEach(group.buckets) { bucket in QuotaRow(bucket: bucket) }
                        }
                    }
                }
                .padding(14)
                .background(Color(nsColor: .controlBackgroundColor), in: RoundedRectangle(cornerRadius: 12))
            }
            HStack {
                Label(report.stale ? "缓存数据" : "实时接口", systemImage: report.stale ? "clock.badge.exclamationmark" : "bolt.horizontal.circle")
                Spacer()
                Text("额度将在 5 分钟内自动更新，也可随时手动刷新")
            }
            .font(.caption2)
            .foregroundStyle(.secondary)
        } else {
            ContentUnavailableViewCompat(
                title: model.status?.gateway.ok == true ? "额度暂不可用" : "等待启动网关",
                detail: model.quotaError ?? "登录所选提供商并完成一次接入后，这里会显示真实剩余额度。"
            )
        }
    }

    private func lowestQuota(in report: QuotaReport) -> Double? {
        report.accounts
            .flatMap { $0.groups ?? [] }
            .flatMap(\.buckets)
            .compactMap(\.remainingPercent)
            .min()
    }

    private func quotaColor(_ remaining: Double?) -> Color {
        guard let remaining else { return .secondary }
        if remaining < 20 { return .red }
        if remaining < 50 { return .orange }
        return .green
    }

    private var refreshTimeText: String {
        guard let date = model.lastQuotaRefresh else { return "等待首次刷新" }
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "zh_CN")
        formatter.dateFormat = "HH:mm:ss"
        return formatter.string(from: date)
    }

    private var connectorPanel: some View {
        NativeCard {
            VStack(alignment: .leading, spacing: 18) {
                CardTitle(kicker: "AGENT CONNECTORS", title: "原生配置助手", icon: "point.3.filled.connected.trianglepath.dotted")
                if let connectors = model.connectors {
                    HStack {
                        Label(connectors.model, systemImage: "cpu")
                        Spacer()
                        Text(connectors.baseURL).font(.caption.monospaced()).foregroundStyle(.secondary).textSelection(.enabled)
                    }
                    ForEach(connectors.connectors) { connector in ConnectorDisclosure(connector: connector) }
                    Label("配置包含当前用户的本机密钥，请勿发给他人。", systemImage: "lock.fill")
                        .font(.caption).foregroundStyle(.secondary)
                } else {
                    ContentUnavailableViewCompat(title: "等待网关", detail: "启动网关后自动生成 Grok Build、Codex、Claude Code、OpenCode 和通用 Agent 配置。")
                }
            }
        }
    }
}

struct ProviderChoiceButton: View {
    let title: String
    let subtitle: String
    let count: Int
    let selected: Bool
    let switching: Bool
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: 10) {
                ZStack {
                    Circle()
                        .fill(selected ? Color.white.opacity(0.2) : Color.secondary.opacity(0.14))
                        .frame(width: 24, height: 24)
                    if switching {
                        ProgressView().controlSize(.mini).tint(.white)
                    } else if selected {
                        Image(systemName: "checkmark").font(.caption.bold()).foregroundStyle(.white)
                    } else {
                        Circle().fill(Color.secondary.opacity(0.7)).frame(width: 7, height: 7)
                    }
                }
                VStack(alignment: .leading, spacing: 2) {
                    Text(title).font(.callout.weight(.semibold))
                    Text(subtitle).font(.caption2).opacity(0.72)
                }
                Spacer(minLength: 8)
                Text("\(count)")
                    .font(.caption.monospacedDigit().weight(.medium))
                    .padding(.horizontal, 8)
                    .padding(.vertical, 4)
                    .background((selected ? Color.white : CodexUPalette.accent).opacity(0.12), in: Capsule())
            }
            .padding(.horizontal, 13)
            .padding(.vertical, 7)
            .frame(minWidth: 190, minHeight: 44)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .foregroundStyle(selected ? Color.white : Color.primary)
        .background(selected ? CodexUPalette.accent : Color.clear, in: RoundedRectangle(cornerRadius: 13))
        .overlay(RoundedRectangle(cornerRadius: 13).stroke(selected ? CodexUPalette.accent : Color.clear))
        .shadow(color: selected ? CodexUPalette.accent.opacity(0.24) : .clear, radius: 10, y: 5)
        .animation(.easeInOut(duration: 0.18), value: selected)
    }
}

struct SidebarButton: View {
    let title: String
    let icon: String
    let selected: Bool
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            Label(title, systemImage: icon).frame(maxWidth: .infinity, alignment: .leading).padding(.vertical, 9).padding(.horizontal, 10)
        }
        .buttonStyle(.plain)
        .background(selected ? Color.accentColor.opacity(0.16) : Color.clear, in: RoundedRectangle(cornerRadius: 8))
        .foregroundStyle(selected ? Color.accentColor : Color.primary)
    }
}

struct StatusTile: View {
    let name: String
    let item: DashboardItem?

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(spacing: 6) {
                Circle().fill(item?.ok == true ? Color.green : Color.secondary.opacity(0.45)).frame(width: 7, height: 7)
                Text(name).font(.caption2.weight(.semibold)).tracking(1).foregroundStyle(.secondary)
            }
            Text(item?.label ?? "正在检查").font(.subheadline.weight(.medium)).lineLimit(1)
            Text(item?.detail ?? " ").font(.caption2).foregroundStyle(.secondary).lineLimit(1)
        }
        .padding(13)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 16))
        .overlay(RoundedRectangle(cornerRadius: 16).stroke(CodexUPalette.border))
        .shadow(color: .black.opacity(0.045), radius: 12, y: 5)
    }
}

struct NativeCard<Content: View>: View {
    @ViewBuilder let content: () -> Content
    var body: some View {
        content()
            .padding(20)
            .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 18))
            .overlay(RoundedRectangle(cornerRadius: 18).stroke(CodexUPalette.border))
            .shadow(color: .black.opacity(0.06), radius: 18, y: 8)
    }
}

struct CardTitle: View {
    let kicker: String
    let title: String
    let icon: String
    var body: some View {
        HStack {
            VStack(alignment: .leading, spacing: 4) {
                Text(kicker).font(.caption2.weight(.bold)).tracking(1.4).foregroundStyle(CodexUPalette.accent)
                Text(title).font(.title2.weight(.semibold))
            }
            Spacer()
            Image(systemName: icon)
                .font(.system(size: 16, weight: .semibold))
                .foregroundStyle(CodexUPalette.accent)
                .frame(width: 34, height: 34)
                .background(CodexUPalette.accent.opacity(0.1), in: RoundedRectangle(cornerRadius: 10))
        }
    }
}

struct QuotaSummaryTile: View {
    let title: String
    let value: String
    let icon: String
    let tint: Color

    var body: some View {
        HStack(spacing: 10) {
            Image(systemName: icon)
                .font(.title3)
                .foregroundStyle(tint)
                .frame(width: 34, height: 34)
                .background(tint.opacity(0.12), in: RoundedRectangle(cornerRadius: 9))
            VStack(alignment: .leading, spacing: 2) {
                Text(title).font(.caption2).foregroundStyle(.secondary)
                Text(value).font(.headline.monospacedDigit()).lineLimit(1)
            }
            Spacer(minLength: 0)
        }
        .padding(11)
        .frame(maxWidth: .infinity, minHeight: 62, alignment: .leading)
        .background(.thinMaterial, in: RoundedRectangle(cornerRadius: 13))
        .overlay(RoundedRectangle(cornerRadius: 13).stroke(CodexUPalette.border))
    }
}

struct QuotaRow: View {
    let bucket: QuotaBucket

    private var quotaTint: Color {
        guard let remaining = bucket.remainingPercent else { return .secondary }
        if remaining < 20 { return .red }
        if remaining < 50 { return .orange }
        return .green
    }

    private var resetText: String? {
        guard let raw = bucket.resetTime else { return nil }
        let iso = ISO8601DateFormatter()
        iso.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        let date = iso.date(from: raw) ?? ISO8601DateFormatter().date(from: raw)
        guard let date else { return raw }
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "zh_CN")
        formatter.dateFormat = "MM-dd HH:mm"
        return formatter.string(from: date)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack {
                Text(bucket.name).font(.callout)
                Spacer()
                if let reset = resetText {
                    Text("重置 \(reset)").font(.caption2.monospacedDigit()).foregroundStyle(.secondary)
                }
                if let remaining = bucket.remainingPercent {
                    Text(String(format: "%.0f%%", remaining))
                        .font(.callout.monospacedDigit().weight(.semibold))
                        .foregroundStyle(quotaTint)
                } else { Text("—").foregroundStyle(.secondary) }
            }
            ProgressView(value: max(0, min(100, bucket.remainingPercent ?? 0)), total: 100)
                .tint(quotaTint)
                .animation(.easeInOut(duration: 0.35), value: bucket.remainingPercent)
            if let description = bucket.description, !description.isEmpty {
                Text(description).font(.caption2).foregroundStyle(.secondary).lineLimit(1)
            }
        }
    }
}

struct ActionButton: View {
    let title: String
    let subtitle: String
    var primary = false
    var destructive = false
    let action: () async -> Void

    var body: some View {
        Button { Task { await action() } } label: {
            HStack {
                VStack(alignment: .leading, spacing: 2) {
                    Text(title).font(.callout.weight(.semibold))
                    Text(subtitle).font(.caption).opacity(0.75)
                }
                Spacer()
                Image(systemName: "chevron.right").font(.caption.weight(.bold))
            }
            .padding(11)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .foregroundStyle(primary ? Color.white : (destructive ? Color.red : Color.primary))
        .background(primary ? CodexUPalette.accent : Color.clear, in: RoundedRectangle(cornerRadius: 12))
        .overlay(RoundedRectangle(cornerRadius: 12).stroke(primary ? Color.clear : CodexUPalette.border))
        .shadow(color: primary ? CodexUPalette.accent.opacity(0.2) : .clear, radius: 10, y: 5)
    }
}

struct ConnectorDisclosure: View {
    let connector: AgentConnector

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            VStack(alignment: .leading, spacing: 3) {
                Text(connector.name).font(.headline)
                Text(connector.description).font(.caption).foregroundStyle(.secondary)
            }.padding(.vertical, 4)
            ForEach(connector.snippets.keys.sorted(), id: \.self) { key in
                VStack(alignment: .leading, spacing: 6) {
                    HStack {
                        Text(key).font(.caption.weight(.semibold)).foregroundStyle(.secondary)
                        Spacer()
                        Button("复制") {
                            NSPasteboard.general.clearContents()
                            NSPasteboard.general.setString(connector.snippets[key] ?? "", forType: .string)
                        }.controlSize(.small)
                    }
                    ScrollView(.horizontal) {
                        Text(connector.snippets[key] ?? "").font(.system(.caption, design: .monospaced)).textSelection(.enabled).padding(10)
                    }
                    .background(Color.black.opacity(0.045), in: RoundedRectangle(cornerRadius: 8))
                }
            }
        }
        .padding(14)
        .background(Color(nsColor: .controlBackgroundColor), in: RoundedRectangle(cornerRadius: 12))
    }
}

struct ContentUnavailableViewCompat: View {
    let title: String
    let detail: String
    var body: some View {
        VStack(spacing: 10) {
            Image(systemName: "waveform.path.ecg.rectangle").font(.system(size: 34)).foregroundStyle(Color.accentColor)
            Text(title).font(.headline)
            Text(detail).font(.callout).foregroundStyle(.secondary).multilineTextAlignment(.center).frame(maxWidth: 420)
        }
        .frame(maxWidth: .infinity, minHeight: 220)
        .padding()
    }
}
