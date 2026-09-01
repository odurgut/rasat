import type { TraceListRow } from "./api";

export type ActivityKind = "in" | "slow" | "err";

const FEED_LIMIT = 50;

export function feedLimit(): number {
  return FEED_LIMIT;
}

/** Error first; else slow when duration meets the window p95; else incoming. */
export function activityKind(row: TraceListRow, slowNs: number): ActivityKind {
  if (row.status_code === 2) {
    return "err";
  }
  if (slowNs > 0 && row.duration_ns >= slowNs) {
    return "slow";
  }
  return "in";
}
