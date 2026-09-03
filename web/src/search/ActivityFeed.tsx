import { useEffect, useRef, useState } from "react";
import { searchTraces, type TraceListRow } from "./api";
import { formatClock, formatDuration } from "./format";
import { serviceRampIndex } from "./color";
import { GhostButton } from "../chrome/GhostButton";
import { Trunc } from "./trunc";
import { activityKind, feedLimit } from "./activity";
import { flushPending, isListAtTop, listenTraceStream, prependLiveRow } from "./live";
import { useRowEnter } from "./rowEnter";

type ActivityFeedProps = {
  start: string;
  end: string;
  rangeKey: string;
  slowNs: number;
  active: boolean;
  onOpen: (traceID: string) => void;
};

export function ActivityFeed({ start, end, rangeKey, slowNs, active, onOpen }: ActivityFeedProps) {
  const [rows, setRows] = useState<TraceListRow[]>([]);
  const [pendingCount, setPendingCount] = useState(0);
  const [epoch, setEpoch] = useState(0);
  const slowRef = useRef(slowNs);
  const followRef = useRef(true);
  const hoverRef = useRef(false);
  const pendingRef = useRef<TraceListRow[]>([]);
  const rowsRef = useRef<TraceListRow[]>([]);
  const scrollRef = useRef<HTMLDivElement>(null);
  slowRef.current = slowNs;
  rowsRef.current = rows;

  function syncFollow(): void {
    followRef.current =
      isListAtTop(scrollRef.current) && !hoverRef.current && pendingRef.current.length === 0;
  }

  function applyPending(): void {
    const pending = pendingRef.current;
    if (pending.length === 0) {
      return;
    }
    pendingRef.current = [];
    setPendingCount(0);
    setRows((prev) => flushPending(prev, pending, feedLimit()));
    followRef.current = isListAtTop(scrollRef.current) && !hoverRef.current;
  }

  function ingestLive(row: TraceListRow): void {
    const limit = feedLimit();
    if (followRef.current) {
      setRows((prev) => prependLiveRow(prev, row, limit));
      return;
    }
    pendingRef.current = prependLiveRow(pendingRef.current, row, limit);
    setPendingCount(pendingRef.current.length);
  }

  const ingestLiveRef = useRef(ingestLive);
  ingestLiveRef.current = ingestLive;

  useEffect(() => {
    let closed = false;
    const ac = new AbortController();
    let stopStream = (): void => undefined;

    void (async () => {
      try {
        const found = await searchTraces(
          {
            service: "",
            op: "",
            min: "",
            status: "",
            start,
            end,
            limit: String(feedLimit()),
          },
          ac.signal,
        );
        if (closed) {
          return;
        }
        pendingRef.current = [];
        setPendingCount(0);
        setRows(found);
        setEpoch((n) => n + 1);
        followRef.current = true;
        hoverRef.current = false;
      } catch (e) {
        if (closed || (e instanceof DOMException && e.name === "AbortError")) {
          return;
        }
      }
      if (!closed) {
        stopStream = listenTraceStream((row) => ingestLiveRef.current(row));
      }
    })();

    return () => {
      closed = true;
      ac.abort();
      stopStream();
    };
  }, [rangeKey]);

  useEffect(() => {
    if (!active) {
      return;
    }
    let closed = false;
    const pull = async () => {
      if (closed || rowsRef.current.length > 0) {
        return;
      }
      try {
        const found = await searchTraces({
          service: "",
          op: "",
          min: "",
          status: "",
          start,
          end,
          limit: String(feedLimit()),
        });
        if (closed || found.length === 0 || rowsRef.current.length > 0) {
          return;
        }
        pendingRef.current = [];
        setPendingCount(0);
        setRows(found);
        setEpoch((n) => n + 1);
      } catch {
        return;
      }
    };
    void pull();
    const id = window.setInterval(() => void pull(), 5_000);
    return () => {
      closed = true;
      window.clearInterval(id);
    };
  }, [active, rangeKey, start, end]);

  const enter = useRowEnter(
    rows.map((r) => r.trace_id),
    epoch,
  );

  return (
    <section className="card card-activity">
      <div className={pendingCount > 0 ? "card-activity-head is-live" : "card-activity-head"}>
        <span>activity</span>
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
      <div
        className="card-activity-list"
        ref={scrollRef}
        onScroll={syncFollow}
        onPointerEnter={() => {
          if (rowsRef.current.length === 0) {
            return;
          }
          hoverRef.current = true;
          followRef.current = false;
        }}
        onPointerLeave={() => {
          hoverRef.current = false;
          syncFollow();
        }}
      >
        {rows.length === 0 ? (
          <p className="empty">no traces yet</p>
        ) : (
          rows.map((row) => {
            const kind = activityKind(row, slowRef.current);
            const cls = ["insight-row"];
            if (kind === "err") {
              cls.push("is-err");
            }
            return (
              <div key={row.trace_id} className={enter.has(row.trace_id) ? "row-enter is-enter" : "row-enter"}>
                <button type="button" className={cls.join(" ")} onClick={() => onOpen(row.trace_id)}>
                <span className="feed-clock">{formatClock(row.timestamp)}</span>
                <span className={kindClass(kind)}>{kindLabel(kind)}</span>
                <span className="insight-svc">
                  <span className={`svc-swatch svc-${serviceRampIndex(row.service)}`} aria-hidden="true" />
                  <Trunc text={row.service || "(unknown)"} />
                </span>
                <span className="insight-op">
                  <Trunc text={row.operation || "(no op)"} />
                </span>
                <span className="insight-dur">{formatDuration(row.duration_ns)}</span>
              </button>
              </div>
            );
          })
        )}
      </div>
    </section>
  );
}

function kindClass(kind: ReturnType<typeof activityKind>): string {
  if (kind === "err") {
    return "feed-kind tok-err";
  }
  if (kind === "slow") {
    return "feed-kind tok-warn";
  }
  return "feed-kind tok-info";
}

function kindLabel(kind: ReturnType<typeof activityKind>): string {
  if (kind === "err") {
    return "ERR";
  }
  if (kind === "slow") {
    return "slow";
  }
  return "in";
}
