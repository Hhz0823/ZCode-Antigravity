import {
  Activity,
  BarChart3,
  Bot,
  Boxes,
  Check,
  ChevronRight,
  CircleGauge,
  Cloud,
  Copy,
  Cpu,
  ExternalLink,
  Gauge,
  GlassWater,
  KeyRound,
  LoaderCircle,
  Maximize2,
  Minimize2,
  Network,
  Power,
  RefreshCw,
  RotateCcw,
  Settings2,
  ShieldCheck,
  Sparkles,
  Users,
  Wifi,
  X,
  Zap,
  type LucideIcon,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import brandMark from "./assets/BrandMark.png";
import { Badge, Button, Card, CardHeader, Progress, Switch } from "./components/ui";
import { cn } from "./lib/utils";
import { mockGet, mockPost } from "./lib/mock-api";
import { desktopBridge, hasDesktopBridge } from "./lib/native-api";

type Provider = "antigravity" | "xai";
type Section = "overview" | "accounts" | "proxy" | "routing" | "connectors" | "analytics" | "settings";

interface DashboardItem { ok: boolean; label: string; detail?: string; running?: boolean }
interface DeviceAuthorization { userCode: string; verificationURL: string }
interface DashboardOperation { running: boolean; name?: string; message?: string; error?: string; deviceAuthorization?: DeviceAuthorization }
interface DashboardStatus {
  version: string;
  gateway: DashboardItem;
  proxy: DashboardItem;
  tun: DashboardItem;
  zcode: DashboardItem;
  providerAccounts: { antigravity: number; xai: number };
  selectedProvider: Provider;
  models: string[];
  operation: DashboardOperation;
  updatedAt: string;
}
interface QuotaBucket { name: string; window: string; remainingPercent?: number; resetTime?: string }
interface QuotaGroup { name: string; buckets: QuotaBucket[] }
interface QuotaAccount { account: string; plan?: string; status: string; statusMessage?: string; groups?: QuotaGroup[]; credits?: { available: boolean; amount: number; creditType: string }; error?: string }
interface QuotaReport { fetchedAt?: string; provider: Provider; source: string; stale: boolean; accounts: QuotaAccount[]; warning?: string }
interface UsageSample { timestamp: string; model: string; outputTokens: number; reasoningTokens: number; totalTokens?: number; latencyMs: number; ttftMs: number; generationMs: number; outputTokensPerSecond: number; speedBasis: string }
interface UsageReport { provider: Provider; available: boolean; latest?: UsageSample; total: { requests: number; outputTokens: number; reasoningTokens: number; averageTokensPerSecond: number }; recent: UsageSample[]; warning?: string }
interface ManagerReport {
  version: string;
  accounts: Array<{ id: string; provider: Provider; label: string; plan?: string; status: string; updatedAt: string }>;
  proxy: { running: boolean; baseURL?: string; port?: number; protocols: Array<{ name: string; path: string; description: string }> };
  routing: { strategy: string; sessionAffinity: boolean; sessionAffinityTTL: string; requestRetry: number; credentialRetry: number; retryInterval: number; backgroundModel: string };
  settings: { autoRefreshMinutes: number; quotaWarningPercent: number; enableGrokModels: boolean; enableOtherModels: boolean; proxyURL: string; theme: string; liquidGlass: boolean; settingsPath: string };
  features: Array<{ id: string; name: string; description: string; available: boolean }>;
}
interface ConnectorResponse { provider: Provider; baseURL: string; model: string; connectors: Array<{ id: string; name: string; description: string; model: string; action?: string; snippets: Record<string, string> }> }
interface StartupInfo { version: string; autoSetup: boolean }

async function apiGet<T>(path: string): Promise<T> {
  if (!hasDesktopBridge()) return mockGet(path) as Promise<T>;
  return desktopBridge()!.apiGet<T>(path);
}

async function apiPost<T = unknown>(path: string, body: Record<string, unknown> = {}): Promise<T> {
  if (!hasDesktopBridge()) return mockPost(path, body) as Promise<T>;
  return desktopBridge()!.apiPost<T>(path, body);
}

function normalizeError(error: unknown) {
  if (error instanceof Error) return error.message;
  if (typeof error === "string") return error;
  return "发生未知错误";
}

function providerName(provider: Provider) {
  return provider === "xai" ? "Grok / xAI" : "Antigravity";
}

function formatNumber(value = 0) {
  return new Intl.NumberFormat("zh-CN").format(value);
}

function formatTime(value?: string) {
  if (!value) return "未提供";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" });
}

function allBuckets(quota?: QuotaReport) {
  return (quota?.accounts ?? []).flatMap((account) =>
    (account.groups ?? []).flatMap((group) => group.buckets.map((bucket) => ({ ...bucket, group: group.name, account: account.account }))),
  );
}

function quotaWindow(quota: QuotaReport | undefined, kind: "five" | "week") {
  const matches = allBuckets(quota).filter((bucket) => {
    const value = `${bucket.group} ${bucket.name} ${bucket.window}`.toLowerCase();
    return kind === "five"
      ? /5\s*(小时|hour|h)|5-hour/.test(value)
      : /周|week|7\s*(天|day)|7-day/.test(value);
  }).filter((bucket) => typeof bucket.remainingPercent === "number");
  return matches.sort((a, b) => (a.remainingPercent ?? 101) - (b.remainingPercent ?? 101))[0];
}

function WindowChrome() {
  const windowAction = (action: "minimize" | "maximize" | "hide") => {
    void desktopBridge()?.windowAction(action);
  };
  return (
    <div className="window-chrome">
      <div className="flex items-center gap-2.5 pointer-events-none">
        <img src={brandMark} alt="" className="size-6 rounded-lg" />
        <span className="text-xs font-medium tracking-wide text-slate-200">ZCode · Antigravity</span>
        <Badge tone="blue">Electron</Badge>
      </div>
      <div className="window-controls">
        <button aria-label="最小化" onClick={() => windowAction("minimize")}><Minimize2 /></button>
        <button aria-label="最大化" onClick={() => windowAction("maximize")}><Maximize2 /></button>
        <button aria-label="隐藏到任务栏" className="close" onClick={() => windowAction("hide")}><X /></button>
      </div>
    </div>
  );
}

const navigation: Array<{ id: Section; label: string; icon: LucideIcon }> = [
  { id: "overview", label: "总览", icon: Gauge },
  { id: "accounts", label: "账号管理", icon: Users },
  { id: "proxy", label: "API 代理", icon: Network },
  { id: "routing", label: "模型路由", icon: Boxes },
  { id: "connectors", label: "Agent 接入", icon: Bot },
  { id: "analytics", label: "用量统计", icon: BarChart3 },
  { id: "settings", label: "设置", icon: Settings2 },
];

function HorizontalNavigation({ section, setSection }: { section: Section; setSection: (value: Section) => void }) {
  return (
    <nav className="horizontal-nav" aria-label="控制中心功能">
      <div className="horizontal-nav-items">
        {navigation.map((item) => {
          const Icon = item.icon;
          return <button key={item.id} onClick={() => setSection(item.id)} className={cn("nav-item", section === item.id && "active")}><Icon /><span>{item.label}</span></button>;
        })}
      </div>
      <div className="nav-signature">
        <ShieldCheck />
        <span>Antigravity Tools 核心功能</span>
      </div>
    </nav>
  );
}

function ProviderTabs({ provider, counts, busy, grokEnabled, onSelect }: { provider: Provider; counts?: DashboardStatus["providerAccounts"]; busy: boolean; grokEnabled: boolean; onSelect: (provider: Provider) => void }) {
  const providers: Provider[] = grokEnabled ? ["antigravity", "xai"] : ["antigravity"];
  return (
    <div className="provider-tabs">
      {providers.map((item) => (
        <button key={item} disabled={busy} onClick={() => onSelect(item)} className={cn("provider-tab", provider === item && "active")}>
          {item === "antigravity" ? <Sparkles /> : <Zap />}
          <span>{providerName(item)}</span>
          <span className="count">{item === "antigravity" ? counts?.antigravity ?? 0 : counts?.xai ?? 0}</span>
        </button>
      ))}
    </div>
  );
}

function StatusStrip({ status }: { status?: DashboardStatus }) {
  const items = [
    ["TUN", status?.tun, Wifi], ["PROXY", status?.proxy, Cloud], ["BRIDGE", status?.gateway, Cpu], ["ZCODE", status?.zcode, Bot],
  ] as const;
  return (
    <div className="status-strip">
      {items.map(([name, item, Icon]) => <div key={name} className="status-cell"><div className="flex items-center gap-2"><span className={cn("status-dot", item?.ok && "online")} /><span className="status-name">{name}</span><Icon className="ml-auto size-3.5 text-slate-500" /></div><p>{item?.label ?? "正在读取"}</p><span title={item?.detail}>{item?.detail ?? "等待本机状态"}</span></div>)}
    </div>
  );
}

function QuotaHero({ quota, provider, warningPercent = 20 }: { quota?: QuotaReport; provider: Provider; warningPercent?: number }) {
  if (quota && quota.accounts.length === 0) {
    const Icon = provider === "xai" ? Zap : KeyRound;
    const loginNeeded = quota.warning?.includes("尚未登录") ?? false;
    const title = loginNeeded
      ? provider === "xai" ? "尚未登录 Grok / xAI" : "尚未登录 Antigravity"
      : `${providerName(provider)} 额度暂不可用`;
    return <div className="quota-empty md:col-span-2"><span className="quota-icon"><Icon /></span><div><h3>{title}</h3><p>{quota.warning || `请先完成 ${providerName(provider)} 授权`}</p></div></div>;
  }
  const five = quotaWindow(quota, "five");
  const week = quotaWindow(quota, "week");
  const cards = provider === "xai"
    ? [{ label: "共享额度", bucket: allBuckets(quota)[0], icon: Zap }]
    : [{ label: "当前 5 小时", bucket: five, icon: Activity }, { label: "本周额度", bucket: week, icon: CircleGauge }];
  return (
    <div className="grid gap-3 md:grid-cols-2">
      {cards.map(({ label, bucket, icon: Icon }) => {
        const value = bucket?.remainingPercent;
        const low = typeof value === "number" && value <= warningPercent;
        return <div className="quota-tile" key={label}><div className="mb-5 flex items-center justify-between"><div className="quota-icon"><Icon /></div><Badge tone={low ? "bad" : typeof value === "number" ? "good" : "neutral"}>{quota?.stale ? "缓存" : "实时"}</Badge></div><p className="text-xs text-slate-400">{label}</p><div className="mt-1 flex items-end gap-1"><span className="text-4xl font-semibold tracking-[-.05em] text-white">{typeof value === "number" ? Math.round(value) : "--"}</span><span className="mb-1 text-sm text-slate-500">%</span></div><div className="mt-4"><Progress value={value ?? 0} warning={low} /></div><p className="mt-2.5 truncate text-[11px] text-slate-500">重置 {formatTime(bucket?.resetTime)}</p></div>;
      })}
      {provider === "xai" && quota?.accounts[0]?.credits && <div className="quota-tile"><div className="mb-4 flex items-center justify-between"><div className="quota-icon"><KeyRound /></div><Badge tone="blue">Credits</Badge></div><p className="text-xs text-slate-400">可用点数</p><p className="mt-1 text-3xl font-semibold text-white">{quota.accounts[0].credits.amount}</p><p className="mt-3 text-[11px] text-slate-500">{quota.accounts[0].credits.creditType || "xAI"}</p></div>}
    </div>
  );
}

function ActionPanel({ busy, grokEnabled, onAction }: { busy: boolean; grokEnabled: boolean; onAction: (action: string) => void }) {
  const actions = [
    ["setup", "一键接入 ZCode", "启动网关并同步当前提供商", Sparkles, "primary"],
    ["login", "登录 Antigravity", "通过浏览器完成 Google OAuth", KeyRound, "secondary"],
    ["login-grok", "登录 Grok / xAI", "通过官方 xAI 设备授权", Zap, "secondary"],
    ["sync", "修复并重新同步", "校验网关和 ZCode Provider", RotateCcw, "secondary"],
    ["open-zcode", "打开 ZCode", "接入完成后开始使用", ExternalLink, "secondary"],
    ["stop", "停止本地网关", "保留账号和聊天记录", Power, "danger"],
  ] as const;
  const visibleActions = actions.filter(([id]) => id !== "login-grok" || grokEnabled);
  return <div className="space-y-2.5">{visibleActions.map(([id, title, description, Icon, variant]) => <button key={id} disabled={busy} onClick={() => onAction(id)} className={cn("action-row", variant === "primary" && "primary", variant === "danger" && "danger")}><span className="action-icon"><Icon /></span><span className="min-w-0 flex-1 text-left"><strong>{title}</strong><small>{description}</small></span><ChevronRight /></button>)}</div>;
}

function AuthenticationOverlay({ operation, provider, onCopy, onError }: { operation?: DashboardOperation; provider: Provider; onCopy: (value: string) => void; onError: (message: string) => void }) {
  if (!operation?.running || !["login", "login-grok", "setup"].includes(operation.name ?? "")) return null;
  const device = operation.deviceAuthorization;
  const grok = operation.name === "login-grok" || (operation.name === "setup" && provider === "xai");
  const openVerificationPage = async () => {
    if (!device) return;
    try {
      if (hasDesktopBridge()) await desktopBridge()!.openXaiVerificationURL(device.verificationURL);
      else window.open(device.verificationURL, "_blank", "noopener,noreferrer");
    } catch (error) {
      onError(`无法打开 xAI 授权页：${normalizeError(error)}`);
    }
  };
  return (
    <div className="auth-overlay" role="dialog" aria-modal="true" aria-labelledby="auth-title">
      <div className="auth-dialog glass-card">
        <div className="auth-glow" />
        <div className="auth-icon">{grok ? <Zap /> : <KeyRound />}</div>
        <p className="auth-eyebrow">SECURE DEVICE AUTHORIZATION</p>
        <h2 id="auth-title">{grok ? "Grok / xAI 设备授权" : "Antigravity 登录"}</h2>
        {grok && device ? (
          <>
            <p className="auth-description">请在已经打开的 xAI 官方页面中输入下方验证码。验证码只用于本次登录，不会保存到日志。</p>
            <label className="auth-code-label" htmlFor="xai-user-code">临时验证码</label>
            <input id="xai-user-code" className="auth-code" value={device.userCode} readOnly spellCheck={false} onFocus={(event) => event.currentTarget.select()} />
            <div className="auth-actions">
              <Button onClick={() => onCopy(device.userCode)}><Copy className="size-4" />复制验证码</Button>
              <Button variant="secondary" onClick={() => void openVerificationPage()}><ExternalLink className="size-4" />打开 xAI 授权页</Button>
            </div>
            <p className="auth-hint">授权后无需返回填写，软件会自动检测并完成登录。</p>
          </>
        ) : (
          <>
            <p className="auth-description">{grok ? "正在向 xAI 申请临时验证码，请稍候…" : "Google OAuth 已在浏览器中打开，请完成账号授权。软件正在等待登录结果。"}</p>
            <div className="auth-waiting"><LoaderCircle className="animate-spin" /><span>{operation.message || "正在等待浏览器授权…"}</span></div>
          </>
        )}
        {grok && device && <div className="auth-waiting compact"><LoaderCircle className="animate-spin" /><span>正在等待 xAI 确认…</span></div>}
      </div>
    </div>
  );
}

function Overview({ status, quota, usage, manager, provider, busy, onAction }: { status?: DashboardStatus; quota?: QuotaReport; usage?: UsageReport; manager?: ManagerReport; provider: Provider; busy: boolean; onAction: (action: string) => void }) {
  const latest = usage?.latest;
  return (
    <div className="content-grid">
      <Card className="min-h-[450px]">
        <CardHeader eyebrow={provider === "xai" ? "GROK USAGE" : "ANTIGRAVITY USAGE"} title={provider === "xai" ? "Grok 模型额度" : "Gemini 模型额度"} description={quota?.fetchedAt ? `上次额度刷新 ${formatTime(quota.fetchedAt)}` : "每 5 分钟自动刷新；切换提供商立即刷新"} action={<Badge tone={quota?.warning ? "warn" : "good"}>{quota?.warning ? "需注意" : "额度监控"}</Badge>} />
        <div className="p-5">
          <div className="overview-metrics">
            <Metric label="最近输出" value={latest ? formatNumber(latest.outputTokens) : "—"} suffix={latest ? "tok" : ""} />
            <Metric label="有效吞吐" value={latest ? latest.outputTokensPerSecond.toFixed(1) : "—"} suffix={latest ? "tok/s" : ""} />
            <Metric label="推理 Token" value={latest ? formatNumber(latest.reasoningTokens) : "—"} suffix={latest ? "tok" : ""} />
            <Metric label="本地累计输出" value={usage ? formatNumber(usage.total.outputTokens) : "—"} suffix={usage ? "tok" : ""} />
          </div>
          <QuotaHero quota={quota} provider={provider} warningPercent={manager?.settings.quotaWarningPercent} />
          <div className="mt-5 rounded-2xl border border-white/[.07] bg-slate-950/20 p-4"><div className="flex items-center justify-between"><span className="text-xs text-slate-400">当前模型</span><Badge tone="blue">{status?.models.length ?? 0} 个</Badge></div><p className="mt-2 line-clamp-2 text-sm leading-6 text-slate-200">{status?.models.length ? status.models.join("  ·  ") : "等待网关同步"}</p></div>
          {quota?.warning && <p className="mt-3 text-xs leading-5 text-amber-200/80">{quota.warning}</p>}
        </div>
      </Card>
      <Card><CardHeader eyebrow="LOCAL ACTIONS" title="接入控制" description={status?.operation.message || status?.operation.error || "所有操作均在本机完成"} action={busy ? <LoaderCircle className="size-4 animate-spin text-sky-300" /> : undefined} /><div className="p-5"><ActionPanel busy={busy} grokEnabled={manager?.settings.enableGrokModels ?? false} onAction={onAction} /></div></Card>
    </div>
  );
}

function Metric({ label, value, suffix }: { label: string; value: string; suffix: string }) {
  return <div className="px-4"><p className="text-[11px] text-slate-500">{label}</p><p className="mt-1 text-xl font-semibold tracking-tight text-white">{value} <span className="text-xs font-normal text-slate-500">{suffix}</span></p></div>;
}

function AccountsView({ manager, quota, provider }: { manager?: ManagerReport; quota?: QuotaReport; provider: Provider }) {
  const accounts = manager?.accounts.filter((account) => account.provider === provider) ?? [];
  return <Card><CardHeader eyebrow="ACCOUNT MANAGER" title={`${providerName(provider)} 账号`} description="仅显示脱敏账号信息，不在界面中暴露凭据" action={<Badge tone="blue">{accounts.length} 个账号</Badge>} /><div className="grid gap-3 p-5 md:grid-cols-2">{accounts.length ? accounts.map((account) => { const live = quota?.accounts.find((item) => item.account === account.label); return <div className="account-card" key={account.id}><div className="flex items-start justify-between"><div className="account-avatar"><Users /></div><Badge tone={account.status === "active" ? "good" : "warn"}>{account.status}</Badge></div><h3>{account.label}</h3><p>{account.plan || live?.plan || "未识别订阅"}</p><div className="mt-4 flex items-center gap-2 text-[11px] text-slate-500"><ShieldCheck className="size-3.5 text-emerald-300" /> 当前用户加密存储 · {formatTime(account.updatedAt)}</div></div>; }) : <Empty icon={Users} title="尚未发现账号" text={`请先登录 ${providerName(provider)}`} />}</div></Card>;
}

function ProxyView({ manager, status }: { manager?: ManagerReport; status?: DashboardStatus }) {
  return <div className="grid gap-4 md:grid-cols-[1.1fr_.9fr]"><Card><CardHeader eyebrow="LOCAL GATEWAY" title="本地多协议代理" description={manager?.proxy.baseURL || "网关未启动"} action={<Badge tone={manager?.proxy.running ? "good" : "bad"}>{manager?.proxy.running ? "在线" : "离线"}</Badge>} /><div className="grid gap-3 p-5">{manager?.proxy.protocols.map((item) => <div className="protocol-row" key={item.name}><div className="protocol-icon"><Network /></div><div><h3>{item.name}</h3><p>{item.description}</p></div><code>{item.path}</code></div>)}</div></Card><Card><CardHeader eyebrow="NETWORK" title="连接诊断" description="本机回环地址，不开放到局域网" /><div className="space-y-3 p-5">{[["TUN（可选）", status?.tun], ["网络出口", status?.proxy], ["网关", status?.gateway], ["ZCode", status?.zcode]].map(([label, item]) => <div className="diagnostic-row" key={label as string}><span className={cn("status-dot", (item as DashboardItem)?.ok && "online")} /><div><p>{label as string}</p><small>{(item as DashboardItem)?.label ?? "读取中"}</small></div></div>)}</div></Card></div>;
}

function RoutingView({ manager, saveSetting, saving }: { manager?: ManagerReport; saveSetting: (body: Record<string, unknown>) => void; saving: boolean }) {
  const routes = [{ id: "round-robin", title: "轮询", text: "账号之间平均分配请求" }, { id: "weighted", title: "加权", text: "优先使用健康和高额度账号" }, { id: "fill-first", title: "填满优先", text: "当前账号用尽后自动切换" }];
  return <div className="grid gap-4 md:grid-cols-[1.15fr_.85fr]"><Card><CardHeader eyebrow="ROUTING" title="模型路由策略" description="保存后由本地网关即时应用" action={saving ? <LoaderCircle className="size-4 animate-spin" /> : undefined} /><div className="grid gap-3 p-5">{routes.map((route) => <button key={route.id} disabled={saving} onClick={() => saveSetting({ routingStrategy: route.id })} className={cn("route-card", manager?.routing.strategy === route.id && "active")}><span className="route-radio">{manager?.routing.strategy === route.id && <Check />}</span><div><h3>{route.title}</h3><p>{route.text}</p></div></button>)}</div></Card><Card><CardHeader eyebrow="RECOVERY" title="自动自愈" description="控制重试和会话稳定性" /><div className="space-y-3 p-5"><Switch checked={manager?.routing.sessionAffinity ?? false} onChange={(value) => saveSetting({ sessionAffinity: value })} label="会话亲和" /><InfoLine label="请求重试" value={`${manager?.routing.requestRetry ?? 0} 次`} /><InfoLine label="凭据轮换" value={`${manager?.routing.credentialRetry ?? 0} 次`} /><InfoLine label="最大重试间隔" value={`${manager?.routing.retryInterval ?? 0} 秒`} /><InfoLine label="后台模型" value={manager?.routing.backgroundModel || "自动"} /></div></Card></div>;
}

function ConnectorsView({ connectors, onCopy, onAction, busy }: { connectors?: ConnectorResponse; onCopy: (value: string) => void; onAction: (action: string) => void; busy: boolean }) {
  return <Card><CardHeader eyebrow="AGENT CONNECTORS" title="接入更多 Agent 程序" description={connectors ? `当前模型 ${connectors.model} · ${connectors.baseURL}` : "请先启动本地网关"} /><div className="grid gap-3 p-5 md:grid-cols-2">{connectors?.connectors.length ? connectors.connectors.map((connector) => <div className="connector-card" key={connector.id}><div className="flex items-center justify-between"><div className="connector-icon"><Bot /></div><Badge tone="blue">{connector.model}</Badge></div><h3>{connector.name}</h3><p>{connector.description}</p>{connector.action && <Button className="mt-4 w-full" variant="primary" disabled={busy} onClick={() => onAction(connector.action!)}><Zap className="size-4" />一键接入</Button>}<div className="mt-4 space-y-2">{Object.entries(connector.snippets).map(([name, snippet]) => <button onClick={() => onCopy(snippet)} className="snippet-row" key={name}><span>{name}</span><Copy /></button>)}</div></div>) : <Empty icon={Bot} title="等待网关" text="执行一键接入后生成当前提供商配置" />}</div></Card>;
}

function AnalyticsView({ usage }: { usage?: UsageReport }) {
  const maxSpeed = Math.max(1, ...(usage?.recent.map((item) => item.outputTokensPerSecond) ?? [1]));
  return <div className="space-y-4"><div className="grid gap-3 md:grid-cols-4"><StatCard icon={Zap} label="输出 Token" value={formatNumber(usage?.total.outputTokens)} /><StatCard icon={Sparkles} label="推理 Token" value={formatNumber(usage?.total.reasoningTokens)} /><StatCard icon={Gauge} label="平均速度" value={`${(usage?.total.averageTokensPerSecond ?? 0).toFixed(1)} tok/s`} /><StatCard icon={Activity} label="请求数量" value={formatNumber(usage?.total.requests)} /></div><Card><CardHeader eyebrow="THROUGHPUT" title="Token 输出速度" description="按生成阶段耗时计算，不把首 Token 等待时间混入速度" /><div className="p-5"><div className="chart-grid">{(usage?.recent.slice(-18) ?? []).map((sample, index) => <div className="chart-column" key={`${sample.timestamp}-${index}`} title={`${sample.model}: ${sample.outputTokensPerSecond.toFixed(1)} tok/s`}><span style={{ height: `${Math.max(5, sample.outputTokensPerSecond / maxSpeed * 100)}%` }} /></div>)}</div>{!usage?.recent.length && <Empty icon={BarChart3} title="暂无 Token 记录" text="通过本地网关调用模型后会自动统计" />}</div></Card><Card><CardHeader title="最近请求" /><div className="divide-y divide-white/[.06] px-5 pb-4">{usage?.recent.slice(-8).reverse().map((sample, index) => <div className="usage-row" key={`${sample.timestamp}-${index}`}><div className="min-w-0"><p className="truncate">{sample.model}</p><small>{formatTime(sample.timestamp)} · {sample.latencyMs} ms</small></div><span>{formatNumber(sample.outputTokens)} tok</span><strong>{sample.outputTokensPerSecond.toFixed(1)} tok/s</strong></div>)}</div></Card></div>;
}

function SettingsView({ manager, saveSetting, saving, syncing, onSync }: { manager?: ManagerReport; saveSetting: (body: Record<string, unknown>) => void; saving: boolean; syncing: boolean; onSync: () => void }) {
  return <div className="grid gap-4 md:grid-cols-[1fr_.85fr]"><Card><CardHeader eyebrow="MODEL ACCESS" title="模型与界面设置" description="默认仅暴露 Gemini；扩展模型需主动开启并重新同步" action={saving ? <LoaderCircle className="size-4 animate-spin" /> : undefined} /><div className="space-y-3 p-5"><Switch checked={manager?.settings.enableGrokModels ?? false} onChange={(value) => saveSetting({ enableGrokModels: value })} label="Grok 模型" /><Switch checked={manager?.settings.enableOtherModels ?? false} onChange={(value) => saveSetting({ enableOtherModels: value })} label="其他 AI 文本模型（Claude / GPT 等）" /><Button className="w-full" variant="primary" disabled={saving || syncing} onClick={onSync}><RefreshCw className={cn("size-4", syncing && "animate-spin")} />应用模型开关</Button><Switch checked={manager?.settings.liquidGlass ?? true} onChange={(value) => saveSetting({ liquidGlass: value })} label="白色液态玻璃背景" /><div className="setting-row"><div><p>额度自动刷新</p><small>默认每 5 分钟，不影响 5 秒状态探测</small></div><div className="segmented">{[5, 10].map((value) => <button className={cn(manager?.settings.autoRefreshMinutes === value && "active")} onClick={() => saveSetting({ autoRefreshMinutes: value })} key={value}>{value} 分钟</button>)}</div></div><InfoLine label="网络模式" value={manager?.settings.proxyURL || "自动 v2rayN / 系统代理 / 直连"} /><InfoLine label="低额度警告" value={`${manager?.settings.quotaWarningPercent ?? 20}%`} /><InfoLine label="设置文件" value={manager?.settings.settingsPath || "当前用户目录"} /></div></Card><Card><CardHeader eyebrow="FEATURES" title="Antigravity Tools 能力" description="当前原生管理核心提供的功能" /><div className="space-y-2 p-5">{manager?.features.map((feature) => <div className="feature-row" key={feature.id}><span className={cn("feature-check", feature.available && "active")}>{feature.available && <Check />}</span><div><p>{feature.name}</p><small>{feature.description}</small></div></div>)}</div></Card></div>;
}

function InfoLine({ label, value }: { label: string; value: string }) { return <div className="info-line"><span>{label}</span><strong title={value}>{value}</strong></div>; }
function Empty({ icon: Icon, title, text }: { icon: LucideIcon; title: string; text: string }) { return <div className="empty-state"><Icon /><h3>{title}</h3><p>{text}</p></div>; }
function StatCard({ icon: Icon, label, value }: { icon: LucideIcon; label: string; value: string }) { return <Card className="p-4"><div className="mb-3 flex items-center justify-between"><span className="quota-icon !size-8"><Icon /></span><Activity className="size-3.5 text-slate-600" /></div><p className="text-[11px] text-slate-500">{label}</p><p className="mt-1 text-xl font-semibold text-white">{value}</p></Card>; }

function WidgetGauge({ label, bucket, icon: Icon }: { label: string; bucket?: QuotaBucket; icon: LucideIcon }) {
  const value = bucket?.remainingPercent;
  return (
    <div className="widget-gauge">
      <div className="widget-gauge-header"><span><Icon />{label}</span><strong>{typeof value === "number" ? `${Math.round(value)}%` : "--"}</strong></div>
      <Progress value={value ?? 0} warning={typeof value === "number" && value <= 20} />
      <p>重置 {formatTime(bucket?.resetTime)}</p>
    </div>
  );
}

function TrayWidget() {
  const [provider, setProvider] = useState<Provider>("antigravity");
  const [status, setStatus] = useState<DashboardStatus>();
  const [quota, setQuota] = useState<QuotaReport>();
  const [usage, setUsage] = useState<UsageReport>();
  const [manager, setManager] = useState<ManagerReport>();
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState("正在读取当前额度…");
  const refreshing = useRef(false);
  const quotaRef = useRef<QuotaReport | undefined>(undefined);

  const refresh = useCallback(async (forceQuota = false, requestedProvider?: Provider) => {
    if (refreshing.current) return;
    refreshing.current = true;
    setLoading(true);
    try {
      const [nextStatus, nextManager] = await Promise.all([
        apiGet<DashboardStatus>("/api/status"),
        apiGet<ManagerReport>("/api/manager"),
      ]);
      const target = requestedProvider ?? nextStatus.selectedProvider;
      setStatus(nextStatus);
      setManager(nextManager);
      setProvider(target);
      const accountCount = target === "xai" ? nextStatus.providerAccounts.xai : nextStatus.providerAccounts.antigravity;
      const nextUsage = await apiGet<UsageReport>(`/api/usage?provider=${target}`).catch(() => undefined);
      setUsage(nextUsage);
      if (accountCount === 0) {
        quotaRef.current = undefined;
        setQuota(undefined);
        setMessage(`尚未登录 ${providerName(target)}`);
      } else if (!nextStatus.gateway.ok) {
        if (!quotaRef.current || quotaRef.current.provider !== target) setQuota(undefined);
        setMessage("本地网关未启动，请打开控制中心接入 ZCode");
      } else if (forceQuota || !quotaRef.current || quotaRef.current.provider !== target) {
        const nextQuota = await apiGet<QuotaReport>(`/api/quota?provider=${target}`);
        quotaRef.current = nextQuota;
        setQuota(nextQuota);
        setMessage(nextQuota.warning || `已于 ${formatTime(nextQuota.fetchedAt)} 更新`);
      }
    } catch (error) {
      setMessage(`额度刷新失败：${normalizeError(error)}`);
    } finally {
      setLoading(false);
      refreshing.current = false;
    }
  }, []);

  const selectProvider = useCallback(async (next: Provider) => {
    if (next === provider || refreshing.current) return;
    setProvider(next);
    quotaRef.current = undefined;
    setQuota(undefined);
    setMessage(`正在切换到 ${providerName(next)}…`);
    try {
      await apiPost("/api/provider", { provider: next });
      await refresh(true, next);
    } catch (error) {
      setMessage(`切换失败：${normalizeError(error)}`);
    }
  }, [provider, refresh]);

  useEffect(() => {
    void refresh(true);
    const statusTimer = window.setInterval(() => void refresh(false), 5_000);
    const quotaTimer = window.setInterval(() => void refresh(true), 5 * 60_000);
    const nativeRefresh = () => void refresh(true);
    window.addEventListener("zcode:refresh", nativeRefresh);
    desktopBridge()?.onRefresh(nativeRefresh);
    return () => {
      clearInterval(statusTimer);
      clearInterval(quotaTimer);
      window.removeEventListener("zcode:refresh", nativeRefresh);
      desktopBridge()?.removeRefreshListener();
    };
  }, [refresh]);

  const five = quotaWindow(quota, "five");
  const week = quotaWindow(quota, "week");
  const shared = allBuckets(quota).find((bucket) => typeof bucket.remainingPercent === "number");
  const account = quota?.accounts[0];
  const openMain = () => { void desktopBridge()?.showMainWindow(); };
  const hide = () => { void desktopBridge()?.windowAction("hide"); };

  return (
    <div className="widget-shell">
      <div className="widget-orb widget-orb-one" /><div className="widget-orb widget-orb-two" /><div className="noise-layer" />
      <header className="widget-header">
        <div className="widget-brand"><span><Sparkles /></span><div><strong>ZCode 当前额度</strong><small>{status?.gateway.ok ? "本地网关在线" : "本地安全核心"}</small></div></div>
        <div className="widget-header-actions">
          <button aria-label="刷新额度" disabled={loading} onClick={() => void refresh(true)}><RefreshCw className={cn(loading && "animate-spin")} /></button>
          <button aria-label="关闭小组件" onClick={hide}><X /></button>
        </div>
      </header>
      <div className="widget-provider-tabs">
        {(["antigravity", ...(manager?.settings.enableGrokModels ? ["xai" as Provider] : [])] as Provider[]).map((item) => <button key={item} aria-label={`切换到 ${providerName(item)}`} className={cn(provider === item && "active")} onClick={() => void selectProvider(item)}><span>{item === "xai" ? <Zap /> : <Sparkles />}{providerName(item)}</span><em>{item === "xai" ? status?.providerAccounts.xai ?? 0 : status?.providerAccounts.antigravity ?? 0}</em></button>)}
      </div>
      <section className="widget-content">
        <div className="widget-account"><span className={cn("status-dot", status?.gateway.ok && "online")} /><div><strong>{account?.account || providerName(provider)}</strong><small>{account?.plan || message}</small></div><Badge tone={quota?.stale ? "warn" : quota ? "good" : "neutral"}>{quota?.stale ? "缓存" : quota ? "实时" : "等待"}</Badge></div>
        <div className="widget-gauges">
          {provider === "xai" ? <WidgetGauge label="共享额度" bucket={shared} icon={Zap} /> : <><WidgetGauge label="当前 5 小时" bucket={five} icon={Activity} /><WidgetGauge label="本周额度" bucket={week} icon={CircleGauge} /></>}
          {provider === "xai" && <div className="widget-credit"><span>可用 Credits</span><strong>{account?.credits?.available ? account.credits.amount : "--"}</strong><small>{account?.credits?.creditType || "xAI"}</small></div>}
        </div>
        <div className="widget-stats"><div><span>累计输出</span><strong>{formatNumber(usage?.total.outputTokens)} <small>tok</small></strong></div><div><span>当前速度</span><strong>{(usage?.latest?.outputTokensPerSecond ?? 0).toFixed(1)} <small>tok/s</small></strong></div></div>
      </section>
      <footer className="widget-footer"><button onClick={openMain}><ExternalLink />打开控制中心</button><span>{message}</span></footer>
    </div>
  );
}

function ControlCenterApp() {
  const [section, setSection] = useState<Section>("overview");
  const [provider, setProvider] = useState<Provider>("antigravity");
  const [status, setStatus] = useState<DashboardStatus>();
  const [quota, setQuota] = useState<QuotaReport>();
  const [usage, setUsage] = useState<UsageReport>();
  const [manager, setManager] = useState<ManagerReport>();
  const [connectors, setConnectors] = useState<ConnectorResponse>();
  const [version, setVersion] = useState("1.0.0");
  const [busy, setBusy] = useState(false);
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState<{ text: string; error?: boolean }>();
  const refreshing = useRef(false);
  const pendingRefresh = useRef<{ forceQuota: boolean; provider?: Provider } | undefined>(undefined);
  const refreshRef = useRef<((forceQuota?: boolean, requestedProvider?: Provider) => Promise<void>) | undefined>(undefined);
  const initialized = useRef(false);
  const scrollIdleTimer = useRef<number | undefined>(undefined);

  const refresh = useCallback(async (forceQuota = false, requestedProvider?: Provider) => {
    if (refreshing.current) {
      const pending = pendingRefresh.current;
      pendingRefresh.current = {
        forceQuota: forceQuota || pending?.forceQuota || false,
        provider: requestedProvider ?? pending?.provider,
      };
      return;
    }
    refreshing.current = true;
    const target = requestedProvider ?? provider;
    try {
      const [nextStatus, nextUsage, nextManager] = await Promise.all([
        apiGet<DashboardStatus>("/api/status"),
        apiGet<UsageReport>(`/api/usage?provider=${target}`),
        apiGet<ManagerReport>("/api/manager"),
      ]);
      setStatus(nextStatus); setUsage(nextUsage); setManager(nextManager);
      if (!requestedProvider) setProvider(nextStatus.selectedProvider);
      const gatewayRecovered = nextStatus.gateway.ok
        && quota?.provider === target
        && quota.warning?.includes("本地网关尚未运行");
      if (forceQuota || !quota || quota.provider !== target || gatewayRecovered) {
        const accountCount = target === "xai" ? nextStatus.providerAccounts.xai : nextStatus.providerAccounts.antigravity;
        if (accountCount === 0) {
          setQuota({
            fetchedAt: nextStatus.updatedAt,
            provider: target,
            source: "本地账号状态",
            stale: false,
            accounts: [],
            warning: target === "xai"
              ? "尚未登录 Grok / xAI，请点击右侧“登录 Grok / xAI”获取验证码"
              : "尚未登录 Antigravity，请点击右侧“登录 Antigravity”完成授权",
          });
        } else if (!nextStatus.gateway.ok) {
          setQuota((current) => current?.provider === target
            ? { ...current, stale: true, warning: "本地网关尚未运行，请点击“一键接入 ZCode”后刷新额度" }
            : { provider: target, source: "本地网关状态", stale: true, accounts: [], warning: "本地网关尚未运行，请点击“一键接入 ZCode”后刷新额度" });
        } else {
          try {
            setQuota(await apiGet<QuotaReport>(`/api/quota?provider=${target}`));
          } catch (error) {
            const message = normalizeError(error);
            setQuota((current) => current?.provider === target
              ? { ...current, stale: true, warning: `实时额度暂不可用：${message}` }
              : { provider: target, source: "额度接口", stale: true, accounts: [], warning: `实时额度暂不可用：${message}` });
            if (forceQuota) setNotice({ text: `${providerName(target)} 额度刷新失败：${message}`, error: true });
          }
        }
      }
      try { setConnectors(await apiGet<ConnectorResponse>("/api/connectors")); } catch { setConnectors(undefined); }
    } catch (error) {
      setNotice({ text: `状态刷新失败：${normalizeError(error)}`, error: true });
    } finally {
      refreshing.current = false;
      const pending = pendingRefresh.current;
      pendingRefresh.current = undefined;
      if (pending) {
        window.setTimeout(() => void refreshRef.current?.(pending.forceQuota, pending.provider), 0);
      }
    }
  }, [provider, quota]);
  refreshRef.current = refresh;

  const runAction = useCallback(async (action: string) => {
    if (busy) return;
    setBusy(true); setNotice({ text: action === "setup" ? "正在启动网关并接入 ZCode…" : "本机操作正在进行…" });
    try {
      await apiPost("/api/action", { action });
      const interactiveLogin = action === "login" || action === "login-grok" || action === "setup";
      const deadline = Date.now() + (interactiveLogin ? 10 * 60_000 : 120_000);
      let completed = false;
      while (Date.now() < deadline) {
        await new Promise((resolve) => setTimeout(resolve, 900));
        const next = await apiGet<DashboardStatus>("/api/status");
        setStatus(next);
        if (!next.operation.running) {
          if (next.operation.error) throw new Error(next.operation.error);
          completed = true;
          break;
        }
      }
      if (!completed) throw new Error("授权等待超时，请重新发起登录");
      setNotice({ text: action === "setup" ? "ZCode 已接入并启动" : "操作已完成" });
      await refresh(true);
    } catch (error) { setNotice({ text: normalizeError(error), error: true }); }
    finally { setBusy(false); }
  }, [busy, refresh]);

  const selectProvider = useCallback(async (next: Provider) => {
    if (next === provider || busy) return;
    setBusy(true); setNotice({ text: `正在切换到 ${providerName(next)}…` });
    try { await apiPost("/api/provider", { provider: next }); setProvider(next); setQuota(undefined); await refresh(true, next); setNotice({ text: `已切换到 ${providerName(next)}` }); }
    catch (error) { setNotice({ text: `切换失败：${normalizeError(error)}`, error: true }); }
    finally { setBusy(false); }
  }, [busy, provider, refresh]);

  const saveSetting = useCallback(async (body: Record<string, unknown>) => {
    if (saving) return;
    setSaving(true);
    try {
      setManager(await apiPost<ManagerReport>("/api/manager/settings", body));
      if (body.enableGrokModels === false) {
        setProvider("antigravity");
        setQuota(undefined);
      }
      setNotice({ text: "设置已保存" });
    }
    catch (error) { setNotice({ text: `保存失败：${normalizeError(error)}`, error: true }); }
    finally { setSaving(false); }
  }, [saving]);

  const copy = useCallback(async (value: string, message = "Agent 配置已复制") => {
    try {
      if (hasDesktopBridge()) await desktopBridge()!.writeClipboard(value);
      else await navigator.clipboard.writeText(value);
      setNotice({ text: message });
    }
    catch { setNotice({ text: "无法写入剪贴板", error: true }); }
  }, []);

  useEffect(() => {
    if (initialized.current) return;
    initialized.current = true;
    void (async () => {
      try {
        const startup: StartupInfo = hasDesktopBridge() ? await desktopBridge()!.startupInfo() : { version: "1.0.0", autoSetup: false };
        setVersion(startup.version);
        await refresh(true);
        if (startup.autoSetup) await runAction("setup");
      } catch (error) { setNotice({ text: normalizeError(error), error: true }); }
    })();
  }, [refresh, runAction]);

  useEffect(() => {
    const statusTimer = window.setInterval(() => void refresh(false), 5_000);
    const quotaMinutes = Math.max(1, manager?.settings.autoRefreshMinutes ?? 5);
    const quotaTimer = window.setInterval(() => void refresh(true), quotaMinutes * 60_000);
    const heartbeat = window.setInterval(() => void apiPost("/api/heartbeat").catch(() => undefined), 15_000);
    const trayRefresh = () => void refresh(true);
    window.addEventListener("zcode:refresh", trayRefresh);
    desktopBridge()?.onRefresh(trayRefresh);
    return () => {
      clearInterval(statusTimer);
      clearInterval(quotaTimer);
      clearInterval(heartbeat);
      window.removeEventListener("zcode:refresh", trayRefresh);
      desktopBridge()?.removeRefreshListener();
    };
  }, [manager?.settings.autoRefreshMinutes, refresh]);

  useEffect(() => {
    if (!quota && !usage) return;
    if (hasDesktopBridge()) void desktopBridge()!.updateTraySummary({ provider, fiveHour: quotaWindow(quota, "five")?.remainingPercent, week: quotaWindow(quota, "week")?.remainingPercent, tokensPerSecond: usage?.latest?.outputTokensPerSecond });
  }, [provider, quota, usage]);

  useEffect(() => {
    if (!notice) return;
    const timer = window.setTimeout(() => setNotice(undefined), notice.error ? 7000 : 3200);
    return () => clearTimeout(timer);
  }, [notice]);

  useEffect(() => () => {
    if (scrollIdleTimer.current) window.clearTimeout(scrollIdleTimer.current);
    document.documentElement.classList.remove("window-scrolling");
  }, []);

  const handleMainScroll = useCallback(() => {
    document.documentElement.classList.add("window-scrolling");
    if (scrollIdleTimer.current) window.clearTimeout(scrollIdleTimer.current);
    scrollIdleTimer.current = window.setTimeout(() => {
      document.documentElement.classList.remove("window-scrolling");
      scrollIdleTimer.current = undefined;
    }, 140);
  }, []);

  const subtitle = useMemo(() => status?.operation.running ? status.operation.message || "本机操作正在进行" : status?.gateway.ok ? "本地安全核心在线" : "请选择提供商并执行一键接入", [status]);
  const content = section === "overview" ? <Overview {...{ status, quota, usage, manager, provider, busy, onAction: runAction }} />
    : section === "accounts" ? <AccountsView {...{ manager, quota, provider }} />
    : section === "proxy" ? <ProxyView {...{ manager, status }} />
    : section === "routing" ? <RoutingView {...{ manager, saveSetting, saving }} />
    : section === "connectors" ? <ConnectorsView connectors={connectors} onCopy={copy} onAction={runAction} busy={busy} />
    : section === "analytics" ? <AnalyticsView usage={usage} />
    : <SettingsView manager={manager} saveSetting={saveSetting} saving={saving} syncing={busy} onSync={() => void runAction("sync")} />;

  return (
    <div className={cn("app-shell", manager?.settings.liquidGlass === false && "solid-mode")}>
      <div className="liquid-orb orb-one" /><div className="liquid-orb orb-two" /><div className="noise-layer" />
      <WindowChrome />
      <div className="app-body">
        <main className="main-panel" onScroll={handleMainScroll}>
          <header className="mac-brand-bar">
            <div className="mac-brand-identity"><img className="mac-brand-logo" src={brandMark} alt="ZCode Antigravity" /><div><h1>ZCode Antigravity</h1><p>Updated {status?.updatedAt ? formatTime(status.updatedAt) : "等待首次刷新"}</p></div></div>
            <div className="mac-brand-actions"><Badge tone={status?.gateway.ok ? "good" : "neutral"}>{status?.gateway.ok ? "本地在线" : "Local only"}</Badge><Button size="icon" aria-label="刷新额度" onClick={() => void refresh(true)} disabled={busy}><RefreshCw className={cn("size-4", refreshing.current && "animate-spin")} /></Button></div>
          </header>
          <div className="mac-meta-row"><div><span className={cn("status-dot", status?.gateway.ok && "online")} /><strong>{providerName(provider)}</strong><span>· {provider === "xai" ? status?.providerAccounts.xai ?? 0 : status?.providerAccounts.antigravity ?? 0} 个账号</span></div><div><Activity /><span>额度每 {manager?.settings.autoRefreshMinutes ?? 5} 分钟 · Token 每 5 秒</span></div></div>
          <HorizontalNavigation section={section} setSection={setSection} />
          <ProviderTabs provider={provider} counts={status?.providerAccounts} busy={busy} grokEnabled={manager?.settings.enableGrokModels ?? false} onSelect={(value) => void selectProvider(value)} />
          <StatusStrip status={status} />
          <div className="page-content">{content}</div>
          <footer className="mac-app-footer"><span>127.0.0.1 · 当前用户密钥</span><span>{subtitle} · v{version}</span></footer>
        </main>
      </div>
      <AuthenticationOverlay operation={status?.operation} provider={provider} onCopy={(value) => void copy(value, "xAI 验证码已复制")} onError={(text) => setNotice({ text, error: true })} />
      {notice && <div className={cn("toast", notice.error && "error")}><span className={cn("status-dot online", notice.error && "!bg-rose-400 !shadow-[0_0_12px_rgba(251,113,133,.8)]")} /><p>{notice.text}</p></div>}
    </div>
  );
}

export default function App() {
  const widgetMode = new URLSearchParams(window.location.search).get("view") === "widget";
  return widgetMode ? <TrayWidget /> : <ControlCenterApp />;
}
