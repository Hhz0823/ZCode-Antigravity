"use strict";

const { spawn } = require("node:child_process");
const crypto = require("node:crypto");
const fs = require("node:fs");
const path = require("node:path");
const {
  app,
  BrowserWindow,
  clipboard,
  dialog,
  ipcMain,
  Menu,
  nativeImage,
  nativeTheme,
  net,
  screen,
  shell,
  Tray,
} = require("electron");
const { assertApiPath, assertUpdateInstaller, assertXaiURL, normalizeConnection, trayTooltip } = require("./protocol.cjs");
const { version } = require("../package.json");

const WIDGET_WIDTH = 430;
const WIDGET_HEIGHT = 368;
const rendererIDs = new Set();

let mainWindow;
let quotaWidget;
let tray;
let nativeChild;
let nativeConnection;
let quitting = false;
let autoSetupPending = process.argv.includes("--auto-setup");
let postUpdatePending = process.argv.includes("--post-update");

if (!app.requestSingleInstanceLock()) {
  app.quit();
} else {
  app.enableSandbox();
  app.on("second-instance", () => showMainWindow());
}

function resourcePath(name) {
  const packaged = path.join(process.resourcesPath, name);
  if (fs.existsSync(packaged)) return packaged;
  return path.join(__dirname, "..", "..", "assets", name);
}

function corePath() {
  const explicit = process.env.ZCODE_ANTIGRAVITY_CORE;
  if (explicit) return path.resolve(explicit);
  return path.join(path.dirname(process.execPath), "ZCode-Antigravity.exe");
}

function startNativeHost() {
  return new Promise((resolve, reject) => {
    const executable = corePath();
    if (!fs.existsSync(executable)) {
      reject(new Error(`缺少后台核心：${executable}`));
      return;
    }

    const child = spawn(executable, ["native-host"], {
      cwd: path.dirname(executable),
      windowsHide: true,
      stdio: ["pipe", "pipe", "ignore"],
    });
    nativeChild = child;
    child.stdout.setEncoding("utf8");

    let settled = false;
    let buffer = "";
    const timer = setTimeout(() => finish(new Error("等待后台核心启动超时")), 20_000);

    function finish(error, connection) {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      if (error) {
        child.kill();
        reject(error);
      } else {
        nativeConnection = connection;
        resolve(connection);
      }
    }

    child.once("error", (error) => finish(new Error(`无法启动后台核心：${error.message}`)));
    child.stdout.on("data", (chunk) => {
      if (settled) return;
      buffer += chunk;
      if (buffer.length > 32 * 1024) {
        finish(new Error("后台连接信息异常"));
        return;
      }
      const newline = buffer.indexOf("\n");
      if (newline < 0) return;
      try {
        finish(undefined, normalizeConnection(JSON.parse(buffer.slice(0, newline).trim())));
      } catch (error) {
        finish(new Error(`后台连接信息无法解析：${error.message}`));
      }
    });
    child.once("exit", (code) => {
      nativeConnection = undefined;
      nativeChild = undefined;
      if (!settled) finish(new Error(`后台核心提前退出（${code ?? "unknown"}）`));
      else if (!quitting && tray) tray.setToolTip("ZCode · 本地核心已离线");
    });
  });
}

function stopNativeHost() {
  const child = nativeChild;
  nativeChild = undefined;
  nativeConnection = undefined;
  if (!child) return;
  child.stdin.end();
  const timer = setTimeout(() => {
    if (child.exitCode === null) child.kill();
  }, 2_000);
  timer.unref();
  child.once("exit", () => clearTimeout(timer));
}

function assertRenderer(event) {
  if (!rendererIDs.has(event.sender.id)) throw new Error("拒绝未知窗口访问本机能力");
}

function sanitizeBody(value) {
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("请求正文必须是对象");
  const serialized = JSON.stringify(value);
  if (Buffer.byteLength(serialized, "utf8") > 256 * 1024) throw new Error("请求正文过大");
  return serialized;
}

async function apiRequest(method, apiPath, body) {
  if (!nativeConnection) throw new Error("本地安全核心尚未连接");
  const safePath = assertApiPath(method, apiPath);
  const options = {
    method,
    headers: { "X-ZCAB-Session": nativeConnection.session },
    signal: AbortSignal.timeout(safePath === "/api/update" && method === "POST" ? 20 * 60_000 : safePath.startsWith("/api/quota") ? 120_000 : 30_000),
    cache: "no-store",
  };
  if (method === "POST") {
    options.headers["Content-Type"] = "application/json";
    options.body = sanitizeBody(body ?? {});
  }
  const response = await net.fetch(`${nativeConnection.baseURL}${safePath}`, options);
  const text = await response.text();
  let payload = null;
  if (text.trim()) {
    try { payload = JSON.parse(text); }
    catch { throw new Error("后台返回了无效数据"); }
  }
  if (!response.ok) throw new Error(payload?.error || `后台返回 HTTP ${response.status}`);
  return payload;
}

function registerIPC() {
  ipcMain.handle("api:get", async (event, apiPath) => {
    assertRenderer(event);
    return apiRequest("GET", apiPath);
  });
  ipcMain.handle("api:post", async (event, apiPath, body) => {
    assertRenderer(event);
    return apiRequest("POST", apiPath, body);
  });
  ipcMain.handle("app:startup-info", (event) => {
    assertRenderer(event);
    const autoSetup = autoSetupPending;
    autoSetupPending = false;
    const postUpdate = postUpdatePending;
    postUpdatePending = false;
    return { version, autoSetup, postUpdate };
  });
  ipcMain.handle("app:install-update", async (event, download) => {
    assertRenderer(event);
    if (process.platform !== "win32") throw new Error("当前平台不能启动 Windows 更新安装器");
    const dataRoot = process.env.ZCODE_ANTIGRAVITY_DATA_DIR || path.join(process.env.LOCALAPPDATA || "", "ZCodeAntigravity");
    const updatesRoot = path.join(dataRoot, "updates");
    let installer = assertUpdateInstaller(download, updatesRoot);
    const info = fs.lstatSync(installer);
    if (!info.isFile() || info.isSymbolicLink() || info.size <= 0) throw new Error("更新安装器不是普通文件");
    const realRoot = fs.realpathSync(updatesRoot);
    installer = fs.realpathSync(installer);
    assertUpdateInstaller({ ...download, path: installer }, realRoot);
    const digest = await new Promise((resolve, reject) => {
      const hash = crypto.createHash("sha256");
      const stream = fs.createReadStream(installer);
      stream.on("error", reject);
      stream.on("data", (chunk) => hash.update(chunk));
      stream.on("end", () => resolve(hash.digest("hex")));
    });
    if (digest.toLowerCase() !== download.sha256.toLowerCase()) throw new Error("更新安装器 SHA-256 复核失败");
    const child = spawn(installer, ["--update"], { cwd: path.dirname(installer), detached: true, windowsHide: true, stdio: "ignore" });
    child.unref();
    setImmediate(() => app.quit());
    return { status: "started" };
  });
  ipcMain.handle("tray:update-summary", (event, summary) => {
    assertRenderer(event);
    tray?.setToolTip(trayTooltip(summary));
  });
  ipcMain.handle("window:show-main", (event) => {
    assertRenderer(event);
    showMainWindow();
  });
  ipcMain.handle("shell:open-xai", async (event, url) => {
    assertRenderer(event);
    await shell.openExternal(assertXaiURL(url));
  });
  ipcMain.handle("window:action", (event, action) => {
    assertRenderer(event);
    const window = BrowserWindow.fromWebContents(event.sender);
    if (!window) return;
    if (action === "minimize" && window === mainWindow) window.minimize();
    else if (action === "maximize" && window === mainWindow) window.isMaximized() ? window.unmaximize() : window.maximize();
    else if (action === "hide") window.hide();
    else throw new Error("不支持的窗口操作");
  });
  ipcMain.handle("clipboard:write", (event, value) => {
    assertRenderer(event);
    if (typeof value !== "string" || value.length > 1_000_000) throw new Error("剪贴板内容无效");
    clipboard.writeText(value);
  });
}

function secureWindow(window) {
  const rendererID = window.webContents.id;
  rendererIDs.add(rendererID);
  window.on("closed", () => rendererIDs.delete(rendererID));
  window.webContents.setWindowOpenHandler(() => ({ action: "deny" }));
  window.webContents.on("will-navigate", (event) => event.preventDefault());
  window.webContents.on("will-attach-webview", (event) => event.preventDefault());
  window.webContents.session.setPermissionRequestHandler((_contents, _permission, callback) => callback(false));
  window.webContents.session.setPermissionCheckHandler(() => false);
}

function commonWindowOptions() {
  return {
    frame: false,
    transparent: true,
    backgroundColor: "#00FFFFFF",
    backgroundMaterial: "acrylic",
    roundedCorners: true,
    hasShadow: true,
    show: false,
    icon: resourcePath("AppIcon.ico"),
    webPreferences: {
      preload: path.join(__dirname, "preload.cjs"),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
      webSecurity: true,
      devTools: !app.isPackaged,
      spellcheck: false,
      navigateOnDragDrop: false,
      backgroundThrottling: true,
    },
  };
}

function loadRenderer(window, query) {
  return window.loadFile(path.join(app.getAppPath(), "dist", "index.html"), query ? { query } : undefined);
}

function createMainWindow() {
  mainWindow = new BrowserWindow({
    ...commonWindowOptions(),
    title: "ZCode · Antigravity 控制中心 · Electron",
    width: 1180,
    height: 760,
    minWidth: 960,
    minHeight: 660,
    center: true,
    resizable: true,
    minimizable: true,
    maximizable: true,
    skipTaskbar: false,
  });
  secureWindow(mainWindow);
  mainWindow.removeMenu();
  mainWindow.on("close", (event) => {
    if (quitting) return;
    event.preventDefault();
    mainWindow.hide();
  });
  mainWindow.once("ready-to-show", () => mainWindow.show());
  void loadRenderer(mainWindow);
}

function createQuotaWidget() {
  quotaWidget = new BrowserWindow({
    ...commonWindowOptions(),
    title: "ZCode · 当前额度",
    width: WIDGET_WIDTH,
    height: WIDGET_HEIGHT,
    minWidth: WIDGET_WIDTH,
    minHeight: WIDGET_HEIGHT,
    maxWidth: WIDGET_WIDTH,
    maxHeight: WIDGET_HEIGHT,
    resizable: false,
    minimizable: false,
    maximizable: false,
    skipTaskbar: true,
    alwaysOnTop: true,
    focusable: true,
  });
  secureWindow(quotaWidget);
  quotaWidget.removeMenu();
  quotaWidget.setAlwaysOnTop(true, "pop-up-menu");
  quotaWidget.on("blur", () => {
    if (!quitting && !quotaWidget.webContents.isDevToolsOpened()) quotaWidget.hide();
  });
  quotaWidget.on("close", (event) => {
    if (quitting) return;
    event.preventDefault();
    quotaWidget.hide();
  });
  void loadRenderer(quotaWidget, { view: "widget" });
}

function showMainWindow() {
  if (!mainWindow || mainWindow.isDestroyed()) return;
  quotaWidget?.hide();
  if (mainWindow.isMinimized()) mainWindow.restore();
  mainWindow.show();
  mainWindow.focus();
}

function sendRefresh() {
  for (const window of [mainWindow, quotaWidget]) {
    if (window && !window.isDestroyed()) window.webContents.send("zcode:refresh");
  }
}

function toggleQuotaWidget() {
  if (!quotaWidget || quotaWidget.isDestroyed()) return;
  if (quotaWidget.isVisible()) {
    quotaWidget.hide();
    return;
  }
  const cursor = screen.getCursorScreenPoint();
  const { workArea } = screen.getDisplayNearestPoint(cursor);
  const left = workArea.x + 8;
  const right = workArea.x + workArea.width - WIDGET_WIDTH - 8;
  const top = workArea.y + 8;
  const bottom = workArea.y + workArea.height - WIDGET_HEIGHT - 8;
  const x = Math.max(left, Math.min(right, Math.round(cursor.x - WIDGET_WIDTH / 2)));
  const preferredY = cursor.y > workArea.y + workArea.height / 2 ? cursor.y - WIDGET_HEIGHT - 14 : cursor.y + 14;
  const y = Math.max(top, Math.min(bottom, Math.round(preferredY)));
  quotaWidget.setPosition(x, y, false);
  quotaWidget.webContents.send("zcode:refresh");
  quotaWidget.showInactive();
}

function createTray() {
  const icon = nativeImage.createFromPath(resourcePath("AppIcon.ico"));
  tray = new Tray(icon);
  tray.setToolTip("ZCode · 正在读取额度");
  tray.setContextMenu(Menu.buildFromTemplate([
    { label: "显示控制中心", click: showMainWindow },
    { label: "刷新额度", click: sendRefresh },
    { type: "separator" },
    { label: "退出", click: () => app.quit() },
  ]));
  tray.on("click", toggleQuotaWidget);
}

app.whenReady().then(async () => {
  try {
    app.setAppUserModelId("cn.zcode.antigravity");
    nativeTheme.themeSource = "light";
    await startNativeHost();
    registerIPC();
    createMainWindow();
    createQuotaWidget();
    createTray();
  } catch (error) {
    dialog.showErrorBox("ZCode Antigravity 启动失败", error instanceof Error ? error.message : String(error));
    app.quit();
  }
});

app.on("activate", showMainWindow);
app.on("window-all-closed", () => {});
app.on("before-quit", () => {
  quitting = true;
  stopNativeHost();
  tray?.destroy();
});
