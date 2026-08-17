#![windows_subsystem = "windows"]

use serde::Deserialize;
use serde_json::json;
use std::ffi::c_void;
use std::io::{BufRead, BufReader};
use std::os::windows::process::CommandExt;
use std::process::{Child, ChildStdin, Command, Stdio};
use std::ptr::{null, null_mut};
use std::sync::{Mutex, OnceLock};
use std::thread;
use std::time::{Duration, Instant};
use windows_sys::Win32::Foundation::*;
use windows_sys::Win32::Graphics::Gdi::*;
use windows_sys::Win32::System::DataExchange::*;
use windows_sys::Win32::System::LibraryLoader::*;
use windows_sys::Win32::System::Memory::*;
use windows_sys::Win32::System::Threading::CREATE_NO_WINDOW;
use windows_sys::Win32::UI::Controls::*;
use windows_sys::Win32::UI::HiDpi::*;
use windows_sys::Win32::UI::Shell::*;
use windows_sys::Win32::UI::WindowsAndMessaging::*;

const VERSION: &str = "0.4.0-test";
const SS_LEFT: u32 = 0;
const CF_UNICODETEXT_VALUE: u32 = 13;
const WM_REFRESH_READY: u32 = WM_APP + 1;
const WM_TRAY: u32 = WM_APP + 20;
const TIMER_REFRESH: usize = 1;

const ID_PROVIDER_ANTIGRAVITY: i32 = 100;
const ID_PROVIDER_GROK: i32 = 101;
const ID_REFRESH: i32 = 102;
const ID_SETUP: i32 = 200;
const ID_LOGIN_ANTIGRAVITY: i32 = 201;
const ID_LOGIN_GROK: i32 = 202;
const ID_SYNC: i32 = 203;
const ID_OPEN_ZCODE: i32 = 204;
const ID_STOP: i32 = 205;
const ID_COPY_CONNECTORS: i32 = 206;
const ID_TRAY_OPEN: usize = 300;
const ID_TRAY_REFRESH: usize = 301;
const ID_TRAY_ANTIGRAVITY: usize = 302;
const ID_TRAY_GROK: usize = 303;
const ID_TRAY_QUIT: usize = 304;

static STATE: OnceLock<Mutex<AppState>> = OnceLock::new();

#[derive(Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct NativeConnection {
    base_url: String,
    session: String,
}

struct NativeHost {
    child: Child,
    stdin: Option<ChildStdin>,
    connection: NativeConnection,
}

impl NativeHost {
    fn start() -> Result<Self, String> {
        let exe = std::env::current_exe().map_err(|error| format!("无法定位程序目录：{error}"))?;
        let directory = exe.parent().ok_or_else(|| "无法定位程序目录".to_string())?;
        let core = directory.join("ZCode-Antigravity.exe");
        if !core.is_file() {
            return Err(format!("缺少后台核心：{}", core.display()));
        }
        let mut child = Command::new(&core)
            .arg("native-host")
            .current_dir(directory)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::null())
            .creation_flags(CREATE_NO_WINDOW)
            .spawn()
            .map_err(|error| format!("无法启动后台核心：{error}"))?;
        let stdin = child.stdin.take();
        let stdout = child
            .stdout
            .take()
            .ok_or_else(|| "无法读取后台连接信息".to_string())?;
        let mut reader = BufReader::new(stdout);
        let mut line = String::new();
        reader
            .read_line(&mut line)
            .map_err(|error| format!("读取后台连接信息失败：{error}"))?;
        let connection: NativeConnection = serde_json::from_str(line.trim())
            .map_err(|error| format!("后台连接信息无效：{error}"))?;
        thread::spawn(move || {
            let mut sink = String::new();
            while reader.read_line(&mut sink).unwrap_or(0) > 0 {
                sink.clear();
            }
        });
        Ok(Self {
            child,
            stdin,
            connection,
        })
    }

    fn shutdown(&mut self) {
        self.stdin.take();
        let deadline = Instant::now() + Duration::from_secs(2);
        while Instant::now() < deadline {
            if self.child.try_wait().ok().flatten().is_some() {
                return;
            }
            thread::sleep(Duration::from_millis(60));
        }
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

#[derive(Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct DashboardItem {
    ok: bool,
    label: String,
    detail: Option<String>,
}

#[derive(Clone, Default, Deserialize)]
struct ProviderAccounts {
    antigravity: i32,
    xai: i32,
}

#[derive(Clone, Default, Deserialize)]
struct OperationState {
    running: bool,
    message: Option<String>,
    error: Option<String>,
}

#[derive(Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct DashboardStatus {
    gateway: DashboardItem,
    proxy: DashboardItem,
    tun: DashboardItem,
    zcode: DashboardItem,
    provider_accounts: ProviderAccounts,
    selected_provider: String,
    models: Vec<String>,
    operation: OperationState,
}

#[derive(Clone, Default, Deserialize)]
struct QuotaReport {
    accounts: Vec<QuotaAccount>,
    warning: Option<String>,
}

#[derive(Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct QuotaAccount {
    account: String,
    plan: Option<String>,
    status: String,
    status_message: Option<String>,
    groups: Option<Vec<QuotaGroup>>,
    credits: Option<CreditInfo>,
    error: Option<String>,
}

#[derive(Clone, Default, Deserialize)]
struct QuotaGroup {
    name: String,
    buckets: Vec<QuotaBucket>,
}

#[derive(Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct QuotaBucket {
    name: String,
    remaining_percent: Option<f64>,
    reset_time: Option<String>,
}

#[derive(Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct CreditInfo {
    available: bool,
    amount: f64,
    credit_type: String,
}

#[derive(Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ConnectorResponse {
    model: String,
    base_url: String,
    connectors: Vec<AgentConnector>,
}

#[derive(Clone, Default, Deserialize)]
struct AgentConnector {
    name: String,
    description: String,
    snippets: std::collections::BTreeMap<String, String>,
}

struct RefreshResult {
    status: Result<DashboardStatus, String>,
    quota: Result<QuotaReport, String>,
    connectors: Result<ConnectorResponse, String>,
}

#[derive(Default)]
struct Controls {
    subtitle: isize,
    provider_antigravity: isize,
    provider_grok: isize,
    refresh: isize,
    status_tun: isize,
    status_proxy: isize,
    status_bridge: isize,
    status_zcode: isize,
    quota_title: isize,
    quota_body: isize,
    quota_progress: isize,
    models: isize,
    action_header: isize,
    setup: isize,
    login_antigravity: isize,
    login_grok: isize,
    sync: isize,
    open_zcode: isize,
    stop: isize,
    copy_connectors: isize,
    footer: isize,
}

struct AppState {
    host: NativeHost,
    hwnd: isize,
    controls: Controls,
    font: isize,
    font_bold: isize,
    font_title: isize,
    dpi: u32,
    refreshing: bool,
    quitting: bool,
    provider: String,
    connectors_text: String,
}

impl AppState {
    fn new(host: NativeHost) -> Self {
        Self {
            host,
            hwnd: 0,
            controls: Controls::default(),
            font: 0,
            font_bold: 0,
            font_title: 0,
            dpi: 96,
            refreshing: false,
            quitting: false,
            provider: "antigravity".to_string(),
            connectors_text: String::new(),
        }
    }
}

fn wide(value: &str) -> Vec<u16> {
    value.encode_utf16().chain(std::iter::once(0)).collect()
}

fn scale(value: i32, dpi: u32) -> i32 {
    ((value as i64 * dpi as i64) / 96) as i32
}

fn loword(value: usize) -> i32 {
    (value & 0xffff) as i32
}

unsafe fn set_text(hwnd: isize, text: &str) {
    if hwnd != 0 {
        let text = wide(text);
        unsafe { SetWindowTextW(hwnd as HWND, text.as_ptr()) };
    }
}

unsafe fn create_control(
    parent: HWND,
    class_name: &str,
    text: &str,
    style: u32,
    id: i32,
    font: isize,
) -> isize {
    let class_name = wide(class_name);
    let text = wide(text);
    let hwnd = unsafe {
        CreateWindowExW(
            0,
            class_name.as_ptr(),
            text.as_ptr(),
            WS_CHILD | WS_VISIBLE | style,
            0,
            0,
            0,
            0,
            parent,
            id as usize as HMENU,
            GetModuleHandleW(null()),
            null_mut(),
        )
    };
    if !hwnd.is_null() && font != 0 {
        unsafe { SendMessageW(hwnd, WM_SETFONT, font as WPARAM, 1) };
    }
    hwnd as isize
}

unsafe fn create_font(point_size: i32, weight: i32, dpi: u32) -> isize {
    let face = wide("Segoe UI Variable Text");
    unsafe {
        CreateFontW(
            -scale(point_size, dpi),
            0,
            0,
            0,
            weight,
            0,
            0,
            0,
            DEFAULT_CHARSET as u32,
            OUT_DEFAULT_PRECIS as u32,
            CLIP_DEFAULT_PRECIS as u32,
            CLEARTYPE_QUALITY as u32,
            (DEFAULT_PITCH | FF_DONTCARE) as u32,
            face.as_ptr(),
        ) as isize
    }
}

unsafe fn create_controls(hwnd: HWND) {
    let Some(state_lock) = STATE.get() else {
        return;
    };
    let mut state = state_lock.lock().unwrap();
    state.dpi = unsafe { GetDpiForWindow(hwnd) }.max(96);
    state.font = unsafe { create_font(14, FW_NORMAL as i32, state.dpi) };
    state.font_bold = unsafe { create_font(14, FW_SEMIBOLD as i32, state.dpi) };
    state.font_title = unsafe { create_font(26, FW_SEMIBOLD as i32, state.dpi) };

    let font = state.font;
    let font_bold = state.font_bold;
    let font_title = state.font_title;
    let c = &mut state.controls;
    c.subtitle = unsafe {
        create_control(
            hwnd,
            "STATIC",
            "Swift/Rust 原生控制中心 · 正在连接本地核心",
            SS_LEFT,
            0,
            font,
        )
    };
    c.provider_antigravity = unsafe {
        create_control(
            hwnd,
            "BUTTON",
            "Antigravity",
            BS_AUTORADIOBUTTON as u32 | WS_GROUP,
            ID_PROVIDER_ANTIGRAVITY,
            font_bold,
        )
    };
    c.provider_grok = unsafe {
        create_control(
            hwnd,
            "BUTTON",
            "Grok / xAI",
            BS_AUTORADIOBUTTON as u32,
            ID_PROVIDER_GROK,
            font_bold,
        )
    };
    c.refresh = unsafe {
        create_control(
            hwnd,
            "BUTTON",
            "刷新",
            BS_PUSHBUTTON as u32,
            ID_REFRESH,
            font,
        )
    };
    c.status_tun = unsafe {
        create_control(
            hwnd,
            "STATIC",
            "TUN\r\n正在检查",
            SS_LEFT | WS_BORDER,
            0,
            font,
        )
    };
    c.status_proxy = unsafe {
        create_control(
            hwnd,
            "STATIC",
            "PROXY\r\n正在检查",
            SS_LEFT | WS_BORDER,
            0,
            font,
        )
    };
    c.status_bridge = unsafe {
        create_control(
            hwnd,
            "STATIC",
            "BRIDGE\r\n正在检查",
            SS_LEFT | WS_BORDER,
            0,
            font,
        )
    };
    c.status_zcode = unsafe {
        create_control(
            hwnd,
            "STATIC",
            "ZCODE\r\n正在检查",
            SS_LEFT | WS_BORDER,
            0,
            font,
        )
    };
    c.quota_title =
        unsafe { create_control(hwnd, "STATIC", "Gemini 模型额度", SS_LEFT, 0, font_title) };
    c.quota_body =
        unsafe { create_control(hwnd, "STATIC", "等待首次刷新…", SS_LEFT, 0, font) };
    c.quota_progress =
        unsafe { create_control(hwnd, "msctls_progress32", "", PBS_SMOOTH, 0, font) };
    unsafe { SendMessageW(c.quota_progress as HWND, PBM_SETRANGE32, 0, 100) };
    c.models =
        unsafe { create_control(hwnd, "STATIC", "模型：等待网关同步", SS_LEFT, 0, font) };
    c.action_header =
        unsafe { create_control(hwnd, "STATIC", "接入控制", SS_LEFT, 0, font_title) };
    c.setup = unsafe {
        create_control(
            hwnd,
            "BUTTON",
            "一键接入 ZCode",
            BS_DEFPUSHBUTTON as u32,
            ID_SETUP,
            font_bold,
        )
    };
    c.login_antigravity = unsafe {
        create_control(
            hwnd,
            "BUTTON",
            "登录 Antigravity",
            BS_PUSHBUTTON as u32,
            ID_LOGIN_ANTIGRAVITY,
            font,
        )
    };
    c.login_grok = unsafe {
        create_control(
            hwnd,
            "BUTTON",
            "登录 Grok / xAI",
            BS_PUSHBUTTON as u32,
            ID_LOGIN_GROK,
            font,
        )
    };
    c.sync = unsafe {
        create_control(
            hwnd,
            "BUTTON",
            "修复并重新同步",
            BS_PUSHBUTTON as u32,
            ID_SYNC,
            font,
        )
    };
    c.open_zcode = unsafe {
        create_control(
            hwnd,
            "BUTTON",
            "打开 ZCode",
            BS_PUSHBUTTON as u32,
            ID_OPEN_ZCODE,
            font,
        )
    };
    c.stop = unsafe {
        create_control(
            hwnd,
            "BUTTON",
            "停止本地网关",
            BS_PUSHBUTTON as u32,
            ID_STOP,
            font,
        )
    };
    c.copy_connectors = unsafe {
        create_control(
            hwnd,
            "BUTTON",
            "复制当前 Agent 配置",
            BS_PUSHBUTTON as u32,
            ID_COPY_CONNECTORS,
            font,
        )
    };
    c.footer = unsafe {
        create_control(
            hwnd,
            "STATIC",
            "127.0.0.1 · 当前用户密钥 · Windows Rust Native",
            SS_LEFT,
            0,
            font,
        )
    };
    unsafe {
        SendMessageW(
            c.provider_antigravity as HWND,
            BM_SETCHECK,
            BST_CHECKED as WPARAM,
            0,
        )
    };
}

unsafe fn layout(hwnd: HWND) {
    let Some(state_lock) = STATE.get() else {
        return;
    };
    let state = state_lock.lock().unwrap();
    let dpi = state.dpi;
    let mut rect = RECT::default();
    unsafe { GetClientRect(hwnd, &mut rect) };
    let width = rect.right - rect.left;
    let margin = scale(28, dpi);
    let gap = scale(12, dpi);
    let top = scale(24, dpi);
    let provider_y = scale(70, dpi);
    let provider_w = scale(150, dpi);
    let status_y = scale(120, dpi);
    let status_h = scale(72, dpi);
    let status_w = (width - margin * 2 - gap * 3) / 4;
    let action_w = scale(300, dpi);
    let action_x = width - margin - action_w;
    let content_top = scale(220, dpi);
    let left_w = (action_x - gap - margin).max(scale(420, dpi));
    let button_h = scale(42, dpi);

    let move_control = |handle: isize, x: i32, y: i32, w: i32, h: i32| unsafe {
        if handle != 0 {
            MoveWindow(handle as HWND, x, y, w, h, 1)
        } else {
            0
        }
    };
    move_control(
        state.controls.subtitle,
        margin,
        top,
        width - margin * 2,
        scale(34, dpi),
    );
    move_control(
        state.controls.provider_antigravity,
        margin,
        provider_y,
        provider_w,
        scale(34, dpi),
    );
    move_control(
        state.controls.provider_grok,
        margin + provider_w + gap,
        provider_y,
        provider_w,
        scale(34, dpi),
    );
    move_control(
        state.controls.refresh,
        width - margin - scale(90, dpi),
        provider_y,
        scale(90, dpi),
        scale(34, dpi),
    );
    for (index, handle) in [
        state.controls.status_tun,
        state.controls.status_proxy,
        state.controls.status_bridge,
        state.controls.status_zcode,
    ]
    .iter()
    .enumerate()
    {
        move_control(
            *handle,
            margin + index as i32 * (status_w + gap),
            status_y,
            status_w,
            status_h,
        );
    }
    move_control(
        state.controls.quota_title,
        margin,
        content_top,
        left_w,
        scale(40, dpi),
    );
    move_control(
        state.controls.quota_progress,
        margin,
        content_top + scale(52, dpi),
        left_w,
        scale(18, dpi),
    );
    move_control(
        state.controls.quota_body,
        margin,
        content_top + scale(82, dpi),
        left_w,
        scale(270, dpi),
    );
    move_control(
        state.controls.models,
        margin,
        content_top + scale(360, dpi),
        left_w,
        scale(92, dpi),
    );
    move_control(
        state.controls.action_header,
        action_x,
        content_top,
        action_w,
        scale(40, dpi),
    );
    for (index, handle) in [
        state.controls.setup,
        state.controls.login_antigravity,
        state.controls.login_grok,
        state.controls.sync,
        state.controls.open_zcode,
        state.controls.stop,
    ]
    .iter()
    .enumerate()
    {
        move_control(
            *handle,
            action_x,
            content_top + scale(52, dpi) + index as i32 * (button_h + gap),
            action_w,
            button_h,
        );
    }
    move_control(
        state.controls.copy_connectors,
        action_x,
        content_top + scale(52, dpi) + 6 * (button_h + gap),
        action_w,
        button_h,
    );
    move_control(
        state.controls.footer,
        margin,
        rect.bottom - scale(32, dpi),
        width - margin * 2,
        scale(22, dpi),
    );
}

fn api_get<T: for<'de> Deserialize<'de>>(
    connection: &NativeConnection,
    path: &str,
) -> Result<T, String> {
    let url = format!("{}{}", connection.base_url, path);
    let mut response = ureq::get(url)
        .header("X-ZCAB-Session", &connection.session)
        .call()
        .map_err(|error| error.to_string())?;
    response
        .body_mut()
        .read_json()
        .map_err(|error| error.to_string())
}

fn api_post(
    connection: &NativeConnection,
    path: &str,
    body: serde_json::Value,
) -> Result<(), String> {
    let url = format!("{}{}", connection.base_url, path);
    ureq::post(url)
        .header("X-ZCAB-Session", &connection.session)
        .send_json(body)
        .map_err(|error| error.to_string())?;
    Ok(())
}

fn request_refresh(hwnd: HWND) {
    let Some(state_lock) = STATE.get() else {
        return;
    };
    let (connection, hwnd_value) = {
        let mut state = state_lock.lock().unwrap();
        if state.refreshing {
            return;
        }
        state.refreshing = true;
        (state.host.connection.clone(), hwnd as isize)
    };
    thread::spawn(move || {
        let status = api_get::<DashboardStatus>(&connection, "/api/status");
        let provider = status
            .as_ref()
            .map(|value| value.selected_provider.as_str())
            .unwrap_or("antigravity");
        let quota = api_get::<QuotaReport>(&connection, &format!("/api/quota?provider={provider}"));
        let connectors = api_get::<ConnectorResponse>(&connection, "/api/connectors");
        let update = Box::new(RefreshResult {
            status,
            quota,
            connectors,
        });
        unsafe {
            let _ = PostMessageW(
                hwnd_value as HWND,
                WM_REFRESH_READY,
                0,
                Box::into_raw(update) as LPARAM,
            );
        }
    });
}

fn format_item(name: &str, item: &DashboardItem) -> String {
    format!(
        "{}  {}\r\n{}{}",
        if item.ok { "●" } else { "○" },
        name,
        item.label,
        item.detail
            .as_deref()
            .map(|value| format!(" · {value}"))
            .unwrap_or_default()
    )
}

fn quota_text(report: &QuotaReport) -> (String, i32) {
    let mut lines = Vec::new();
    let mut lowest = 100.0_f64;
    let mut has_value = false;
    for account in &report.accounts {
        let plan = account.plan.as_deref().unwrap_or(&account.status);
        lines.push(format!("{}  ·  {}", account.account, plan));
        if let Some(error) = &account.error {
            lines.push(format!("  ⚠ {error}"));
        }
        for group in account.groups.as_deref().unwrap_or(&[]) {
            lines.push(format!("  {}", group.name));
            for bucket in &group.buckets {
                let value = bucket.remaining_percent.unwrap_or(0.0);
                if bucket.remaining_percent.is_some() {
                    lowest = lowest.min(value);
                    has_value = true;
                }
                let reset = bucket.reset_time.as_deref().unwrap_or("");
                lines.push(format!("    {}：{:.0}%  {}", bucket.name, value, reset));
            }
        }
        if let Some(credits) = &account.credits
            && credits.available
        {
            lines.push(format!(
                "  Credits：{:.2} {}",
                credits.amount, credits.credit_type
            ));
        }
        if let Some(message) = &account.status_message
            && !message.is_empty()
        {
            lines.push(format!("  {message}"));
        }
    }
    if let Some(warning) = &report.warning
        && !warning.is_empty()
    {
        lines.push(format!("提示：{warning}"));
    }
    if lines.is_empty() {
        lines.push("额度暂不可用，请先登录并启动网关。".to_string());
    }
    (
        lines.join("\r\n"),
        if has_value { lowest.round() as i32 } else { 0 },
    )
}

fn connectors_text(connectors: &ConnectorResponse) -> String {
    let mut output = format!(
        "ZCode Local Bridge\r\nModel: {}\r\nBase URL: {}\r\n",
        connectors.model, connectors.base_url
    );
    for connector in &connectors.connectors {
        output.push_str(&format!(
            "\r\n=== {} ===\r\n{}\r\n",
            connector.name, connector.description
        ));
        for (name, snippet) in &connector.snippets {
            output.push_str(&format!("\r\n[{name}]\r\n{snippet}\r\n"));
        }
    }
    output
}

unsafe fn apply_refresh(update: RefreshResult) {
    let Some(state_lock) = STATE.get() else {
        return;
    };
    let mut state = state_lock.lock().unwrap();
    state.refreshing = false;
    match update.status {
        Ok(status) => {
            state.provider = status.selected_provider.clone();
            unsafe {
                let operation_message = status
                    .operation
                    .error
                    .as_deref()
                    .or(status.operation.message.as_deref());
                set_text(
                    state.controls.subtitle,
                    operation_message.unwrap_or(if status.operation.running {
                        "本地操作正在进行…"
                    } else if status.gateway.ok {
                        "本地安全核心在线"
                    } else {
                        "请选择提供商并执行一键接入"
                    }),
                );
                set_text(state.controls.status_tun, &format_item("TUN", &status.tun));
                set_text(
                    state.controls.status_proxy,
                    &format_item("PROXY", &status.proxy),
                );
                set_text(
                    state.controls.status_bridge,
                    &format_item("BRIDGE", &status.gateway),
                );
                set_text(
                    state.controls.status_zcode,
                    &format_item("ZCODE", &status.zcode),
                );
                set_text(
                    state.controls.models,
                    &format!(
                        "当前模型（{}）\r\n{}",
                        status.models.len(),
                        if status.models.is_empty() {
                            "等待网关同步".to_string()
                        } else {
                            status.models.join("  ·  ")
                        }
                    ),
                );
                set_text(
                    state.controls.provider_antigravity,
                    &format!("Antigravity  {}", status.provider_accounts.antigravity),
                );
                set_text(
                    state.controls.provider_grok,
                    &format!("Grok / xAI  {}", status.provider_accounts.xai),
                );
                SendMessageW(
                    state.controls.provider_antigravity as HWND,
                    BM_SETCHECK,
                    if state.provider == "xai" {
                        BST_UNCHECKED
                    } else {
                        BST_CHECKED
                    } as WPARAM,
                    0,
                );
                SendMessageW(
                    state.controls.provider_grok as HWND,
                    BM_SETCHECK,
                    if state.provider == "xai" {
                        BST_CHECKED
                    } else {
                        BST_UNCHECKED
                    } as WPARAM,
                    0,
                );
                set_text(
                    state.controls.quota_title,
                    if state.provider == "xai" {
                        "Grok 共享额度"
                    } else {
                        "Gemini 模型额度"
                    },
                );
            }
        }
        Err(error) => unsafe {
            set_text(state.controls.subtitle, &format!("状态刷新失败：{error}"))
        },
    }
    match update.quota {
        Ok(quota) => {
            let (text, percent) = quota_text(&quota);
            unsafe {
                set_text(state.controls.quota_body, &text);
                SendMessageW(
                    state.controls.quota_progress as HWND,
                    PBM_SETPOS,
                    percent as WPARAM,
                    0,
                );
            }
            update_tray(&state, Some(percent));
        }
        Err(error) => {
            unsafe {
                set_text(
                    state.controls.quota_body,
                    &format!("额度暂不可用\r\n{error}"),
                )
            };
            update_tray(&state, None);
        }
    }
    if let Ok(connectors) = update.connectors {
        state.connectors_text = connectors_text(&connectors);
    }
}

fn run_operation(hwnd: HWND, action: &'static str) {
    let Some(state_lock) = STATE.get() else {
        return;
    };
    let connection = state_lock.lock().unwrap().host.connection.clone();
    let hwnd_value = hwnd as isize;
    thread::spawn(move || {
        let _ = api_post(&connection, "/api/action", json!({"action": action}));
        thread::sleep(Duration::from_millis(450));
        unsafe {
            let _ = PostMessageW(hwnd_value as HWND, WM_TIMER, TIMER_REFRESH, 0);
        }
    });
}

fn select_provider(hwnd: HWND, provider: &'static str) {
    let Some(state_lock) = STATE.get() else {
        return;
    };
    let connection = state_lock.lock().unwrap().host.connection.clone();
    let hwnd_value = hwnd as isize;
    thread::spawn(move || {
        let _ = api_post(&connection, "/api/provider", json!({"provider": provider}));
        unsafe {
            let _ = PostMessageW(hwnd_value as HWND, WM_TIMER, TIMER_REFRESH, 0);
        }
    });
}

unsafe fn copy_to_clipboard(hwnd: HWND, value: &str) -> Result<(), String> {
    let wide = wide(value);
    let bytes = wide.len() * std::mem::size_of::<u16>();
    if unsafe { OpenClipboard(hwnd) } == 0 {
        return Err("无法打开剪贴板".to_string());
    }
    unsafe { EmptyClipboard() };
    let memory = unsafe { GlobalAlloc(GMEM_MOVEABLE, bytes) };
    if memory.is_null() {
        unsafe { CloseClipboard() };
        return Err("无法分配剪贴板内存".to_string());
    }
    let target = unsafe { GlobalLock(memory) } as *mut u16;
    unsafe {
        std::ptr::copy_nonoverlapping(wide.as_ptr(), target, wide.len());
        GlobalUnlock(memory);
    }
    if unsafe { SetClipboardData(CF_UNICODETEXT_VALUE, memory as HANDLE) }.is_null() {
        unsafe {
            GlobalFree(memory);
            CloseClipboard();
        }
        return Err("写入剪贴板失败".to_string());
    }
    unsafe { CloseClipboard() };
    Ok(())
}

unsafe fn add_tray_icon(hwnd: HWND) {
    let mut data: NOTIFYICONDATAW = unsafe { std::mem::zeroed() };
    data.cbSize = std::mem::size_of::<NOTIFYICONDATAW>() as u32;
    data.hWnd = hwnd;
    data.uID = 1;
    data.uFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP;
    data.uCallbackMessage = WM_TRAY;
    data.hIcon = unsafe { load_app_icon() };
    copy_wide_fixed(&mut data.szTip, "ZCode Antigravity · 正在读取额度");
    unsafe { Shell_NotifyIconW(NIM_ADD, &data) };
}

unsafe fn load_app_icon() -> HICON {
    let resource = std::ptr::without_provenance::<u16>(1);
    let icon = unsafe { LoadIconW(GetModuleHandleW(null()), resource) };
    if icon.is_null() {
        unsafe { LoadIconW(null_mut(), IDI_APPLICATION) }
    } else {
        icon
    }
}

fn copy_wide_fixed<const N: usize>(target: &mut [u16; N], text: &str) {
    let encoded: Vec<u16> = text.encode_utf16().take(N.saturating_sub(1)).collect();
    target[..encoded.len()].copy_from_slice(&encoded);
    target[encoded.len()] = 0;
}

fn update_tray(state: &AppState, percent: Option<i32>) {
    let mut data: NOTIFYICONDATAW = unsafe { std::mem::zeroed() };
    data.cbSize = std::mem::size_of::<NOTIFYICONDATAW>() as u32;
    data.hWnd = state.hwnd as HWND;
    data.uID = 1;
    data.uFlags = NIF_TIP;
    let provider = if state.provider == "xai" {
        "Grok"
    } else {
        "Antigravity"
    };
    copy_wide_fixed(
        &mut data.szTip,
        &percent
            .map(|value| format!("ZCode · {provider} 最低剩余额度 {value}%"))
            .unwrap_or_else(|| format!("ZCode · {provider} 额度暂不可用")),
    );
    unsafe { Shell_NotifyIconW(NIM_MODIFY, &data) };
}

unsafe fn remove_tray_icon(hwnd: HWND) {
    let mut data: NOTIFYICONDATAW = unsafe { std::mem::zeroed() };
    data.cbSize = std::mem::size_of::<NOTIFYICONDATAW>() as u32;
    data.hWnd = hwnd;
    data.uID = 1;
    unsafe { Shell_NotifyIconW(NIM_DELETE, &data) };
}

unsafe fn show_tray_menu(hwnd: HWND) {
    let menu = unsafe { CreatePopupMenu() };
    let provider = STATE
        .get()
        .map(|state| state.lock().unwrap().provider.clone())
        .unwrap_or_default();
    unsafe {
        AppendMenuW(menu, MF_STRING, ID_TRAY_OPEN, wide("打开控制中心").as_ptr());
        AppendMenuW(menu, MF_SEPARATOR, 0, null());
        AppendMenuW(
            menu,
            MF_STRING | if provider == "xai" { 0 } else { MF_CHECKED },
            ID_TRAY_ANTIGRAVITY,
            wide("使用 Antigravity").as_ptr(),
        );
        AppendMenuW(
            menu,
            MF_STRING | if provider == "xai" { MF_CHECKED } else { 0 },
            ID_TRAY_GROK,
            wide("使用 Grok").as_ptr(),
        );
        AppendMenuW(menu, MF_STRING, ID_TRAY_REFRESH, wide("刷新额度").as_ptr());
        AppendMenuW(menu, MF_SEPARATOR, 0, null());
        AppendMenuW(menu, MF_STRING, ID_TRAY_QUIT, wide("退出").as_ptr());
        let mut point = POINT::default();
        GetCursorPos(&mut point);
        SetForegroundWindow(hwnd);
        let selected = TrackPopupMenu(
            menu,
            TPM_RETURNCMD | TPM_RIGHTBUTTON,
            point.x,
            point.y,
            0,
            hwnd,
            null(),
        ) as usize;
        DestroyMenu(menu);
        match selected {
            ID_TRAY_OPEN => {
                ShowWindow(hwnd, SW_RESTORE);
                SetForegroundWindow(hwnd);
            }
            ID_TRAY_REFRESH => request_refresh(hwnd),
            ID_TRAY_ANTIGRAVITY => select_provider(hwnd, "antigravity"),
            ID_TRAY_GROK => select_provider(hwnd, "xai"),
            ID_TRAY_QUIT => {
                if let Some(state) = STATE.get() {
                    state.lock().unwrap().quitting = true;
                }
                DestroyWindow(hwnd);
            }
            _ => {}
        }
    }
}

unsafe extern "system" fn window_proc(
    hwnd: HWND,
    message: u32,
    wparam: WPARAM,
    lparam: LPARAM,
) -> LRESULT {
    match message {
        WM_CREATE => {
            if let Some(state) = STATE.get() {
                state.lock().unwrap().hwnd = hwnd as isize;
            }
            unsafe {
                create_controls(hwnd);
                layout(hwnd);
                add_tray_icon(hwnd);
                SetTimer(hwnd, TIMER_REFRESH, 15_000, None);
            }
            request_refresh(hwnd);
            0
        }
        WM_SIZE => {
            unsafe { layout(hwnd) };
            0
        }
        WM_DPICHANGED => {
            if let Some(state) = STATE.get() {
                state.lock().unwrap().dpi = (wparam >> 16) as u32;
            }
            let suggested = lparam as *const RECT;
            if !suggested.is_null() {
                let rect = unsafe { *suggested };
                unsafe {
                    SetWindowPos(
                        hwnd,
                        null_mut(),
                        rect.left,
                        rect.top,
                        rect.right - rect.left,
                        rect.bottom - rect.top,
                        SWP_NOZORDER | SWP_NOACTIVATE,
                    )
                };
            }
            unsafe { layout(hwnd) };
            0
        }
        WM_GETMINMAXINFO => {
            let dpi = STATE
                .get()
                .map(|state| state.lock().unwrap().dpi)
                .unwrap_or(96);
            let info = lparam as *mut MINMAXINFO;
            if !info.is_null() {
                unsafe {
                    (*info).ptMinTrackSize.x = scale(900, dpi);
                    (*info).ptMinTrackSize.y = scale(670, dpi);
                }
            }
            0
        }
        WM_COMMAND => {
            match loword(wparam) {
                ID_PROVIDER_ANTIGRAVITY => select_provider(hwnd, "antigravity"),
                ID_PROVIDER_GROK => select_provider(hwnd, "xai"),
                ID_REFRESH => request_refresh(hwnd),
                ID_SETUP => run_operation(hwnd, "setup"),
                ID_LOGIN_ANTIGRAVITY => run_operation(hwnd, "login"),
                ID_LOGIN_GROK => run_operation(hwnd, "login-grok"),
                ID_SYNC => run_operation(hwnd, "sync"),
                ID_OPEN_ZCODE => run_operation(hwnd, "open-zcode"),
                ID_STOP => run_operation(hwnd, "stop"),
                ID_COPY_CONNECTORS => {
                    let text = STATE
                        .get()
                        .map(|state| state.lock().unwrap().connectors_text.clone())
                        .unwrap_or_default();
                    if text.is_empty() {
                        unsafe {
                            MessageBoxW(
                                hwnd,
                                wide("请先启动网关并刷新数据。").as_ptr(),
                                wide("Agent 配置").as_ptr(),
                                MB_OK | MB_ICONINFORMATION,
                            )
                        };
                    } else if let Err(error) = unsafe { copy_to_clipboard(hwnd, &text) } {
                        unsafe {
                            MessageBoxW(
                                hwnd,
                                wide(&error).as_ptr(),
                                wide("复制失败").as_ptr(),
                                MB_OK | MB_ICONERROR,
                            )
                        };
                    } else {
                        unsafe {
                            set_text(
                                STATE.get().unwrap().lock().unwrap().controls.subtitle,
                                "Agent 配置已复制到剪贴板，请勿发给他人",
                            )
                        };
                    }
                }
                _ => {}
            }
            0
        }
        WM_TIMER => {
            request_refresh(hwnd);
            0
        }
        WM_REFRESH_READY => {
            if lparam != 0 {
                let update = unsafe { Box::from_raw(lparam as *mut RefreshResult) };
                unsafe { apply_refresh(*update) };
            }
            0
        }
        WM_TRAY => {
            match lparam as u32 {
                WM_LBUTTONUP | WM_LBUTTONDBLCLK => unsafe {
                    ShowWindow(hwnd, SW_RESTORE);
                    SetForegroundWindow(hwnd);
                },
                WM_RBUTTONUP => unsafe { show_tray_menu(hwnd) },
                _ => {}
            }
            0
        }
        WM_CLOSE => {
            let quitting = STATE
                .get()
                .map(|state| state.lock().unwrap().quitting)
                .unwrap_or(false);
            if quitting {
                unsafe {
                    DestroyWindow(hwnd);
                }
            } else {
                unsafe {
                    ShowWindow(hwnd, SW_HIDE);
                }
            }
            0
        }
        WM_DESTROY => {
            unsafe {
                KillTimer(hwnd, TIMER_REFRESH);
                remove_tray_icon(hwnd)
            };
            if let Some(state) = STATE.get() {
                let mut state = state.lock().unwrap();
                state.host.shutdown();
                unsafe {
                    if state.font != 0 {
                        DeleteObject(state.font as HGDIOBJ);
                    }
                    if state.font_bold != 0 {
                        DeleteObject(state.font_bold as HGDIOBJ);
                    }
                    if state.font_title != 0 {
                        DeleteObject(state.font_title as HGDIOBJ);
                    }
                }
            }
            unsafe { PostQuitMessage(0) };
            0
        }
        _ => unsafe { DefWindowProcW(hwnd, message, wparam, lparam) },
    }
}

fn show_fatal(message: &str) {
    unsafe {
        MessageBoxW(
            null_mut(),
            wide(message).as_ptr(),
            wide("ZCode Antigravity").as_ptr(),
            MB_OK | MB_ICONERROR | MB_TOPMOST,
        );
    }
}

fn main() {
    unsafe {
        SetProcessDpiAwarenessContext(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2);
        let controls = INITCOMMONCONTROLSEX {
            dwSize: std::mem::size_of::<INITCOMMONCONTROLSEX>() as u32,
            dwICC: ICC_PROGRESS_CLASS,
        };
        InitCommonControlsEx(&controls);
    }
    let host = match NativeHost::start() {
        Ok(host) => host,
        Err(error) => {
            show_fatal(&error);
            return;
        }
    };
    let _ = STATE.set(Mutex::new(AppState::new(host)));

    unsafe {
        let instance = GetModuleHandleW(null());
        let class_name = wide("ZCodeAntigravityRustNative");
        let class = WNDCLASSEXW {
            cbSize: std::mem::size_of::<WNDCLASSEXW>() as u32,
            style: CS_HREDRAW | CS_VREDRAW,
            lpfnWndProc: Some(window_proc),
            cbClsExtra: 0,
            cbWndExtra: 0,
            hInstance: instance,
            hIcon: load_app_icon(),
            hCursor: LoadCursorW(null_mut(), IDC_ARROW),
            hbrBackground: (COLOR_WINDOW + 1) as HBRUSH,
            lpszMenuName: null(),
            lpszClassName: class_name.as_ptr(),
            hIconSm: load_app_icon(),
        };
        if RegisterClassExW(&class) == 0 {
            show_fatal("无法注册 Windows 原生窗口");
            return;
        }
        let title = wide(&format!("ZCode · Antigravity 控制中心 · Rust {VERSION}"));
        let hwnd = CreateWindowExW(
            WS_EX_APPWINDOW,
            class_name.as_ptr(),
            title.as_ptr(),
            WS_OVERLAPPEDWINDOW | WS_VISIBLE,
            CW_USEDEFAULT,
            CW_USEDEFAULT,
            1120,
            780,
            null_mut(),
            null_mut(),
            instance,
            null_mut::<c_void>(),
        );
        if hwnd.is_null() {
            show_fatal("无法创建 Windows 原生窗口");
            return;
        }
        ShowWindow(hwnd, SW_SHOW);
        UpdateWindow(hwnd);
        let mut message = MSG::default();
        while GetMessageW(&mut message, null_mut(), 0, 0) > 0 {
            TranslateMessage(&message);
            DispatchMessageW(&message);
        }
    }
}
