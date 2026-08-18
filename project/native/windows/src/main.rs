#![windows_subsystem = "windows"]

mod protocol;

use protocol::NativeConnection;
use serde::Deserialize;
use serde_json::{Value, json};
use std::io::{BufRead, BufReader};
use std::os::windows::process::CommandExt;
use std::process::{Child, ChildStdin, Command, Stdio};
use std::sync::{
    Mutex,
    atomic::{AtomicBool, Ordering},
};
use std::thread;
use std::time::{Duration, Instant};
use tauri::menu::{Menu, MenuItem, PredefinedMenuItem};
use tauri::tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent};
use tauri::window::{Color, Effect, EffectsBuilder};
use tauri::{AppHandle, Manager, PhysicalPosition, PhysicalSize, State, WebviewUrl, WebviewWindowBuilder};
use windows_sys::Win32::System::Threading::CREATE_NO_WINDOW;

const VERSION: &str = "0.6.4-test";
const WIDGET_LABEL: &str = "quota-widget";

struct NativeHost {
    child: Child,
    stdin: Option<ChildStdin>,
}

impl NativeHost {
    fn start() -> Result<(Self, NativeConnection), String> {
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
        Ok((Self { child, stdin }, connection))
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

impl Drop for NativeHost {
    fn drop(&mut self) {
        self.shutdown();
    }
}

struct AppRuntime {
    _host: Mutex<NativeHost>,
    connection: NativeConnection,
    auto_setup: AtomicBool,
}

impl AppRuntime {
    fn start() -> Result<Self, String> {
        let (host, connection) = NativeHost::start()?;
        let auto_setup = std::env::args().any(|argument| argument == "--auto-setup");
        Ok(Self {
            _host: Mutex::new(host),
            connection,
            auto_setup: AtomicBool::new(auto_setup),
        })
    }
}

fn allowed_get_path(path: &str) -> bool {
    matches!(path, "/api/status" | "/api/connectors" | "/api/manager")
        || matches!(
            path,
            "/api/quota?provider=antigravity"
                | "/api/quota?provider=xai"
                | "/api/usage?provider=antigravity"
                | "/api/usage?provider=xai"
        )
}

fn allowed_post_path(path: &str) -> bool {
    matches!(
        path,
        "/api/action" | "/api/provider" | "/api/manager/settings" | "/api/heartbeat" | "/api/close"
    )
}

fn parse_response(mut response: ureq::http::Response<ureq::Body>) -> Result<Value, String> {
    let text = response
        .body_mut()
        .read_to_string()
        .map_err(|error| error.to_string())?;
    if text.trim().is_empty() {
        return Ok(Value::Null);
    }
    serde_json::from_str(&text).map_err(|error| format!("后台返回了无效数据：{error}"))
}

fn request_get(connection: &NativeConnection, path: &str) -> Result<Value, String> {
    if !allowed_get_path(path) {
        return Err("不允许访问该本机接口".to_string());
    }
    let response = ureq::get(format!("{}{}", connection.base_url, path))
        .header("X-ZCAB-Session", &connection.session)
        .call()
        .map_err(|error| error.to_string())?;
    parse_response(response)
}

fn request_post(connection: &NativeConnection, path: &str, body: Value) -> Result<Value, String> {
    if !allowed_post_path(path) {
        return Err("不允许访问该本机接口".to_string());
    }
    let response = ureq::post(format!("{}{}", connection.base_url, path))
        .header("X-ZCAB-Session", &connection.session)
        .send_json(body)
        .map_err(|error| error.to_string())?;
    parse_response(response)
}

#[tauri::command]
async fn api_get(state: State<'_, AppRuntime>, path: String) -> Result<Value, String> {
    let connection = state.connection.clone();
    tauri::async_runtime::spawn_blocking(move || request_get(&connection, &path))
        .await
        .map_err(|error| error.to_string())?
}

#[tauri::command]
async fn api_post(
    state: State<'_, AppRuntime>,
    path: String,
    body: Value,
) -> Result<Value, String> {
    let connection = state.connection.clone();
    tauri::async_runtime::spawn_blocking(move || request_post(&connection, &path, body))
        .await
        .map_err(|error| error.to_string())?
}

#[derive(serde::Serialize)]
#[serde(rename_all = "camelCase")]
struct StartupInfo {
    version: &'static str,
    auto_setup: bool,
}

#[tauri::command]
fn startup_info(state: State<'_, AppRuntime>) -> StartupInfo {
    StartupInfo {
        version: VERSION,
        auto_setup: state.auto_setup.swap(false, Ordering::SeqCst),
    }
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct TraySummary {
    provider: String,
    five_hour: Option<f64>,
    week: Option<f64>,
    tokens_per_second: Option<f64>,
}

#[tauri::command]
fn update_tray_summary(app: AppHandle, summary: TraySummary) -> Result<(), String> {
    let provider = if summary.provider == "xai" {
        "Grok"
    } else {
        "Antigravity"
    };
    let mut parts = vec![format!("ZCode · {provider}")];
    if let Some(value) = summary.five_hour {
        parts.push(format!("5小时 {value:.0}%"));
    }
    if let Some(value) = summary.week {
        parts.push(format!("本周 {value:.0}%"));
    }
    if let Some(value) = summary.tokens_per_second {
        parts.push(format!("{value:.1} tok/s"));
    }
    if parts.len() == 1 {
        parts.push("额度暂不可用".to_string());
    }
    app.tray_by_id("main")
        .ok_or_else(|| "任务栏图标尚未就绪".to_string())?
        .set_tooltip(Some(parts.join(" · ")))
        .map_err(|error| error.to_string())
}

#[tauri::command]
fn open_xai_verification_url(url: String) -> Result<(), String> {
    let parsed: ureq::http::Uri = url.parse().map_err(|_| "xAI 授权地址无效".to_string())?;
    if parsed.scheme_str() != Some("https") || parsed.host() != Some("accounts.x.ai") {
        return Err("只允许打开 xAI 官方授权地址".to_string());
    }
    Command::new("rundll32.exe")
        .arg("url.dll,FileProtocolHandler")
        .arg(url)
        .creation_flags(CREATE_NO_WINDOW)
        .spawn()
        .map_err(|error| format!("无法打开 xAI 授权页：{error}"))?;
    Ok(())
}

fn show_main(app: &AppHandle) {
    if let Some(widget) = app.get_webview_window(WIDGET_LABEL) {
        let _ = widget.hide();
    }
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.unminimize();
        let _ = window.show();
        let _ = window.set_focus();
    }
}

#[tauri::command]
fn show_main_window(app: AppHandle) {
    show_main(&app);
}

fn build_quota_widget(app: &tauri::App) -> tauri::Result<()> {
    WebviewWindowBuilder::new(
        app,
        WIDGET_LABEL,
        WebviewUrl::App("index.html?view=widget".into()),
    )
    .title("ZCode · 当前额度")
    .inner_size(430.0, 368.0)
    .min_inner_size(430.0, 368.0)
    .max_inner_size(430.0, 368.0)
    .resizable(false)
    .decorations(false)
    .transparent(true)
    .background_color(Color(0, 0, 0, 0))
    .shadow(true)
    .skip_taskbar(true)
    .always_on_top(true)
    .focused(false)
    .visible(false)
    .effects(
        EffectsBuilder::new()
            .effect(Effect::Acrylic)
            .color(Color(228, 235, 249, 82))
            .build(),
    )
    .build()?;
    Ok(())
}

fn toggle_quota_widget(app: &AppHandle, click: PhysicalPosition<f64>) {
    let Some(window) = app.get_webview_window(WIDGET_LABEL) else {
        return;
    };
    if window.is_visible().unwrap_or(false) {
        let _ = window.hide();
        return;
    }

    let size = window
        .outer_size()
        .unwrap_or_else(|_| PhysicalSize::new(430, 368));
    if let Ok(Some(monitor)) = app
        .monitor_from_point(click.x, click.y)
        .or_else(|_| app.primary_monitor())
    {
        let area = monitor.work_area();
        let left = area.position.x as f64 + 8.0;
        let top = area.position.y as f64 + 8.0;
        let right = area.position.x as f64 + area.size.width as f64 - size.width as f64 - 8.0;
        let bottom = area.position.y as f64 + area.size.height as f64 - size.height as f64 - 8.0;
        let x = (click.x - size.width as f64 / 2.0).clamp(left, right.max(left));
        let preferred_y = if click.y > area.position.y as f64 + area.size.height as f64 / 2.0 {
            click.y - size.height as f64 - 14.0
        } else {
            click.y + 14.0
        };
        let y = preferred_y.clamp(top, bottom.max(top));
        let _ = window.set_position(PhysicalPosition::new(x.round() as i32, y.round() as i32));
    }
    let _ = window.eval("window.dispatchEvent(new CustomEvent('zcode:refresh'))");
    let _ = window.show();
    let _ = window.set_focus();
}

fn build_tray(app: &tauri::App) -> tauri::Result<()> {
    let show = MenuItem::with_id(app, "show", "显示控制中心", true, None::<&str>)?;
    let refresh = MenuItem::with_id(app, "refresh", "刷新额度", true, None::<&str>)?;
    let quit = MenuItem::with_id(app, "quit", "退出", true, None::<&str>)?;
    let separator = PredefinedMenuItem::separator(app)?;
    let menu = Menu::with_items(app, &[&show, &refresh, &separator, &quit])?;
    let mut builder = TrayIconBuilder::with_id("main")
        .menu(&menu)
        .show_menu_on_left_click(false)
        .tooltip("ZCode · 正在读取额度")
        .on_tray_icon_event(|tray, event| {
            if let TrayIconEvent::Click {
                position,
                button: MouseButton::Left,
                button_state: MouseButtonState::Up,
                ..
            } = event
            {
                toggle_quota_widget(tray.app_handle(), position);
            }
        })
        .on_menu_event(|app, event| match event.id.as_ref() {
            "show" => show_main(app),
            "refresh" => {
                if let Some(window) = app.get_webview_window("main") {
                    let _ = window.eval("window.dispatchEvent(new CustomEvent('zcode:refresh'))");
                }
                if let Some(window) = app.get_webview_window(WIDGET_LABEL) {
                    let _ = window.eval("window.dispatchEvent(new CustomEvent('zcode:refresh'))");
                }
            }
            "quit" => app.exit(0),
            _ => {}
        });
    if let Some(icon) = app.default_window_icon() {
        builder = builder.icon(icon.clone());
    }
    builder.build(app)?;
    Ok(())
}

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| {
            show_main(app)
        }))
        .setup(|app| {
            let runtime = AppRuntime::start().map_err(std::io::Error::other)?;
            app.manage(runtime);
            build_quota_widget(app)?;
            build_tray(app)?;
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            api_get,
            api_post,
            startup_info,
            update_tray_summary,
            show_main_window,
            open_xai_verification_url
        ])
        .on_window_event(|window, event| {
            match event {
                tauri::WindowEvent::CloseRequested { api, .. } => {
                    api.prevent_close();
                    let _ = window.hide();
                }
                tauri::WindowEvent::Focused(false) if window.label() == WIDGET_LABEL => {
                    let _ = window.hide();
                }
                _ => {}
            }
        })
        .run(tauri::generate_context!())
        .unwrap_or_else(|error| {
            let message = json!({ "error": error.to_string() });
            eprintln!("{message}");
        });
}
