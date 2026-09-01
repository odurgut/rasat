import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import { applyTheme, readTheme } from "./theme";
import { startFavicon } from "./favicon";
import "./fonts/fonts.css";
import "./styles/tokens.css";
import "./styles/base.css";

applyTheme(readTheme());
startFavicon();

const el = document.getElementById("root");
if (!el) {
  throw new Error("root element missing");
}

createRoot(el).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
