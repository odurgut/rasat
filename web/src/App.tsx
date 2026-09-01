import { useEffect, useState, type ReactNode } from "react";
import { Chrome } from "./chrome/Chrome";
import { applyTheme, readTheme, type Theme } from "./theme";
import { type View } from "./views";
import { OverviewView } from "./search/OverviewView";
import { ServicesView } from "./search/ServicesView";
import { TracesView } from "./search/TracesView";
import { LogsView } from "./search/LogsView";
import { MapView } from "./search/MapView";
import { defaultForm, formForService, formFromSearchParams, pageTraceID, writePageURL, type SearchForm } from "./search/query";

export function App() {
  const [view, setView] = useState<View>(() => (pageTraceID() ? "traces" : "overview"));
  const [seen, setSeen] = useState<Set<View>>(() => new Set([pageTraceID() ? "traces" : "overview"]));
  const [theme, setTheme] = useState<Theme>(() => readTheme());
  const [logsTrace, setLogsTrace] = useState("");
  const [tracesEpoch, setTracesEpoch] = useState(0);
  const [serviceFocus, setServiceFocus] = useState("");
  const [serviceTick, setServiceTick] = useState(0);

  useEffect(() => {
    applyTheme(theme);
  }, [theme]);

  function clearTraceURL(): void {
    const form = formFromSearchParams(new URLSearchParams(window.location.search));
    writePageURL(form, "");
  }

  function go(next: View): void {
    setSeen((prev) => {
      if (prev.has(next)) {
        return prev;
      }
      const copy = new Set(prev);
      copy.add(next);
      return copy;
    });
    setView(next);
  }

  function openService(name: string): void {
    const form = formForService(name, new URLSearchParams(window.location.search));
    writePageURL(form, "");
    setServiceFocus(name);
    setServiceTick((n) => n + 1);
    go("services");
  }

  function openTracesForService(name: string): void {
    const form = formForService(name, new URLSearchParams(window.location.search));
    writePageURL(form, "");
    setTracesEpoch((n) => n + 1);
    go("traces");
  }

  function openTracesForm(form: SearchForm): void {
    writePageURL(form, "");
    setTracesEpoch((n) => n + 1);
    go("traces");
  }

  function openTrace(id: string): void {
    const current = formFromSearchParams(new URLSearchParams(window.location.search));
    writePageURL(
      { ...defaultForm(), start: current.start, end: current.end, limit: current.limit },
      id,
    );
    setTracesEpoch((n) => n + 1);
    go("traces");
  }

  return (
    <div className="shell">
      <Chrome
        view={view}
        theme={theme}
        onView={(v) => {
          if (v === "logs") {
            setLogsTrace("");
          }
          if (v !== "traces") {
            clearTraceURL();
          }
          go(v);
        }}
        onTheme={setTheme}
      />
      <div className="shell-main">
        {seen.has("overview") ? (
          <ShellView active={view === "overview"}>
            <OverviewView
              active={view === "overview"}
              onOpenService={openService}
              onOpenTraces={openTracesForm}
              onOpenTrace={openTrace}
            />
          </ShellView>
        ) : null}
        {seen.has("traces") ? (
          <ShellView active={view === "traces"}>
            <TracesView
              reloadToken={tracesEpoch}
              onOpenLogs={(id) => {
                setLogsTrace(id);
                go("logs");
              }}
              onOpenService={openService}
            />
          </ShellView>
        ) : null}
        {seen.has("services") ? (
          <ShellView active={view === "services"}>
            <ServicesView
              focus={serviceFocus}
              focusTick={serviceTick}
              onOpen={openTracesForService}
              onOpenTraces={openTracesForm}
            />
          </ShellView>
        ) : null}
        {seen.has("map") ? (
          <ShellView active={view === "map"}>
            <MapView onOpen={openTracesForService} onOpenService={openService} />
          </ShellView>
        ) : null}
        {seen.has("logs") ? (
          <ShellView active={view === "logs"}>
            <LogsView initialTraceID={logsTrace} onOpenTrace={openTrace} onOpenService={openService} />
          </ShellView>
        ) : null}
      </div>
    </div>
  );
}

function ShellView({ active, children }: { active: boolean; children: ReactNode }) {
  return (
    <div className={active ? "shell-view" : "shell-view is-idle"} aria-hidden={!active}>
      {children}
    </div>
  );
}
