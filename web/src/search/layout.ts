import type { SpanDetail, TraceDetail } from "./api";

export type LaidSpan = {
  span: SpanDetail;
  depth: number;
  offsetPct: number;
  widthPct: number;
  selfRatio: number;
  hasChildren: boolean;
  isLast: boolean;
  ancestorLast: boolean[];
};

export function layoutWaterfall(detail: TraceDetail): LaidSpan[] {
  const spans = detail.spans;
  const byId = new Map<string, SpanDetail>();
  for (const s of spans) {
    byId.set(s.span_id, s);
  }

  const children = new Map<string, SpanDetail[]>();
  const roots: SpanDetail[] = [];
  for (const s of spans) {
    const p = s.parent_span_id;
    if (!p || !byId.has(p)) {
      roots.push(s);
      continue;
    }
    const list = children.get(p) ?? [];
    list.push(s);
    children.set(p, list);
  }
  for (const list of children.values()) {
    list.sort(compareSpans);
  }

  const tDur = Math.max(detail.duration_ns, 1);
  const originNs = parseTimeNs(detail.timestamp);
  const out: LaidSpan[] = [];
  const seen = new Set<string>();

  const walk = (s: SpanDetail, depth: number, isLast: boolean, ancestorLast: boolean[]): void => {
    if (seen.has(s.span_id)) {
      return;
    }
    seen.add(s.span_id);
    const kids = children.get(s.span_id) ?? [];
    out.push(layoutOne(s, depth, originNs, tDur, kids, isLast, ancestorLast));
    const nextAnc = [...ancestorLast, isLast];
    for (let i = 0; i < kids.length; i++) {
      const c = kids[i];
      if (!c) {
        continue;
      }
      walk(c, depth + 1, i === kids.length - 1, nextAnc);
    }
  };

  for (let i = 0; i < roots.length; i++) {
    const r = roots[i];
    if (!r) {
      continue;
    }
    walk(r, 0, i === roots.length - 1, []);
  }
  const extra: SpanDetail[] = [];
  for (const s of spans) {
    if (!seen.has(s.span_id)) {
      extra.push(s);
    }
  }
  for (let i = 0; i < extra.length; i++) {
    const s = extra[i];
    if (!s) {
      continue;
    }
    walk(s, 0, i === extra.length - 1, []);
  }
  return out;
}

function layoutOne(
  s: SpanDetail,
  depth: number,
  originNs: number | null,
  tDur: number,
  kids: SpanDetail[],
  isLast: boolean,
  ancestorLast: boolean[],
): LaidSpan {
  const offNs = spanOffsetNs(s, originNs);
  let offsetPct = (offNs / tDur) * 100;
  let widthPct = (s.duration_ns / tDur) * 100;
  offsetPct = clamp(offsetPct, 0, 99.6);
  widthPct = clamp(widthPct, 0.4, 100 - offsetPct);

  let childNs = 0;
  for (const c of kids) {
    childNs += c.duration_ns;
  }
  const selfRatio = s.duration_ns <= 0 ? 1 : clamp(1 - childNs / s.duration_ns, 0, 1);
  return {
    span: s,
    depth,
    offsetPct,
    widthPct,
    selfRatio,
    hasChildren: kids.length > 0,
    isLast,
    ancestorLast,
  };
}

function compareSpans(a: SpanDetail, b: SpanDetail): number {
  const off = spanOffsetNs(a, null) - spanOffsetNs(b, null);
  if (off !== 0) {
    return off;
  }
  const ta = parseTimeNs(a.timestamp) ?? 0;
  const tb = parseTimeNs(b.timestamp) ?? 0;
  if (ta !== tb) {
    return ta - tb;
  }
  return a.span_id.localeCompare(b.span_id);
}

/** Epoch nanoseconds. Fractional RFC3339 is not left to Date.parse (ms only). */
export function parseTimeNs(iso: string): number | null {
  const s = iso.trim();
  const m = s.match(/^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d+))?(Z|[+-]\d{2}:?\d{2})$/);
  if (!m || m[1] === undefined || m[3] === undefined) {
    const ms = Date.parse(s);
    return Number.isFinite(ms) ? ms * 1_000_000 : null;
  }
  const ms = Date.parse(`${m[1]}${m[3]}`);
  if (!Number.isFinite(ms)) {
    return null;
  }
  const frac = (m[2] ?? "").padEnd(9, "0").slice(0, 9);
  const nano = Number.parseInt(frac, 10);
  return ms * 1_000_000 + (Number.isFinite(nano) ? nano : 0);
}

function spanOffsetNs(s: SpanDetail, originNs: number | null): number {
  if (typeof s.start_offset_ns === "number" && Number.isFinite(s.start_offset_ns) && s.start_offset_ns >= 0) {
    return s.start_offset_ns;
  }
  const start = parseTimeNs(s.timestamp);
  if (start === null || originNs === null) {
    return 0;
  }
  return Math.max(0, start - originNs);
}

function clamp(n: number, lo: number, hi: number): number {
  return Math.min(hi, Math.max(lo, n));
}
