#![windows_subsystem = "windows"]

mod protocol;

use protocol::{ConnectorResponse, NativeConnection};
use serde::Deserialize;
use serde_json::json;
use std::collections::HashMap;
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
use windows_sys::Win32::UI::Input::KeyboardAndMouse::EnableWindow;
use windows_sys::Win32::UI::Shell::*;
use windows_sys::Win32::UI::WindowsAndMessaging::*;

const VERSION: &str = "0.4.6-test";
const SS_LEFT: u32 = 0;
const CF_UNICODETEXT_VALUE: u32 = 13;
const WM_REFRESH_READY: u32 = WM_APP + 1;
const WM_OPERATION_POSTED: u32 = WM_APP + 2;
const WM_PROVIDER_READY: u32 = WM_APP + 3;
const WM_TRAY: u32 = WM_APP + 20;
const TIMER_REFRESH: usize = 1;
const TIMER_OPERATION_POLL: usize = 2;
const STATUS_REFRESH_MS: u32 = 5_000;
const QUOTA_REFRESH_INTERVAL: Duration = Duration::from_secs(5 * 60);

const COLOR_SIDEBAR: COLORREF = 0x00442308;
const COLOR_SIDEBAR_ACTIVE: COLORREF = 0x008C4A16;
const COLOR_BACKGROUND: COLORREF = 0x00FFF9F4;
const COLOR_CARD: COLORREF = 0x00FFFFFF;
const COLOR_BORDER: COLORREF = 0x00F0E3D3;
const COLOR_GRID: COLORREF = 0x00F8F0E7;
const COLOR_PRIMARY: COLORREF = 0x00F56B16;
const COLOR_PRIMARY_DARK: COLORREF = 0x00CF5710;
const COLOR_PRIMARY_SOFT: COLORREF = 0x00FFF2E9;
const COLOR_TEXT: COLORREF = 0x00422A16;
const COLOR_MUTED: COLORREF = 0x00907762;
const COLOR_LIGHT_TEXT: COLORREF = 0x00F8E9DC;
const COLOR_SUCCESS: COLORREF = 0x0062A024;
const COLOR_WARNING: COLORREF = 0x001E91DC;
const COLOR_DANGER: COLORREF = 0x005A41DA;
const COLOR_TRACK: COLORREF = 0x00F4EDE6;

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
#[serde(rename_all = "camelCase")]
struct QuotaReport {
    fetched_at: Option<String>,
    provider: String,
    source: String,
    stale: bool,
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
    window: String,
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

struct RefreshResult {
    status: Result<DashboardStatus, String>,
    quota: Option<Result<QuotaReport, String>>,
    usage: Result<UsageReport, String>,
    connectors: Result<ConnectorResponse, String>,
    requested_provider: String,
}

struct OperationPostResult {
    error: Option<String>,
}

struct ProviderSelectResult {
    requested: String,
    previous: String,
    error: Option<String>,
}

#[derive(Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct UsageSample {
    timestamp: String,
    model: String,
    output_tokens: i64,
    reasoning_tokens: i64,
    latency_ms: i64,
    ttft_ms: i64,
    generation_ms: i64,
    output_tokens_per_second: f64,
    speed_basis: String,
}

#[derive(Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct UsageAggregate {
    requests: i32,
    output_tokens: i64,
    reasoning_tokens: i64,
    average_tokens_per_second: f64,
}

#[derive(Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct UsageReport {
    provider: String,
    available: bool,
    latest: Option<UsageSample>,
    total: UsageAggregate,
    warning: Option<String>,
}

#[derive(Clone, Copy, Default)]
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
    refresh_again: bool,
    operation_pending: bool,
    provider_switching: bool,
    quitting: bool,
    provider: String,
    connectors_text: String,
    quota: Option<QuotaReport>,
    quota_cache: HashMap<String, QuotaReport>,
    quota_error: Option<String>,
    usage: Option<UsageReport>,
    usage_cache: HashMap<String, UsageReport>,
    provider_accounts: ProviderAccounts,
    last_quota_refresh: Option<Instant>,
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
            refresh_again: false,
            operation_pending: false,
            provider_switching: false,
            quitting: false,
            provider: "antigravity".to_string(),
            connectors_text: String::new(),
            quota: None,
            quota_cache: HashMap::new(),
            quota_error: None,
            usage: None,
            usage_cache: HashMap::new(),
            provider_accounts: ProviderAccounts::default(),
            last_quota_refresh: None,
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
    let face = wide("Microsoft YaHei UI");
    let pixel_height = -((point_size * dpi as i32 + 36) / 72).max(1);
    unsafe {
        CreateFontW(
            pixel_height,
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
    let dpi = unsafe { GetDpiForWindow(hwnd) }.max(96);
    let font = unsafe { create_font(10, FW_NORMAL as i32, dpi) };
    let font_bold = unsafe { create_font(10, FW_SEMIBOLD as i32, dpi) };
    let font_title = unsafe { create_font(17, FW_SEMIBOLD as i32, dpi) };
    let mut controls = Controls::default();
    let c = &mut controls;
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
            BS_OWNERDRAW as u32 | WS_GROUP,
            ID_PROVIDER_ANTIGRAVITY,
            font_bold,
        )
    };
    c.provider_grok = unsafe {
        create_control(
            hwnd,
            "BUTTON",
            "Grok / xAI",
            BS_OWNERDRAW as u32,
            ID_PROVIDER_GROK,
            font_bold,
        )
    };
    c.refresh = unsafe {
        create_control(
            hwnd,
            "BUTTON",
            "刷新",
            BS_OWNERDRAW as u32,
            ID_REFRESH,
            font,
        )
    };
    c.status_tun =
        unsafe { create_control(hwnd, "STATIC", "TUN\r\n正在检查", SS_LEFT, 0, font) };
    c.status_proxy =
        unsafe { create_control(hwnd, "STATIC", "PROXY\r\n正在检查", SS_LEFT, 0, font) };
    c.status_bridge =
        unsafe { create_control(hwnd, "STATIC", "BRIDGE\r\n正在检查", SS_LEFT, 0, font) };
    c.status_zcode =
        unsafe { create_control(hwnd, "STATIC", "ZCODE\r\n正在检查", SS_LEFT, 0, font) };
    c.quota_title =
        unsafe { create_control(hwnd, "STATIC", "Gemini 模型额度", SS_LEFT, 0, font_title) };
    c.quota_body =
        unsafe { create_control(hwnd, "STATIC", "等待首次刷新…", SS_LEFT, 0, font) };
    c.quota_progress =
        unsafe { create_control(hwnd, "msctls_progress32", "", PBS_SMOOTH, 0, font) };
    unsafe { SendMessageW(c.quota_progress as HWND, PBM_SETRANGE32, 0, 100) };
    unsafe {
        ShowWindow(c.quota_body as HWND, SW_HIDE);
        ShowWindow(c.quota_progress as HWND, SW_HIDE);
    }
    c.models =
        unsafe { create_control(hwnd, "STATIC", "模型：等待网关同步", SS_LEFT, 0, font) };
    c.action_header =
        unsafe { create_control(hwnd, "STATIC", "接入控制", SS_LEFT, 0, font_title) };
    c.setup = unsafe {
        create_control(
            hwnd,
            "BUTTON",
            "一键接入 ZCode",
            BS_OWNERDRAW as u32,
            ID_SETUP,
            font_bold,
        )
    };
    c.login_antigravity = unsafe {
        create_control(
            hwnd,
            "BUTTON",
            "登录 Antigravity",
            BS_OWNERDRAW as u32,
            ID_LOGIN_ANTIGRAVITY,
            font,
        )
    };
    c.login_grok = unsafe {
        create_control(
            hwnd,
            "BUTTON",
            "登录 Grok / xAI",
            BS_OWNERDRAW as u32,
            ID_LOGIN_GROK,
            font,
        )
    };
    c.sync = unsafe {
        create_control(
            hwnd,
            "BUTTON",
            "修复并重新同步",
            BS_OWNERDRAW as u32,
            ID_SYNC,
            font,
        )
    };
    c.open_zcode = unsafe {
        create_control(
            hwnd,
            "BUTTON",
            "打开 ZCode",
            BS_OWNERDRAW as u32,
            ID_OPEN_ZCODE,
            font,
        )
    };
    c.stop = unsafe {
        create_control(
            hwnd,
            "BUTTON",
            "停止本地网关",
            BS_OWNERDRAW as u32,
            ID_STOP,
            font,
        )
    };
    c.copy_connectors = unsafe {
        create_control(
            hwnd,
            "BUTTON",
            "复制当前 Agent 配置",
            BS_OWNERDRAW as u32,
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
    unsafe {
        SendMessageW(
            c.quota_progress as HWND,
            PBM_SETBARCOLOR,
            0,
            COLOR_PRIMARY as LPARAM,
        );
        SendMessageW(
            c.quota_progress as HWND,
            PBM_SETBKCOLOR,
            0,
            COLOR_PRIMARY_SOFT as LPARAM,
        );
    }
    let mut state = state_lock.lock().unwrap();
    state.dpi = dpi;
    state.font = font;
    state.font_bold = font_bold;
    state.font_title = font_title;
    state.controls = controls;
}

#[derive(Clone, Copy)]
struct LayoutMetrics {
    width: i32,
    height: i32,
    dpi: u32,
    sidebar_w: i32,
    main_x: i32,
    main_w: i32,
    gap: i32,
    provider_y: i32,
    provider_w: i32,
    provider_h: i32,
    status_y: i32,
    status_h: i32,
    status_w: i32,
    content_top: i32,
    content_bottom: i32,
    left_w: i32,
    action_x: i32,
    action_w: i32,
    button_h: i32,
    button_gap: i32,
    compact: bool,
}

unsafe fn layout_metrics(hwnd: HWND, dpi: u32) -> LayoutMetrics {
    let mut rect = RECT::default();
    unsafe { GetClientRect(hwnd, &mut rect) };
    let width = rect.right.max(1);
    let height = rect.bottom.max(1);
    let compact = height < scale(680, dpi) || width < scale(1000, dpi);
    let sidebar_w = scale(if compact { 190 } else { 210 }, dpi).min(width / 3);
    let margin = scale(if compact { 16 } else { 26 }, dpi);
    let gap = scale(12, dpi);
    let main_x = sidebar_w + margin;
    let main_w = (width - sidebar_w - margin * 2).max(1);
    let provider_y = scale(if compact { 64 } else { 76 }, dpi);
    let provider_w = scale(if compact { 138 } else { 150 }, dpi);
    let provider_h = scale(if compact { 32 } else { 38 }, dpi);
    let status_y = scale(if compact { 106 } else { 126 }, dpi);
    let status_h = scale(if compact { 62 } else { 76 }, dpi);
    let status_w = (main_w - gap * 3) / 4;
    let content_top = scale(if compact { 180 } else { 222 }, dpi);
    let content_bottom = height - scale(if compact { 14 } else { 24 }, dpi);
    let action_w = scale(284, dpi).min(main_w * 36 / 100);
    let action_x = main_x + main_w - action_w;
    let left_w = action_x - gap - main_x;
    LayoutMetrics {
        width,
        height,
        dpi,
        sidebar_w,
        main_x,
        main_w,
        gap,
        provider_y,
        provider_w,
        provider_h,
        status_y,
        status_h,
        status_w,
        content_top,
        content_bottom,
        left_w,
        action_x,
        action_w,
        button_h: scale(if compact { 30 } else { 40 }, dpi),
        button_gap: scale(if compact { 4 } else { 9 }, dpi),
        compact,
    }
}

unsafe fn layout(hwnd: HWND) {
    let Some(state_lock) = STATE.get() else {
        return;
    };
    let (dpi, controls) = {
        let state = state_lock.lock().unwrap();
        (state.dpi, state.controls)
    };
    let m = unsafe { layout_metrics(hwnd, dpi) };
    let inset = scale(20, m.dpi);
    let models_h = scale(76, m.dpi);
    let models_y = m.content_bottom - inset - models_h;

    let move_control = |handle: isize, x: i32, y: i32, w: i32, h: i32| unsafe {
        if handle != 0 {
            MoveWindow(handle as HWND, x, y, w.max(1), h.max(1), 1)
        } else {
            0
        }
    };
    move_control(
        controls.subtitle,
        m.main_x,
        scale(48, m.dpi),
        m.main_w,
        scale(22, m.dpi),
    );
    move_control(
        controls.provider_antigravity,
        m.main_x,
        m.provider_y,
        m.provider_w,
        m.provider_h,
    );
    move_control(
        controls.provider_grok,
        m.main_x + m.provider_w + m.gap,
        m.provider_y,
        m.provider_w,
        m.provider_h,
    );
    move_control(
        controls.refresh,
        m.main_x + m.main_w - scale(92, m.dpi),
        m.provider_y,
        scale(92, m.dpi),
        m.provider_h,
    );
    for (index, handle) in [
        controls.status_tun,
        controls.status_proxy,
        controls.status_bridge,
        controls.status_zcode,
    ]
    .iter()
    .enumerate()
    {
        move_control(
            *handle,
            m.main_x + index as i32 * (m.status_w + m.gap) + scale(14, m.dpi),
            m.status_y + scale(13, m.dpi),
            m.status_w - scale(28, m.dpi),
            m.status_h - scale(24, m.dpi),
        );
    }
    move_control(
        controls.quota_title,
        m.main_x + inset,
        m.content_top + scale(18, m.dpi),
        m.left_w - inset * 2,
        scale(34, m.dpi),
    );
    move_control(
        controls.models,
        m.main_x + inset,
        models_y,
        m.left_w - inset * 2,
        models_h,
    );
    unsafe {
        ShowWindow(
            controls.models as HWND,
            if m.compact { SW_HIDE } else { SW_SHOW },
        )
    };
    move_control(
        controls.action_header,
        m.action_x + inset,
        m.content_top + scale(18, m.dpi),
        m.action_w - inset * 2,
        scale(34, m.dpi),
    );
    for (index, handle) in [
        controls.setup,
        controls.login_antigravity,
        controls.login_grok,
        controls.sync,
        controls.open_zcode,
        controls.stop,
        controls.copy_connectors,
    ]
    .iter()
    .enumerate()
    {
        move_control(
            *handle,
            m.action_x + inset,
            m.content_top
                + scale(if m.compact { 54 } else { 62 }, m.dpi)
                + index as i32 * (m.button_h + m.button_gap),
            m.action_w - inset * 2,
            m.button_h,
        );
    }
    move_control(
        controls.footer,
        scale(20, m.dpi),
        m.height - scale(55, m.dpi),
        m.sidebar_w - scale(40, m.dpi),
        scale(36, m.dpi),
    );
}

unsafe fn recreate_fonts(hwnd: HWND, dpi: u32) {
    let Some(state_lock) = STATE.get() else {
        return;
    };
    let new_font = unsafe { create_font(10, FW_NORMAL as i32, dpi) };
    let new_bold = unsafe { create_font(10, FW_SEMIBOLD as i32, dpi) };
    let new_title = unsafe { create_font(17, FW_SEMIBOLD as i32, dpi) };
    let (controls, old_font, old_bold, old_title) = {
        let mut state = state_lock.lock().unwrap();
        let old_font = std::mem::replace(&mut state.font, new_font);
        let old_bold = std::mem::replace(&mut state.font_bold, new_bold);
        let old_title = std::mem::replace(&mut state.font_title, new_title);
        state.dpi = dpi;
        (state.controls, old_font, old_bold, old_title)
    };
    let normal_controls = [
        controls.subtitle,
        controls.refresh,
        controls.status_tun,
        controls.status_proxy,
        controls.status_bridge,
        controls.status_zcode,
        controls.quota_body,
        controls.models,
        controls.login_antigravity,
        controls.login_grok,
        controls.sync,
        controls.open_zcode,
        controls.stop,
        controls.copy_connectors,
        controls.footer,
    ];
    let bold_controls = [
        controls.provider_antigravity,
        controls.provider_grok,
        controls.setup,
    ];
    let title_controls = [controls.quota_title, controls.action_header];
    for handle in normal_controls {
        if handle != 0 {
            unsafe { SendMessageW(handle as HWND, WM_SETFONT, new_font as WPARAM, 0) };
        }
    }
    for handle in bold_controls {
        if handle != 0 {
            unsafe { SendMessageW(handle as HWND, WM_SETFONT, new_bold as WPARAM, 0) };
        }
    }
    for handle in title_controls {
        if handle != 0 {
            unsafe { SendMessageW(handle as HWND, WM_SETFONT, new_title as WPARAM, 0) };
        }
    }
    unsafe {
        if old_font != 0 {
            DeleteObject(old_font as HGDIOBJ);
        }
        if old_bold != 0 {
            DeleteObject(old_bold as HGDIOBJ);
        }
        if old_title != 0 {
            DeleteObject(old_title as HGDIOBJ);
        }
    }
    unsafe { InvalidateRect(hwnd, null(), 1) };
}

unsafe fn fill_color(hdc: HDC, rect: &RECT, color: COLORREF) {
    let brush = unsafe { CreateSolidBrush(color) };
    unsafe {
        FillRect(hdc, rect, brush);
        DeleteObject(brush as HGDIOBJ);
    }
}

unsafe fn rounded_box(hdc: HDC, rect: &RECT, fill: COLORREF, border: COLORREF, radius: i32) {
    let brush = unsafe { CreateSolidBrush(fill) };
    let pen = unsafe { CreatePen(PS_SOLID, 1, border) };
    let old_brush = unsafe { SelectObject(hdc, brush as HGDIOBJ) };
    let old_pen = unsafe { SelectObject(hdc, pen as HGDIOBJ) };
    unsafe {
        RoundRect(
            hdc,
            rect.left,
            rect.top,
            rect.right,
            rect.bottom,
            radius,
            radius,
        );
        SelectObject(hdc, old_brush);
        SelectObject(hdc, old_pen);
        DeleteObject(brush as HGDIOBJ);
        DeleteObject(pen as HGDIOBJ);
    }
}

unsafe fn draw_label(
    hdc: HDC,
    text: &str,
    mut rect: RECT,
    font: isize,
    color: COLORREF,
    format: u32,
) {
    let text = wide(text);
    let old_font = unsafe { SelectObject(hdc, font as HGDIOBJ) };
    unsafe {
        SetBkMode(hdc, TRANSPARENT as i32);
        SetTextColor(hdc, color);
        DrawTextW(hdc, text.as_ptr(), -1, &mut rect, format);
        SelectObject(hdc, old_font);
    }
}

fn short_iso_time(value: &str) -> String {
    if value.len() >= 19 && value.as_bytes().get(10) == Some(&b'T') {
        format!("{}-{} {}", &value[5..7], &value[8..10], &value[11..16])
    } else {
        value.to_string()
    }
}

fn quota_percent_color(percent: f64) -> COLORREF {
    if percent < 20.0 {
        COLOR_DANGER
    } else if percent < 50.0 {
        COLOR_WARNING
    } else {
        COLOR_SUCCESS
    }
}

fn quota_lowest(report: &QuotaReport) -> Option<f64> {
    report
        .accounts
        .iter()
        .flat_map(|account| account.groups.as_deref().unwrap_or(&[]))
        .flat_map(|group| &group.buckets)
        .filter_map(|bucket| bucket.remaining_percent)
        .reduce(f64::min)
}

fn format_integer(value: i64) -> String {
    let negative = value < 0;
    let digits = value.unsigned_abs().to_string();
    let mut output = String::with_capacity(digits.len() + digits.len() / 3 + usize::from(negative));
    if negative {
        output.push('-');
    }
    for (index, ch) in digits.chars().enumerate() {
        if index > 0 && (digits.len() - index) % 3 == 0 {
            output.push(',');
        }
        output.push(ch);
    }
    output
}

unsafe fn paint_quota_dashboard(hdc: HDC, state: &AppState, m: &LayoutMetrics) {
    let inset = scale(20, m.dpi);
    let content_left = m.main_x + inset;
    let content_right = m.main_x + m.left_w - inset;
    let summary_top = m.content_top + scale(60, m.dpi);
    let summary_height = scale(if m.compact { 48 } else { 58 }, m.dpi);
    let summary_gap = scale(8, m.dpi);
    let summary_width = (content_right - content_left - summary_gap * 4) / 5;
    let report = state.quota.as_ref();
    let account_count = report.map(|value| value.accounts.len()).unwrap_or(0);
    let lowest = report.and_then(quota_lowest);
    let refresh_text = report
        .and_then(|value| value.fetched_at.as_deref())
        .and_then(|value| value.get(11..19))
        .unwrap_or("等待首次刷新");
    let latest_usage = state.usage.as_ref().and_then(|usage| usage.latest.as_ref());
    let speed_label = if latest_usage
        .map(|sample| sample.speed_basis.as_str())
        == Some("generation")
    {
        "生成速度"
    } else {
        "有效吞吐"
    };
    let summary = [
        ("账号", account_count.to_string()),
        (
            "最低剩余",
            lowest
                .map(|value| format!("{value:.0}%"))
                .unwrap_or_else(|| "—".to_string()),
        ),
        (
            "最近输出",
            latest_usage
                .map(|sample| format!("{} tok", format_integer(sample.output_tokens)))
                .unwrap_or_else(|| "—".to_string()),
        ),
        (
            speed_label,
            latest_usage
                .map(|sample| format!("{:.1} tok/s", sample.output_tokens_per_second))
                .unwrap_or_else(|| "—".to_string()),
        ),
        (
            "本地累计",
            state
                .usage
                .as_ref()
                .filter(|usage| usage.available)
                .map(|usage| format!("{} tok", format_integer(usage.total.output_tokens)))
                .unwrap_or_else(|| "—".to_string()),
        ),
    ];
    for (index, (label, value)) in summary.iter().enumerate() {
        let left = content_left + index as i32 * (summary_width + summary_gap);
        let card = RECT {
            left,
            top: summary_top,
            right: left + summary_width,
            bottom: summary_top + summary_height,
        };
        unsafe {
            rounded_box(hdc, &card, COLOR_BACKGROUND, COLOR_BORDER, scale(5, m.dpi));
            draw_label(
                hdc,
                label,
                RECT {
                    left: left + scale(11, m.dpi),
                    top: summary_top + scale(7, m.dpi),
                    right: card.right - scale(8, m.dpi),
                    bottom: summary_top + scale(25, m.dpi),
                },
                state.font,
                COLOR_MUTED,
                DT_LEFT | DT_VCENTER | DT_SINGLELINE,
            );
            draw_label(
                hdc,
                value,
                RECT {
                    left: left + scale(11, m.dpi),
                    top: summary_top + scale(25, m.dpi),
                    right: card.right - scale(8, m.dpi),
                    bottom: card.bottom - scale(5, m.dpi),
                },
                state.font_bold,
                if index == 1 {
                    lowest.map(quota_percent_color).unwrap_or(COLOR_TEXT)
                } else if index == 3 {
                    COLOR_PRIMARY
                } else {
                    COLOR_TEXT
                },
                DT_LEFT | DT_VCENTER | DT_SINGLELINE,
            );
        }
    }

    let meta_top = summary_top + summary_height + scale(9, m.dpi);
    let source = if state.quota_error.is_some() {
        "刷新失败，保留上次数据"
    } else {
        report
            .map(|value| {
                if value.stale {
                    "缓存数据"
                } else if value.source.is_empty() {
                    "实时接口"
                } else {
                    "实时接口"
                }
            })
            .unwrap_or("等待数据")
    };
    let meta = format!("{source} · 额度刷新 {refresh_text} / 每 5 分钟 · Token 统计每 5 秒更新");
    unsafe {
        draw_label(
            hdc,
            &meta,
            RECT {
                left: content_left,
                top: meta_top,
                right: content_right,
                bottom: meta_top + scale(22, m.dpi),
            },
            state.font,
            COLOR_MUTED,
            DT_LEFT | DT_VCENTER | DT_SINGLELINE | DT_END_ELLIPSIS,
        );
    }

    let usage_meta = if let Some(usage) = state.usage.as_ref() {
        let warning = usage
            .warning
            .as_deref()
            .filter(|value| !value.is_empty())
            .map(|value| format!(" · {value}"))
            .unwrap_or_default();
        if let Some(latest) = usage.latest.as_ref() {
            let basis = if latest.speed_basis == "generation" {
                format!(
                    "生成 {:.1}s · 首字节 {:.1}s",
                    latest.generation_ms as f64 / 1000.0,
                    latest.ttft_ms as f64 / 1000.0
                )
            } else {
                format!("完整调用 {:.1}s（有效吞吐）", latest.latency_ms as f64 / 1000.0)
            };
            format!(
                "{} · 输出 {} token（推理 {}）· {} · 本地 {} 次平均 {:.1} tok/s / 累计推理 {} · {}{}",
                latest.model,
                format_integer(latest.output_tokens),
                format_integer(latest.reasoning_tokens),
                basis,
                usage.total.requests,
                usage.total.average_tokens_per_second,
                format_integer(usage.total.reasoning_tokens),
                short_iso_time(&latest.timestamp),
                warning
            )
        } else {
            format!("Token 统计已启用，等待首次成功的模型响应{warning}")
        }
    } else {
        "Token 统计正在连接本地网关…".to_string()
    };
    unsafe {
        draw_label(
            hdc,
            &usage_meta,
            RECT {
                left: content_left,
                top: meta_top + scale(20, m.dpi),
                right: content_right,
                bottom: meta_top + scale(42, m.dpi),
            },
            state.font,
            COLOR_MUTED,
            DT_LEFT | DT_VCENTER | DT_SINGLELINE | DT_END_ELLIPSIS,
        );
    }

    let models_y = m.content_bottom - inset - scale(76, m.dpi);
    let rows_bottom = if m.compact {
        m.content_bottom - inset
    } else {
        models_y - scale(8, m.dpi)
    };
    let mut row_top = meta_top + scale(50, m.dpi);
    let row_height = scale(if m.compact { 42 } else { 55 }, m.dpi);
    let mut rows_drawn = 0usize;
    if let Some(report) = report {
        'accounts: for account in &report.accounts {
            let plan = account.plan.as_deref().unwrap_or(&account.status);
            let account_badge = account
                .credits
                .as_ref()
                .filter(|credits| credits.available)
                .map(|credits| format!("{plan} · {:.2} {}", credits.amount, credits.credit_type))
                .unwrap_or_else(|| plan.to_string());
            if row_top + scale(27, m.dpi) >= rows_bottom {
                break;
            }
            unsafe {
                draw_label(
                    hdc,
                    &account.account,
                    RECT {
                        left: content_left,
                        top: row_top,
                        right: content_right - scale(120, m.dpi),
                        bottom: row_top + scale(24, m.dpi),
                    },
                    state.font_bold,
                    COLOR_TEXT,
                    DT_LEFT | DT_VCENTER | DT_SINGLELINE | DT_END_ELLIPSIS,
                );
                draw_label(
                    hdc,
                    &account_badge,
                    RECT {
                        left: content_right - scale(120, m.dpi),
                        top: row_top,
                        right: content_right,
                        bottom: row_top + scale(24, m.dpi),
                    },
                    state.font,
                    COLOR_MUTED,
                    DT_RIGHT | DT_VCENTER | DT_SINGLELINE | DT_END_ELLIPSIS,
                );
            }
            row_top += scale(27, m.dpi);
            for group in account.groups.as_deref().unwrap_or(&[]) {
                for bucket in &group.buckets {
                    if row_top + row_height > rows_bottom {
                        break 'accounts;
                    }
                    let percent = bucket.remaining_percent.unwrap_or(0.0).clamp(0.0, 100.0);
                    let reset = bucket
                        .reset_time
                        .as_deref()
                        .map(short_iso_time)
                        .map(|value| format!("重置 {value}"))
                        .unwrap_or_else(|| "重置时间待同步".to_string());
                    let label = if group.name.is_empty() {
                        bucket.name.clone()
                    } else {
                        format!("{} · {}", group.name, bucket.name)
                    };
                    unsafe {
                        draw_label(
                            hdc,
                            &label,
                            RECT {
                                left: content_left,
                                top: row_top,
                                right: content_right - scale(160, m.dpi),
                                bottom: row_top + scale(22, m.dpi),
                            },
                            state.font,
                            COLOR_TEXT,
                            DT_LEFT | DT_VCENTER | DT_SINGLELINE | DT_END_ELLIPSIS,
                        );
                        draw_label(
                            hdc,
                            &reset,
                            RECT {
                                left: content_right - scale(230, m.dpi),
                                top: row_top,
                                right: content_right - scale(48, m.dpi),
                                bottom: row_top + scale(22, m.dpi),
                            },
                            state.font,
                            COLOR_MUTED,
                            DT_RIGHT | DT_VCENTER | DT_SINGLELINE | DT_END_ELLIPSIS,
                        );
                        draw_label(
                            hdc,
                            &format!("{percent:.0}%"),
                            RECT {
                                left: content_right - scale(46, m.dpi),
                                top: row_top,
                                right: content_right,
                                bottom: row_top + scale(22, m.dpi),
                            },
                            state.font_bold,
                            quota_percent_color(percent),
                            DT_RIGHT | DT_VCENTER | DT_SINGLELINE,
                        );
                        let track = RECT {
                            left: content_left,
                            top: row_top + scale(29, m.dpi),
                            right: content_right,
                            bottom: row_top + scale(if m.compact { 36 } else { 38 }, m.dpi),
                        };
                        rounded_box(hdc, &track, COLOR_TRACK, COLOR_TRACK, scale(5, m.dpi));
                        let fill_width =
                            ((track.right - track.left) as f64 * percent / 100.0).round() as i32;
                        if fill_width > 0 {
                            let fill = RECT {
                                right: track.left + fill_width.max(scale(5, m.dpi)),
                                ..track
                            };
                            rounded_box(
                                hdc,
                                &fill,
                                quota_percent_color(percent),
                                quota_percent_color(percent),
                                scale(5, m.dpi),
                            );
                        }
                    }
                    rows_drawn += 1;
                    row_top += row_height;
                }
            }
        }
    }
    if rows_drawn == 0 {
        let detail = state
            .quota_error
            .as_deref()
            .or_else(|| {
                report.and_then(|value| {
                    value.accounts.iter().find_map(|account| {
                        account
                            .error
                            .as_deref()
                            .or(account.status_message.as_deref())
                    })
                })
            })
            .or_else(|| report.and_then(|value| value.warning.as_deref()))
            .unwrap_or("登录所选提供商并完成接入后，这里会显示真实剩余额度。");
        unsafe {
            rounded_box(
                hdc,
                &RECT {
                    left: content_left,
                    top: row_top,
                    right: content_right,
                    bottom: (row_top + scale(78, m.dpi)).min(rows_bottom),
                },
                COLOR_BACKGROUND,
                COLOR_BORDER,
                scale(6, m.dpi),
            );
            draw_label(
                hdc,
                "等待额度数据",
                RECT {
                    left: content_left + scale(14, m.dpi),
                    top: row_top + scale(10, m.dpi),
                    right: content_right - scale(14, m.dpi),
                    bottom: row_top + scale(35, m.dpi),
                },
                state.font_bold,
                COLOR_TEXT,
                DT_LEFT | DT_VCENTER | DT_SINGLELINE,
            );
            draw_label(
                hdc,
                detail,
                RECT {
                    left: content_left + scale(14, m.dpi),
                    top: row_top + scale(35, m.dpi),
                    right: content_right - scale(14, m.dpi),
                    bottom: (row_top + scale(68, m.dpi)).min(rows_bottom),
                },
                state.font,
                COLOR_MUTED,
                DT_LEFT | DT_TOP | DT_WORDBREAK | DT_END_ELLIPSIS,
            );
        }
    }
}

unsafe fn paint_dashboard(hwnd: HWND) {
    let Some(state_lock) = STATE.get() else {
        return;
    };
    let mut paint = PAINTSTRUCT::default();
    let hdc = unsafe { BeginPaint(hwnd, &mut paint) };
    let state = state_lock.lock().unwrap();
    let m = unsafe { layout_metrics(hwnd, state.dpi) };
    let client = RECT {
        left: 0,
        top: 0,
        right: m.width,
        bottom: m.height,
    };
    unsafe { fill_color(hdc, &client, COLOR_BACKGROUND) };
    let sidebar = RECT {
        left: 0,
        top: 0,
        right: m.sidebar_w,
        bottom: m.height,
    };
    unsafe { fill_color(hdc, &sidebar, COLOR_SIDEBAR) };

    let grid_pen = unsafe { CreatePen(PS_SOLID, 1, COLOR_GRID) };
    let old_pen = unsafe { SelectObject(hdc, grid_pen as HGDIOBJ) };
    let grid = scale(48, m.dpi).max(1);
    let mut x = m.sidebar_w;
    while x < m.width {
        unsafe {
            MoveToEx(hdc, x, 0, null_mut());
            LineTo(hdc, x, m.height);
        }
        x += grid;
    }
    let mut y = 0;
    while y < m.height {
        unsafe {
            MoveToEx(hdc, m.sidebar_w, y, null_mut());
            LineTo(hdc, m.width, y);
        }
        y += grid;
    }
    unsafe {
        SelectObject(hdc, old_pen);
        DeleteObject(grid_pen as HGDIOBJ);
    }

    let logo = RECT {
        left: scale(20, m.dpi),
        top: scale(26, m.dpi),
        right: scale(60, m.dpi),
        bottom: scale(66, m.dpi),
    };
    unsafe { rounded_box(hdc, &logo, COLOR_SIDEBAR, COLOR_PRIMARY, scale(4, m.dpi)) };
    unsafe {
        draw_label(
            hdc,
            "ZA",
            logo,
            state.font_bold,
            COLOR_LIGHT_TEXT,
            DT_CENTER | DT_VCENTER | DT_SINGLELINE,
        );
        draw_label(
            hdc,
            "ZCode Bridge",
            RECT {
                left: scale(72, m.dpi),
                top: scale(25, m.dpi),
                right: m.sidebar_w - scale(12, m.dpi),
                bottom: scale(49, m.dpi),
            },
            state.font_bold,
            COLOR_CARD,
            DT_LEFT | DT_VCENTER | DT_SINGLELINE,
        );
        draw_label(
            hdc,
            "Antigravity Control",
            RECT {
                left: scale(72, m.dpi),
                top: scale(48, m.dpi),
                right: m.sidebar_w - scale(10, m.dpi),
                bottom: scale(70, m.dpi),
            },
            state.font,
            COLOR_MUTED,
            DT_LEFT | DT_VCENTER | DT_SINGLELINE,
        );
        draw_label(
            hdc,
            "控制中心",
            RECT {
                left: scale(24, m.dpi),
                top: scale(112, m.dpi),
                right: m.sidebar_w,
                bottom: scale(138, m.dpi),
            },
            state.font,
            COLOR_MUTED,
            DT_LEFT | DT_VCENTER | DT_SINGLELINE,
        );
    }
    let active_nav = RECT {
        left: scale(16, m.dpi),
        top: scale(146, m.dpi),
        right: m.sidebar_w - scale(12, m.dpi),
        bottom: scale(190, m.dpi),
    };
    unsafe {
        rounded_box(
            hdc,
            &active_nav,
            COLOR_SIDEBAR_ACTIVE,
            COLOR_SIDEBAR_ACTIVE,
            scale(4, m.dpi),
        )
    };
    unsafe {
        draw_label(
            hdc,
            "●   模型与额度",
            active_nav,
            state.font_bold,
            COLOR_CARD,
            DT_CENTER | DT_VCENTER | DT_SINGLELINE,
        );
        draw_label(
            hdc,
            "●   连接状态",
            RECT {
                left: scale(28, m.dpi),
                top: scale(199, m.dpi),
                right: m.sidebar_w,
                bottom: scale(235, m.dpi),
            },
            state.font,
            COLOR_MUTED,
            DT_LEFT | DT_VCENTER | DT_SINGLELINE,
        );
        draw_label(
            hdc,
            "●   本机操作",
            RECT {
                left: scale(28, m.dpi),
                top: scale(239, m.dpi),
                right: m.sidebar_w,
                bottom: scale(275, m.dpi),
            },
            state.font,
            COLOR_MUTED,
            DT_LEFT | DT_VCENTER | DT_SINGLELINE,
        );
        draw_label(
            hdc,
            "本地安全边界",
            RECT {
                left: scale(20, m.dpi),
                top: m.height - scale(86, m.dpi),
                right: m.sidebar_w - scale(16, m.dpi),
                bottom: m.height - scale(60, m.dpi),
            },
            state.font,
            COLOR_MUTED,
            DT_LEFT | DT_VCENTER | DT_SINGLELINE,
        );
        draw_label(
            hdc,
            "模型与额度",
            RECT {
                left: m.main_x,
                top: scale(14, m.dpi),
                right: m.main_x + m.main_w,
                bottom: scale(48, m.dpi),
            },
            state.font_title,
            COLOR_TEXT,
            DT_LEFT | DT_VCENTER | DT_SINGLELINE,
        );
    }

    for index in 0..4 {
        let left = m.main_x + index * (m.status_w + m.gap);
        let card = RECT {
            left,
            top: m.status_y,
            right: left + m.status_w,
            bottom: m.status_y + m.status_h,
        };
        unsafe { rounded_box(hdc, &card, COLOR_CARD, COLOR_BORDER, scale(5, m.dpi)) };
    }
    let quota_card = RECT {
        left: m.main_x,
        top: m.content_top,
        right: m.main_x + m.left_w,
        bottom: m.content_bottom,
    };
    let action_card = RECT {
        left: m.action_x,
        top: m.content_top,
        right: m.action_x + m.action_w,
        bottom: m.content_bottom,
    };
    unsafe {
        rounded_box(hdc, &quota_card, COLOR_CARD, COLOR_BORDER, scale(7, m.dpi));
        rounded_box(hdc, &action_card, COLOR_CARD, COLOR_BORDER, scale(7, m.dpi));
        paint_quota_dashboard(hdc, &state, &m);
    }
    drop(state);
    unsafe { EndPaint(hwnd, &paint) };
}

unsafe fn draw_owner_button(draw: &DRAWITEMSTRUCT) {
    let id = draw.CtlID as i32;
    let pressed = draw.itemState & ODS_SELECTED != 0;
    let disabled = draw.itemState & ODS_DISABLED != 0;
    let provider_selected = STATE
        .get()
        .map(|state| {
            let provider = &state.lock().unwrap().provider;
            (id == ID_PROVIDER_ANTIGRAVITY && provider != "xai")
                || (id == ID_PROVIDER_GROK && provider == "xai")
        })
        .unwrap_or(false);
    let primary = id == ID_SETUP;
    let (fill, border, text_color) = if disabled {
        (COLOR_BACKGROUND, COLOR_BORDER, COLOR_MUTED)
    } else if primary {
        (
            if pressed {
                COLOR_PRIMARY_DARK
            } else {
                COLOR_PRIMARY
            },
            COLOR_PRIMARY,
            COLOR_CARD,
        )
    } else if provider_selected {
        (
            if pressed {
                COLOR_BORDER
            } else {
                COLOR_PRIMARY_SOFT
            },
            COLOR_PRIMARY,
            COLOR_PRIMARY,
        )
    } else {
        (
            if pressed {
                COLOR_PRIMARY_SOFT
            } else {
                COLOR_CARD
            },
            COLOR_BORDER,
            COLOR_TEXT,
        )
    };
    unsafe { rounded_box(draw.hDC, &draw.rcItem, fill, border, 5) };
    let mut label = [0u16; 256];
    let length = unsafe { GetWindowTextW(draw.hwndItem, label.as_mut_ptr(), label.len() as i32) };
    let text = String::from_utf16_lossy(&label[..length.max(0) as usize]);
    let font = if primary || provider_selected {
        STATE
            .get()
            .map(|state| state.lock().unwrap().font_bold)
            .unwrap_or(0)
    } else {
        STATE
            .get()
            .map(|state| state.lock().unwrap().font)
            .unwrap_or(0)
    };
    unsafe {
        draw_label(
            draw.hDC,
            &text,
            draw.rcItem,
            font,
            text_color,
            DT_CENTER | DT_VCENTER | DT_SINGLELINE,
        )
    };
}

unsafe fn static_control_color(hdc: HDC, control: HWND) -> LRESULT {
    let Some(state_lock) = STATE.get() else {
        return 0;
    };
    let state = state_lock.lock().unwrap();
    let handle = control as isize;
    let color = if handle == state.controls.footer {
        COLOR_LIGHT_TEXT
    } else if handle == state.controls.subtitle {
        COLOR_MUTED
    } else {
        COLOR_TEXT
    };
    unsafe {
        SetBkMode(hdc, TRANSPARENT as i32);
        SetTextColor(hdc, color);
        GetStockObject(NULL_BRUSH) as LRESULT
    }
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

fn request_refresh(hwnd: HWND, force_quota: bool) {
    let Some(state_lock) = STATE.get() else {
        return;
    };
    let (connection, hwnd_value, fetch_quota, requested_provider) = {
        let mut state = state_lock.lock().unwrap();
        if state.refreshing {
            if force_quota {
                state.refresh_again = true;
            }
            return;
        }
        state.refreshing = true;
        let quota_due = state
            .last_quota_refresh
            .map(|last| last.elapsed() >= QUOTA_REFRESH_INTERVAL)
            .unwrap_or(true);
        (
            state.host.connection.clone(),
            hwnd as isize,
            !state.operation_pending && (force_quota || quota_due),
            state.provider.clone(),
        )
    };
    thread::spawn(move || {
        let status = api_get::<DashboardStatus>(&connection, "/api/status");
        let quota = if fetch_quota {
            Some(api_get::<QuotaReport>(
                &connection,
                &format!("/api/quota?provider={requested_provider}"),
            ))
        } else {
            None
        };
        let usage = api_get::<UsageReport>(
            &connection,
            &format!("/api/usage?provider={requested_provider}"),
        );
        let connectors = api_get::<ConnectorResponse>(&connection, "/api/connectors");
        let update = Box::new(RefreshResult {
            status,
            quota,
            usage,
            connectors,
            requested_provider,
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
    let RefreshResult {
        status,
        quota,
        usage,
        connectors,
        requested_provider,
    } = update;
    let (
        controls,
        hwnd,
        provider,
        operation_running,
        operation_finished,
        quota_for_tray,
        usage_for_tray,
        provider_switching,
        refresh_again,
    ) = {
        let mut state = state_lock.lock().unwrap();
        state.refreshing = false;
        let was_pending = state.operation_pending;
        if let Ok(status) = &status {
            state.provider_accounts = status.provider_accounts.clone();
            if !state.provider_switching {
                state.provider = status.selected_provider.clone();
            }
            state.operation_pending = status.operation.running;
        }
        if let Ok(connectors) = &connectors {
            state.connectors_text = connectors_text(connectors);
        }
        if let Some(result) = &quota {
            if requested_provider == state.provider {
                state.last_quota_refresh = Some(Instant::now());
            }
            match result {
                Ok(report) => {
                    let report_provider = if report.provider == "xai" {
                        "xai"
                    } else {
                        "antigravity"
                    };
                    state
                        .quota_cache
                        .insert(report_provider.to_string(), report.clone());
                    if report_provider == state.provider {
                        state.quota = Some(report.clone());
                        state.quota_error = None;
                    }
                }
                Err(error) if requested_provider == state.provider => {
                    state.quota_error = Some(error.clone())
                }
                Err(_) => {}
            }
        }
        if let Ok(report) = &usage {
            let report_provider = if report.provider == "xai" {
                "xai"
            } else {
                "antigravity"
            };
            state
                .usage_cache
                .insert(report_provider.to_string(), report.clone());
            if report_provider == state.provider {
                state.usage = Some(report.clone());
            }
        }
        let refresh_again = std::mem::take(&mut state.refresh_again);
        (
            state.controls,
            state.hwnd as HWND,
            state.provider.clone(),
            state.operation_pending,
            was_pending && !state.operation_pending,
            state.quota.clone(),
            state.usage.clone(),
            state.provider_switching,
            refresh_again,
        )
    };
    unsafe {
        set_action_controls(controls, !operation_running);
        if operation_running {
            SetTimer(hwnd, TIMER_OPERATION_POLL, 800, None);
        } else {
            KillTimer(hwnd, TIMER_OPERATION_POLL);
        }
    }
    match status {
        Ok(status) => unsafe {
            let operation_message = status
                .operation
                .error
                .as_deref()
                .or(status.operation.message.as_deref());
            set_text(
                controls.subtitle,
                if provider_switching {
                    if provider == "xai" {
                        "正在切换到 Grok / xAI…"
                    } else {
                        "正在切换到 Antigravity…"
                    }
                } else {
                    operation_message.unwrap_or(if status.operation.running {
                        "本地操作正在进行…"
                    } else if status.gateway.ok {
                        "本地安全核心在线"
                    } else {
                        "请选择提供商并执行一键接入"
                    })
                },
            );
            set_text(controls.status_tun, &format_item("TUN", &status.tun));
            set_text(controls.status_proxy, &format_item("PROXY", &status.proxy));
            set_text(
                controls.status_bridge,
                &format_item("BRIDGE", &status.gateway),
            );
            set_text(controls.status_zcode, &format_item("ZCODE", &status.zcode));
            set_text(
                controls.models,
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
                controls.provider_antigravity,
                &format!("Antigravity · {} 个账号", status.provider_accounts.antigravity),
            );
            set_text(
                controls.provider_grok,
                &format!("Grok / xAI · {} 个账号", status.provider_accounts.xai),
            );
            SendMessageW(
                controls.provider_antigravity as HWND,
                BM_SETCHECK,
                if provider == "xai" {
                    BST_UNCHECKED
                } else {
                    BST_CHECKED
                } as WPARAM,
                0,
            );
            SendMessageW(
                controls.provider_grok as HWND,
                BM_SETCHECK,
                if provider == "xai" {
                    BST_CHECKED
                } else {
                    BST_UNCHECKED
                } as WPARAM,
                0,
            );
            set_text(
                controls.quota_title,
                if provider == "xai" {
                    "Grok 共享额度"
                } else {
                    "Gemini 模型额度"
                },
            );
        },
        Err(error) => unsafe {
            set_text(controls.subtitle, &format!("状态刷新失败：{error}"))
        },
    }
    update_tray(
        hwnd,
        &provider,
        quota_for_tray.as_ref(),
        usage_for_tray.as_ref(),
    );
    unsafe {
        InvalidateRect(hwnd, null(), 0);
        InvalidateRect(controls.provider_antigravity as HWND, null(), 1);
        InvalidateRect(controls.provider_grok as HWND, null(), 1);
    }
    if operation_finished || refresh_again {
        request_refresh(hwnd, true);
    }
}

fn run_operation(hwnd: HWND, action: &'static str) {
    let Some(state_lock) = STATE.get() else {
        return;
    };
    let (connection, controls) = {
        let mut state = state_lock.lock().unwrap();
        if state.operation_pending {
            return;
        }
        state.operation_pending = true;
        (state.host.connection.clone(), state.controls)
    };
    unsafe {
        set_text(controls.subtitle, operation_start_message(action));
        set_action_controls(controls, false);
        SetTimer(hwnd, TIMER_OPERATION_POLL, 800, None);
        InvalidateRect(hwnd, null(), 0);
    }
    let hwnd_value = hwnd as isize;
    thread::spawn(move || {
        let update = Box::new(OperationPostResult {
            error: api_post(&connection, "/api/action", json!({"action": action})).err(),
        });
        unsafe {
            let _ = PostMessageW(
                hwnd_value as HWND,
                WM_OPERATION_POSTED,
                0,
                Box::into_raw(update) as LPARAM,
            );
        }
    });
}

fn operation_start_message(action: &str) -> &'static str {
    match action {
        "setup" => "正在启动网关并接入 ZCode…",
        "login" => "正在打开 Antigravity 登录…",
        "login-grok" => "正在打开 Grok / xAI 登录…",
        "sync" => "正在修复并重新同步…",
        "open-zcode" => "正在打开 ZCode…",
        "stop" => "正在停止本地网关…",
        _ => "正在处理…",
    }
}

unsafe fn set_action_controls(controls: Controls, enabled: bool) {
    for handle in [
        controls.setup,
        controls.login_antigravity,
        controls.login_grok,
        controls.sync,
        controls.open_zcode,
        controls.stop,
        controls.copy_connectors,
    ] {
        unsafe {
            EnableWindow(handle as HWND, i32::from(enabled));
            InvalidateRect(handle as HWND, null(), 1);
        }
    }
    unsafe {
        set_text(
            controls.setup,
            if enabled {
                "一键接入 ZCode"
            } else {
                "处理中，请稍候…"
            },
        );
    }
}

fn select_provider(hwnd: HWND, provider: &'static str) {
    let Some(state_lock) = STATE.get() else {
        return;
    };
    let (connection, antigravity, grok, previous) = {
        let mut state = state_lock.lock().unwrap();
        if state.provider_switching || state.provider == provider {
            return;
        }
        let previous = state.provider.clone();
        state.provider_switching = true;
        state.provider = provider.to_string();
        state.quota = state.quota_cache.get(provider).cloned();
        state.usage = state.usage_cache.get(provider).cloned();
        state.quota_error = None;
        state.last_quota_refresh = None;
        (
            state.host.connection.clone(),
            state.controls.provider_antigravity as HWND,
            state.controls.provider_grok as HWND,
            previous,
        )
    };
    unsafe {
        EnableWindow(antigravity, 0);
        EnableWindow(grok, 0);
        InvalidateRect(antigravity, null(), 1);
        InvalidateRect(grok, null(), 1);
        if let Some(state) = STATE.get() {
            let controls = state.lock().unwrap().controls;
            set_text(
                controls.subtitle,
                if provider == "xai" {
                    "正在切换到 Grok / xAI…"
                } else {
                    "正在切换到 Antigravity…"
                },
            );
        }
        InvalidateRect(hwnd, null(), 0);
    }
    let hwnd_value = hwnd as isize;
    thread::spawn(move || {
        let result = Box::new(ProviderSelectResult {
            requested: provider.to_string(),
            previous,
            error: api_post(&connection, "/api/provider", json!({"provider": provider})).err(),
        });
        unsafe {
            let _ = PostMessageW(
                hwnd_value as HWND,
                WM_PROVIDER_READY,
                0,
                Box::into_raw(result) as LPARAM,
            );
        }
    });
}

unsafe fn finish_provider_selection(hwnd: HWND, result: ProviderSelectResult) {
    let Some(state_lock) = STATE.get() else {
        return;
    };
    let (controls, error) = {
        let mut state = state_lock.lock().unwrap();
        state.provider_switching = false;
        if result.error.is_some() {
            state.provider = result.previous.clone();
            state.quota = state.quota_cache.get(&result.previous).cloned();
            state.usage = state.usage_cache.get(&result.previous).cloned();
        } else {
            state.provider = result.requested.clone();
            state.quota = state.quota_cache.get(&result.requested).cloned();
            state.usage = state.usage_cache.get(&result.requested).cloned();
            state.last_quota_refresh = None;
        }
        (state.controls, result.error)
    };
    unsafe {
        EnableWindow(controls.provider_antigravity as HWND, 1);
        EnableWindow(controls.provider_grok as HWND, 1);
        InvalidateRect(controls.provider_antigravity as HWND, null(), 1);
        InvalidateRect(controls.provider_grok as HWND, null(), 1);
        InvalidateRect(hwnd, null(), 0);
    }
    if let Some(error) = error {
        unsafe {
            set_text(controls.subtitle, &format!("提供商切换失败：{error}"));
            MessageBoxW(
                hwnd,
                wide(&format!("提供商切换失败：{error}")).as_ptr(),
                wide("ZCode Antigravity").as_ptr(),
                MB_OK | MB_ICONERROR,
            );
        }
    }
    request_refresh(hwnd, true);
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

#[derive(Clone)]
struct QuotaWindowSummary {
    remaining: f64,
    reset_time: Option<String>,
}

fn quota_window_summary(report: &QuotaReport, kind: &str) -> Option<QuotaWindowSummary> {
    let mut selected: Option<QuotaWindowSummary> = None;
    for account in &report.accounts {
        for group in account.groups.as_deref().unwrap_or(&[]) {
            for bucket in &group.buckets {
                let search = format!("{} {} {}", group.name, bucket.name, bucket.window).to_lowercase();
                let matches = if kind == "five" {
                    search.contains("5小时")
                        || search.contains("5 小时")
                        || search.contains("5-hour")
                        || search.contains("5 hour")
                        || search.contains("5h")
                } else {
                    search.contains("week")
                        || search.contains('周')
                        || search.contains("7-day")
                        || search.contains("7 day")
                        || search.contains("7天")
                };
                let Some(remaining) = bucket.remaining_percent.filter(|_| matches) else {
                    continue;
                };
                if selected
                    .as_ref()
                    .map(|current| remaining < current.remaining)
                    .unwrap_or(true)
                {
                    selected = Some(QuotaWindowSummary {
                        remaining,
                        reset_time: bucket.reset_time.clone(),
                    });
                }
            }
        }
    }
    selected
}

fn quota_window_menu_text(label: &str, value: Option<&QuotaWindowSummary>) -> String {
    match value {
        Some(value) => {
            let reset = value
                .reset_time
                .as_deref()
                .map(short_iso_time)
                .map(|time| format!(" · 重置 {time}"))
                .unwrap_or_default();
            format!("{label}剩余：{:.0}%{reset}", value.remaining)
        }
        None => format!("{label}剩余：当前提供商未提供"),
    }
}

fn update_tray(
    hwnd: HWND,
    provider: &str,
    quota: Option<&QuotaReport>,
    usage: Option<&UsageReport>,
) {
    let mut data: NOTIFYICONDATAW = unsafe { std::mem::zeroed() };
    data.cbSize = std::mem::size_of::<NOTIFYICONDATAW>() as u32;
    data.hWnd = hwnd;
    data.uID = 1;
    data.uFlags = NIF_TIP;
    let provider = if provider == "xai" {
        "Grok"
    } else {
        "Antigravity"
    };
    let five = quota.and_then(|report| quota_window_summary(report, "five"));
    let week = quota.and_then(|report| quota_window_summary(report, "week"));
    let mut parts = vec![format!("ZCode · {provider}")];
    if let Some(five) = five {
        parts.push(format!("5小时 {:.0}%", five.remaining));
    }
    if let Some(week) = week {
        parts.push(format!("本周 {:.0}%", week.remaining));
    }
    if let Some(latest) = usage.and_then(|report| report.latest.as_ref()) {
        parts.push(format!("{:.1} tok/s", latest.output_tokens_per_second));
    }
    if parts.len() == 1 {
        parts.push("额度暂不可用".to_string());
    }
    copy_wide_fixed(&mut data.szTip, &parts.join(" · "));
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
    let (provider, quota, usage, accounts) = STATE
        .get()
        .map(|state| {
            let state = state.lock().unwrap();
            (
                state.provider.clone(),
                state.quota.clone(),
                state.usage.clone(),
                state.provider_accounts.clone(),
            )
        })
        .unwrap_or_default();
    let provider_name = if provider == "xai" { "Grok / xAI" } else { "Antigravity" };
    let account_count = if provider == "xai" { accounts.xai } else { accounts.antigravity };
    let five = quota.as_ref().and_then(|report| quota_window_summary(report, "five"));
    let week = quota.as_ref().and_then(|report| quota_window_summary(report, "week"));
    unsafe {
        AppendMenuW(
            menu,
            MF_STRING | MF_GRAYED,
            0,
            wide(&format!("{provider_name} · {account_count} 个账号")).as_ptr(),
        );
        AppendMenuW(
            menu,
            MF_STRING | MF_GRAYED,
            0,
            wide(&quota_window_menu_text("5 小时", five.as_ref())).as_ptr(),
        );
        AppendMenuW(
            menu,
            MF_STRING | MF_GRAYED,
            0,
            wide(&quota_window_menu_text("本周", week.as_ref())).as_ptr(),
        );
        if let Some(latest) = usage.as_ref().and_then(|report| report.latest.as_ref()) {
            AppendMenuW(
                menu,
                MF_STRING | MF_GRAYED,
                0,
                wide(&format!(
                    "最近输出：{} token · {:.1} token/s",
                    format_integer(latest.output_tokens),
                    latest.output_tokens_per_second
                ))
                .as_ptr(),
            );
        } else {
            AppendMenuW(
                menu,
                MF_STRING | MF_GRAYED,
                0,
                wide("Token 统计：等待首次模型响应").as_ptr(),
            );
        }
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
        AppendMenuW(menu, MF_STRING, ID_TRAY_OPEN, wide("打开控制中心").as_ptr());
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
            ID_TRAY_REFRESH => request_refresh(hwnd, true),
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
        WM_ERASEBKGND => 1,
        WM_PAINT => {
            unsafe { paint_dashboard(hwnd) };
            0
        }
        WM_CTLCOLORSTATIC => unsafe { static_control_color(wparam as HDC, lparam as HWND) },
        WM_DRAWITEM => {
            let draw = lparam as *const DRAWITEMSTRUCT;
            if !draw.is_null() {
                unsafe { draw_owner_button(&*draw) };
                1
            } else {
                0
            }
        }
        WM_CREATE => {
            if let Some(state) = STATE.get() {
                state.lock().unwrap().hwnd = hwnd as isize;
            }
            unsafe {
                create_controls(hwnd);
                layout(hwnd);
                add_tray_icon(hwnd);
                SetTimer(hwnd, TIMER_REFRESH, STATUS_REFRESH_MS, None);
            }
            request_refresh(hwnd, true);
            0
        }
        WM_SIZE => {
            unsafe {
                layout(hwnd);
                InvalidateRect(hwnd, null(), 0);
            }
            0
        }
        WM_DPICHANGED => {
            let dpi = ((wparam >> 16) as u32).max(96);
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
            unsafe {
                recreate_fonts(hwnd, dpi);
                layout(hwnd);
                InvalidateRect(hwnd, null(), 1);
            }
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
                    (*info).ptMinTrackSize.x = scale(760, dpi);
                    (*info).ptMinTrackSize.y = scale(500, dpi);
                }
            }
            0
        }
        WM_COMMAND => {
            match loword(wparam) {
                ID_PROVIDER_ANTIGRAVITY => select_provider(hwnd, "antigravity"),
                ID_PROVIDER_GROK => select_provider(hwnd, "xai"),
                ID_REFRESH => request_refresh(hwnd, true),
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
                        let subtitle = STATE
                            .get()
                            .map(|state| state.lock().unwrap().controls.subtitle)
                            .unwrap_or(0);
                        unsafe {
                            set_text(subtitle, "Agent 配置已复制到剪贴板，请勿发给他人")
                        };
                    }
                }
                _ => {}
            }
            0
        }
        WM_TIMER => {
            request_refresh(hwnd, false);
            0
        }
        WM_OPERATION_POSTED => {
            if lparam != 0 {
                let result = unsafe { Box::from_raw(lparam as *mut OperationPostResult) };
                if let Some(error) = result.error {
                    let controls = STATE.get().map(|state| {
                        let mut state = state.lock().unwrap();
                        state.operation_pending = false;
                        state.controls
                    });
                    unsafe {
                        KillTimer(hwnd, TIMER_OPERATION_POLL);
                        if let Some(controls) = controls {
                            set_action_controls(controls, true);
                            set_text(controls.subtitle, &format!("操作请求失败：{error}"));
                        }
                        MessageBoxW(
                            hwnd,
                            wide(&format!("操作请求失败：{error}")).as_ptr(),
                            wide("ZCode Antigravity").as_ptr(),
                            MB_OK | MB_ICONERROR,
                        );
                    }
                } else {
                    request_refresh(hwnd, false);
                }
            }
            0
        }
        WM_PROVIDER_READY => {
            if lparam != 0 {
                let result = unsafe { Box::from_raw(lparam as *mut ProviderSelectResult) };
                unsafe { finish_provider_selection(hwnd, *result) };
            }
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
                WM_LBUTTONUP => unsafe { show_tray_menu(hwnd) },
                WM_LBUTTONDBLCLK => unsafe {
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
                KillTimer(hwnd, TIMER_OPERATION_POLL);
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
    let auto_setup = std::env::args_os()
        .skip(1)
        .any(|argument| argument.to_string_lossy().eq_ignore_ascii_case("--auto-setup"));
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
            hbrBackground: null_mut(),
            lpszMenuName: null(),
            lpszClassName: class_name.as_ptr(),
            hIconSm: load_app_icon(),
        };
        if RegisterClassExW(&class) == 0 {
            show_fatal("无法注册 Windows 原生窗口");
            return;
        }
        let title = wide(&format!("ZCode · Antigravity 控制中心 · Rust {VERSION}"));
        let window_style = WS_OVERLAPPEDWINDOW;
        let window_ex_style = WS_EX_APPWINDOW;
        let hwnd = CreateWindowExW(
            window_ex_style,
            class_name.as_ptr(),
            title.as_ptr(),
            window_style,
            CW_USEDEFAULT,
            CW_USEDEFAULT,
            900,
            670,
            null_mut(),
            null_mut(),
            instance,
            null_mut::<c_void>(),
        );
        if hwnd.is_null() {
            show_fatal("无法创建 Windows 原生窗口");
            return;
        }
        let window_dpi = GetDpiForWindow(hwnd).max(96);
        let mut desired = RECT {
            left: 0,
            top: 0,
            right: scale(1120, window_dpi),
            bottom: scale(720, window_dpi),
        };
        AdjustWindowRectExForDpi(&mut desired, window_style, 0, window_ex_style, window_dpi);
        let monitor = MonitorFromWindow(hwnd, MONITOR_DEFAULTTONEAREST);
        let mut monitor_info: MONITORINFO = std::mem::zeroed();
        monitor_info.cbSize = std::mem::size_of::<MONITORINFO>() as u32;
        GetMonitorInfoW(monitor, &mut monitor_info);
        let work = monitor_info.rcWork;
        let work_width = work.right - work.left;
        let work_height = work.bottom - work.top;
        let available_width = (work_width - scale(24, window_dpi)).max(1);
        let available_height = (work_height - scale(24, window_dpi)).max(1);
        let minimum_width = scale(900, window_dpi).min(available_width);
        let minimum_height = scale(670, window_dpi).min(available_height);
        let window_width = (desired.right - desired.left)
            .min(available_width)
            .max(minimum_width);
        let window_height = (desired.bottom - desired.top)
            .min(available_height)
            .max(minimum_height);
        ShowWindow(hwnd, SW_SHOW);
        SetWindowPos(
            hwnd,
            null_mut(),
            work.left + (work_width - window_width) / 2,
            work.top + (work_height - window_height) / 2,
            window_width,
            window_height,
            SWP_NOZORDER | SWP_NOACTIVATE,
        );
        UpdateWindow(hwnd);
        if auto_setup {
            PostMessageW(hwnd, WM_COMMAND, ID_SETUP as WPARAM, 0);
        }
        let mut message = MSG::default();
        while GetMessageW(&mut message, null_mut(), 0, 0) > 0 {
            TranslateMessage(&message);
            DispatchMessageW(&message);
        }
    }
}
