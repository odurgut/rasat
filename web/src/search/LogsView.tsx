import { useEffect, useRef, useState } from "react";
import { SearchError, searchLogs, type LogRow } from "./api";
import { LogFilterFields } from "./LogFilter";
import { LogList, levelClass, logRowKey } from "./LogList";
import { defaultLogForm, type LogForm } from "./query";
import { formatTimestamp } from "./format";
import { isLiveTail, isListAtTop, flushPendingLogs, listenLogStream, prependLiveLog, logMatchesForm } from "./live";
import { GhostButton } from "../chrome/GhostButton";
import { PrimaryButton } from "../chrome/PrimaryButton";
import { Workspace } from "../chrome/Workspace";
import { Jump } from "../chrome/Jump";
import { Trunc } from "./trunc";
import { serviceRampIndex } from "./color";
import { useRowEnter } from "./rowEnter";

type Status = "idle" | "searching" | "ok" | "error";

type LogsViewProps = {
  active?: boolean;
  initialTraceID?: string;
  onOpenTrace: (traceID: string) => void;
  onOpenService?: (service: string) => void;
};

export function LogsView({ active = true, initialTraceID = "", onOpenTrace, onOpenService }: LogsViewProps) {
  const [form, setForm] = useState<LogForm>(() => defaultLogForm(new Date(), initialTraceID));
  const [rows, setRows] = useState<LogRow[]>([]);
  const [status, setStatus] = useState<Status>("idle");
  const [error, setError] = useState("");
  const [selected, setSelected] = useState<LogRow | null>(null);
  const seq = useRef(0);
  const prevTrace = useRef(initialTraceID);
  const formRef = useRef(form);
  const statusRef = useRef(status);
  const liveTailRef = useRef(true);
  const scrollRef = useRef<HTMLDivElement>(null);
  const hoverRef = useRef(false);
  const followRef = useRef(true);
  const pendingRef = useRef<LogRow[]>([]);
  const [pendingCount, setPendingCount] = useState(0);
  const [epoch, setEpoch] = useState(0);
  formRef.current = form;
  statusRef.current = status;

  function listLimit(): number {
    return Number.parseInt(formRef.current.limit, 10) || 50;
  }

  function clearPending(): void {
    pendingRef.current = [];
    setPendingCount(0);
  }

  function applyPending(): void {
    const pending = pendingRef.current;
    if (pending.length === 0) {
      return;
    }
    pendingRef.current = [];
    setPendingCount(0);
    const limit = listLimit();
    setRows((prev) => flushPendingLogs(prev, pending, limit));
    followRef.current = isListAtTop(scrollRef.current) && !hoverRef.current;
  }

  function syncFollow(): void {
    followRef.current =
      isListAtTop(scrollRef.current) && !hoverRef.current && pendingRef.current.length === 0;
  }

  function ingestLive(row: LogRow): void {
    const limit = listLimit();
    if (followRef.current) {
      setRows((prev) => prependLiveLog(prev, row, limit));
      return;
    }
    pendingRef.current = prependLiveLog(pendingRef.current, row, limit);
    setPendingCount(pendingRef.current.length);
  }

  const ingestLiveRef = useRef(ingestLive);
  ingestLiveRef.current = ingestLive;

  async function runSearch(next: LogForm, signal?: AbortSignal): Promise<void> {
    const n = ++seq.current;
    setStatus("searching");
    setError("");
    try {
      const found = await searchLogs(next, signal);
      if (n !== seq.current) {
        return;
      }
      setRows(found);
      setStatus("ok");
      liveTailRef.current = isLiveTail(next.end);
      clearPending();
      setEpoch((n) => n + 1);
      setSelected((cur) => {
        if (!cur) {
          return found[0] ?? null;
        }
        return found.find((r) => r.timestamp === cur.timestamp && r.message === cur.message && r.trace_id === cur.trace_id) ?? found[0] ?? null;
      });
    } catch (e) {
      if (n !== seq.current) {
        return;
      }
      if (e instanceof DOMException && e.name === "AbortError") {
        return;
      }
      const msg = e instanceof SearchError || e instanceof Error ? e.message : "search failed";
      setRows([]);
      setSelected(null);
      setError(msg);
      setStatus("error");
      setEpoch((n) => n + 1);
    }
  }

  useEffect(() => {
    if (!active) {
      return;
    }
    const traceChanged = prevTrace.current !== initialTraceID;
    prevTrace.current = initialTraceID;
    if (!traceChanged && statusRef.current === "ok") {
      return;
    }
    const ac = new AbortController();
    void runSearch(defaultLogForm(new Date(), initialTraceID), ac.signal);
    return () => ac.abort();
  }, [active, initialTraceID]);

  useEffect(() => {
    return listenLogStream((row) => {
      if (statusRef.current === "error") {
        return;
      }
      if (!logMatchesForm(row, formRef.current, liveTailRef.current)) {
        return;
      }
      ingestLiveRef.current(row);
    });
  }, []);

  const enter = useRowEnter(
    rows.map((r) => logRowKey(r)),
    epoch,
  );
  const level = selected ? (selected.level || "INFO").toUpperCase() : "";

  return (
    <Workspace
      list={
        <>
          <div className={pendingCount > 0 ? "pane-list-head is-live" : "pane-list-head"}>
            <span className="pane-list-count">{status === "ok" || rows.length > 0 ? `${rows.length}` : ""}</span>
            <div className="pane-list-tools">
              {pendingCount > 0 ? (
                <GhostButton
                  className="is-live"
                  label={`${pendingCount} new`}
                  onClick={() => {
                    hoverRef.current = false;
                    applyPending();
                    const el = scrollRef.current;
                    if (el) {
                      el.scrollTop = 0;
                    }
                    followRef.current = true;
                  }}
                />
              ) : null}
            </div>
          </div>
          <section className="pane-filter is-fill">
            <LogFilterFields
              form={form}
              disabled={status === "searching"}
              onChange={setForm}
              onSubmit={() => void runSearch(form)}
              onReset={() => {
                const next = defaultLogForm();
                setForm(next);
                void runSearch(next);
              }}
            />
          </section>
        </>
      }
      work={
        <div
          className="pane-work-scroll"
          ref={scrollRef}
          onScroll={syncFollow}
          onPointerEnter={() => {
            hoverRef.current = true;
            followRef.current = false;
          }}
          onPointerLeave={() => {
            hoverRef.current = false;
            syncFollow();
          }}
        >
          {status === "error" ? (
            <p className="surface-error">
              <span className="surface-error-word">ERROR</span> {error}
            </p>
          ) : status === "searching" && rows.length === 0 ? (
            <p className="empty">searching</p>
          ) : (
            <LogList rows={rows} selected={selected} enterKeys={enter} onOpen={setSelected} />
          )}
        </div>
      }
      detail={
        selected ? (
          <div className="pane-detail-scroll">
              <div className="kv-table">
                <div className="kv-row">
                  <span className="kv-key">message</span>
                  <span className="kv-val">{selected.message || "(empty)"}</span>
                </div>
                <div className="kv-row">
                  <span className="kv-key">level</span>
                  <span className={`kv-val ${levelClass(level)}`}>{level}</span>
                </div>
                <div className="kv-row">
                  <span className="kv-key">service</span>
                  <span className="kv-val">
                    <span className={`svc-swatch svc-${serviceRampIndex(selected.service || "")}`} aria-hidden="true" />
                    {onOpenService && selected.service ? (
                      <Jump onClick={() => onOpenService(selected.service)}>
                        <Trunc text={selected.service} />
                      </Jump>
                    ) : (
                      <Trunc text={selected.service || "(unknown)"} />
                    )}
                  </span>
                </div>
                <div className="kv-row">
                  <span className="kv-key">time</span>
                  <span className="kv-val">
                    <Trunc text={formatTimestamp(selected.timestamp)} />
                  </span>
                </div>
                <div className="kv-row">
                  <span className="kv-key">trace</span>
                  <span className={selected.trace_id ? "kv-val" : "kv-val is-muted"}>
                    {selected.trace_id ? (
                      <Jump onClick={() => onOpenTrace(selected.trace_id)}>
                        <Trunc text={selected.trace_id} />
                      </Jump>
                    ) : (
                      <Trunc text="—" />
                    )}
                  </span>
                </div>
                {selected.span_id ? (
                  <div className="kv-row">
                    <span className="kv-key">span</span>
                    <span className="kv-val">
                      <Trunc text={selected.span_id} />
                    </span>
                  </div>
                ) : null}
              </div>
              {selected.trace_id ? (
                <p className="search-actions log-jump">
                  <PrimaryButton label="open trace" onClick={() => onOpenTrace(selected.trace_id)} />
                </p>
              ) : null}
          </div>
        ) : undefined
      }
    />
  );
}
