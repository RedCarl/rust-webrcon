import React from "react";
import ReactDOM from "react-dom/client";
import { HashRouter } from "react-router-dom";
import App from "@/App";
import { Toaster } from "@/components/ui/sonner";
import "@/index.css";

const THEME_KEY = "rustpanel-theme";

function applyTheme(next: "light" | "dark") {
  const root = document.documentElement;
  if (next === "dark") root.classList.add("dark");
  else root.classList.remove("dark");
  try {
    localStorage.setItem(THEME_KEY, next);
  } catch {
    return;
  }
}

try {
  const saved = localStorage.getItem(THEME_KEY);
  if (saved === "dark" || saved === "light") applyTheme(saved);
  else applyTheme("light");
} catch {
  applyTheme("light");
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <HashRouter>
      <App />
      <Toaster position="top-right" richColors />
    </HashRouter>
  </React.StrictMode>,
);
