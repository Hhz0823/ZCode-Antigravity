/// <reference types="vite/client" />

type DesktopWindowAction = "minimize" | "maximize" | "hide";

interface ZCodeNativeBridge {
  apiGet<T = unknown>(path: string): Promise<T>;
  apiPost<T = unknown>(path: string, body: Record<string, unknown>): Promise<T>;
  startupInfo(): Promise<{ version: string; autoSetup: boolean; postUpdate: boolean }>;
  installUpdate(download: { version: string; platform: string; assetName: string; path: string; sha256: string }): Promise<{ status: string }>;
  updateTraySummary(summary: {
    provider: "antigravity" | "xai";
    fiveHour?: number;
    week?: number;
    tokensPerSecond?: number;
  }): Promise<void>;
  showMainWindow(): Promise<void>;
  openXaiVerificationURL(url: string): Promise<void>;
  windowAction(action: DesktopWindowAction): Promise<void>;
  writeClipboard(value: string): Promise<void>;
  onRefresh(callback: () => void): void;
  removeRefreshListener(): void;
}

interface Window {
  zcodeNative?: ZCodeNativeBridge;
}
