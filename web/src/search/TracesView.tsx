import { useEffect, useRef, useState } from "react";
import { SearchError, getTrace, searchTraces, type SpanDetail, type TraceDetail, type TraceListRow } from "./api";
import { SearchFormFields } from "./SearchForm";
import { SpanInspector, InsightDock } from "./SpanInspector";
import { TraceList } from "./TraceList";
import { Waterfall, TraceBack } from "./Waterfall";
import { formFromSearchParams, pageTraceID, writePageURL, defaultForm, type SearchForm } from "./query";
import { isLiveTail, isListAtTop, flushPending, parseStreamRow, prependLiveRow, rowMatchesForm, tracesStreamURL } from "./live";
import { GhostButton } from "../chrome/GhostButton";
import { Workspace } from "../chrome/Workspace";
import { useRowEnter } from "./rowEnter";

type Status = "idle" | "searching" | "ok" | "error";

function readInitialForm(): SearchForm {
  return formFromSearchParams(new URLSearchParams(window.location.search));
}

export function TracesView({
  reloadToken = 0,
  onOpenLogs,
  onOpenService,
}: {
  reloadToken?: number;
  onOpenLogs?: (traceID: string) => void;
  onOpenService?: (service: string) => void;
}) {
  const [form, setForm] = useState<SearchForm>(readInitialForm);
  const [rows, setRows] = useState<TraceListRow[]>([]);
  const [status, setStatus] = useState<Status>("idle");
  const [error, setError] = useState<string>("");
  const [traceID, setTraceID] = useState<string>(() => pageTraceID());
  const [detail, setDetail] = useState<TraceDetail | null>(null);
  const [selectedID, setSelectedID] = useState<string>("");
  const [traceErr, setTraceErr] = useState<string>("");
  const [traceLoading, setTraceLoading] = useState(false);
  const seq = useRef(0);
  const traceSeq = useRef(0);
  const formRef = useRef(form);
  const statusRef = useRef(status);
  const liveTailRef = useRef(true);
  const scrollRef = useRef<HTMLDivElement>(null);
  const hoverRef = useRef(false);
  const followRef = useRef(true);
  const pendingRef = useRef<TraceListRow[]>([]);
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
    setRows((prev) => flushPending(prev, pending, limit));
    followRef.current = isListAtTop(scrollRef.current) && !hoverRef.current;
  }

  function syncFollow(): void {
    followRef.current =
      isListAtTop(scrollRef.current) && !hoverRef.current && pendingRef.current.length === 0;
  }

  function ingestLive(row: TraceListRow): void {
    const limit = listLimit();
    if (followRef.current) {
      setRows((prev) => prependLiveRow(prev, row, limit));
      return;
    }
    pendingRef.current = prependLiveRow(pendingRef.current, row, limit);
    setPendingCount(pendingRef.current.length);
  }

  const ingestLiveRef = useRef(ingestLive);
  ingestLiveRef.current = ingestLive;

  async function runSearch(next: SearchForm, signal?: AbortSignal): Promise<void> {
    const n = ++seq.current;
    setStatus("searching");
    setError("");
    writePageURL(next, traceID);
    try {
      const found = await searchTraces(next, signal);
      if (n !== seq.current) {
        return;
      }
      setRows(found);
      setStatus("ok");
      liveTailRef.current = isLiveTail(next.end);
      clearPending();
      setEpoch((n) => n + 1);
    } catch (e) {
      if (n !== seq.current) {
        return;
      }
      if (e instanceof DOMException && e.name === "AbortError") {
        return;
      }
      const msg = e instanceof SearchError || e instanceof Error ? e.message : "search failed";
      setRows([]);
      setError(msg);
      setStatus("error");
      setEpoch((n) => n + 1);
    }
  }

  async function openTrace(id: string, nextForm: SearchForm, signal?: AbortSignal, row?: TraceListRow): Promise<void> {
    const n = ++traceSeq.current;
    setTraceID(id);
    setTraceLoading(true);
    setTraceErr("");
    setDetail(null);
    writePageURL(nextForm, id);
    try {
      const got = await getTrace(id, nextForm, signal, row);
      if (n !== traceSeq.current) {
        return;
      }
      setDetail(got);
      setSelectedID("");
      setTraceLoading(false);
    } catch (e) {
      if (n !== traceSeq.current) {
        return;
      }
      if (e instanceof DOMException && e.name === "AbortError") {
        return;
      }
      const msg = e instanceof SearchError || e instanceof Error ? e.message : "load failed";
      setDetail(null);
      setSelectedID("");
      setTraceErr(msg);
      setTraceLoading(false);
    }
  }

  useEffect(() => {
    const ac = new AbortController();
    const next = readInitialForm();
    setForm(next);
    const id = pageTraceID();
    if (id) {
      void openTrace(id, next, ac.signal);
    } else {
      setTraceID("");
      setDetail(null);
      setSelectedID("");
      setTraceErr("");
      setTraceLoading(false);
      void runSearch(next, ac.signal);
    }
    return () => ac.abort();
  }, [reloadToken]);

  useEffect(() => {
    let closed = false;
    let ws: WebSocket | null = null;
    let retry: number | undefined;

    const connect = () => {
      if (closed) {
        return;
      }
      ws = new WebSocket(tracesStreamURL());
      ws.onmessage = (ev) => {
        if (typeof ev.data !== "string") {
          return;
        }
        const row = parseStreamRow(ev.data);
        if (!row) {
          return;
        }
        if (statusRef.current === "error") {
          return;
        }
        const current = formRef.current;
        if (!rowMatchesForm(row, current, liveTailRef.current)) {
          return;
        }
        ingestLiveRef.current(row);
      };
      ws.onclose = () => {
        if (closed) {
          return;
        }
        retry = window.setTimeout(connect, 2000);
      };
    };
    connect();
    return () => {
      closed = true;
      if (retry !== undefined) {
        window.clearTimeout(retry);
      }
      ws?.close();
    };
  }, []);

  const selected: SpanDetail | undefined = detail?.spans.find((s) => s.span_id === selectedID);
  const onTracePage = Boolean(traceID);

  function closeTrace(): void {
    traceSeq.current += 1;
    setTraceID("");
    setDetail(null);
    setSelectedID("");
    setTraceErr("");
    setTraceLoading(false);
    writePageURL(form, "");
  }

  const enter = useRowEnter(
    rows.map((r) => r.trace_id),
    epoch,
  );

  if (onTracePage) {
    return (
      <Workspace
        work={
          <>
            <div className="pane-work-scroll">
              {traceLoading ? (
                <header className="wf-head">
                  <p className="wf-head-title">
                    <TraceBack onClick={closeTrace} />
                    <span className="muted">loading</span>
                  </p>
                </header>
              ) : traceErr ? (
                <header className="wf-head">
                  <p className="wf-head-title">
                    <TraceBack onClick={closeTrace} />
                    <span className="surface-error-word">ERROR</span>
                    <span className="muted">{traceErr}</span>
                  </p>
                </header>
              ) : detail ? (
                <Waterfall
                  detail={detail}
                  selectedID={selectedID}
                  onSelect={setSelectedID}
                  onOpenService={onOpenService}
                  onBack={closeTrace}
                />
              ) : (
                <header className="wf-head">
                  <p className="wf-head-title">
                    <TraceBack onClick={closeTrace} />
                    <span className="muted">select a trace</span>
                  </p>
                </header>
              )}
            </div>
            {detail && !traceLoading && !traceErr ? (
              <InsightDock
                key={detail.trace_id}
                path={detail.critical_path ?? []}
                pathNs={detail.critical_path_ns ?? 0}
                totalNs={detail.duration_ns}
                bottlenecks={detail.bottlenecks ?? []}
                selectedID={selectedID}
                onSelect={setSelectedID}
              />
            ) : null}
          </>
        }
        detail={
          selected && detail ? (
            <div className="pane-detail-scroll">
              <SpanInspector
                span={selected}
                traceID={detail.trace_id}
                form={form}
                onOpenLogs={onOpenLogs}
                onOpenService={onOpenService}
                onOpenTrace={(id) => void openTrace(id, form)}
              />
            </div>
          ) : undefined
        }
      />
    );
  }

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
            <SearchFormFields
              form={form}
              disabled={status === "searching"}
              onChange={setForm}
              onSubmit={() => void runSearch(form)}
              onReset={() => {
                const next = defaultForm();
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
            <TraceList
              rows={rows}
              selectedID={traceID}
              enterIDs={enter}
              onOpen={(row) => void openTrace(row.trace_id, form, undefined, row)}
            />
          )}
        </div>
      }
    />
  );
}
