"use strict";

const { contextBridge, ipcRenderer } = require("electron");

let refreshListener;

contextBridge.exposeInMainWorld("zcodeNative", Object.freeze({
  apiGet: (path) => ipcRenderer.invoke("api:get", path),
  apiPost: (path, body) => ipcRenderer.invoke("api:post", path, body),
  startupInfo: () => ipcRenderer.invoke("app:startup-info"),
  updateTraySummary: (summary) => ipcRenderer.invoke("tray:update-summary", summary),
  showMainWindow: () => ipcRenderer.invoke("window:show-main"),
  openXaiVerificationURL: (url) => ipcRenderer.invoke("shell:open-xai", url),
  windowAction: (action) => ipcRenderer.invoke("window:action", action),
  writeClipboard: (value) => ipcRenderer.invoke("clipboard:write", value),
  onRefresh: (callback) => {
    if (typeof callback !== "function") return;
    if (refreshListener) ipcRenderer.removeListener("zcode:refresh", refreshListener);
    refreshListener = () => callback();
    ipcRenderer.on("zcode:refresh", refreshListener);
  },
  removeRefreshListener: () => {
    if (!refreshListener) return;
    ipcRenderer.removeListener("zcode:refresh", refreshListener);
    refreshListener = undefined;
  },
}));
