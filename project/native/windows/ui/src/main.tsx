import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import "./index.css";
import { hasDesktopBridge } from "./lib/native-api";

if (hasDesktopBridge()) document.documentElement.classList.add("electron-shell");

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
