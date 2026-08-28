export function hasDesktopBridge(): boolean {
  return typeof window.zcodeNative?.apiGet === "function";
}

export function desktopBridge(): ZCodeNativeBridge | undefined {
  return window.zcodeNative;
}
