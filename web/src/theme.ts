const STORAGE_KEY = "rasat-theme-choice";
const LEGACY_KEY = "rasat-theme";

export type Theme = "light" | "dark";

export function readTheme(): Theme {
  return storedTheme() ?? systemTheme();
}

export function storedTheme(): Theme | null {
  try {
    localStorage.removeItem(LEGACY_KEY);
    const v = localStorage.getItem(STORAGE_KEY);
    if (v === "dark" || v === "light") {
      return v;
    }
  } catch {
    /* private mode */
  }
  return null;
}

export function rememberTheme(theme: Theme): void {
  try {
    localStorage.setItem(STORAGE_KEY, theme);
  } catch {
    /* private mode */
  }
}

export function systemTheme(): Theme {
  try {
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  } catch {
    return "light";
  }
}

export function applyTheme(theme: Theme): void {
  const root = document.documentElement;
  root.classList.add("theme-odurgut");
  root.classList.toggle("dark", theme === "dark");
  const meta = document.querySelector('meta[name="theme-color"]');
  if (meta) {
    meta.setAttribute("content", theme === "dark" ? "#0a0a0a" : "#ffffff");
  }
}

export function watchSystemTheme(onChange: (theme: Theme) => void): () => void {
  let mq: MediaQueryList;
  try {
    mq = window.matchMedia("(prefers-color-scheme: dark)");
  } catch {
    return () => undefined;
  }
  function sync(): void {
    if (storedTheme()) {
      return;
    }
    onChange(systemTheme());
  }
  mq.addEventListener("change", sync);
  return () => mq.removeEventListener("change", sync);
}
