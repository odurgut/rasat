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

  const t0 = Date.parse(detail.timestamp);
  const tDur = Math.max(detail.duration_ns, 1);
  const out: LaidSpan[] = [];
  const seen = new Set<string>();

  const walk = (s: SpanDetail, depth: number, isLast: boolean, ancestorLast: boolean[]): void => {
    if (seen.has(s.span_id)) {
      return;
    }
    seen.add(s.span_id);
    const kids = children.get(s.span_id) ?? [];
    out.push(layoutOne(s, depth, t0, tDur, kids, isLast, ancestorLast));
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
  t0: number,
  tDur: number,
  kids: SpanDetail[],
  isLast: boolean,
  ancestorLast: boolean[],
): LaidSpan {
  const start = Date.parse(s.timestamp);
  let offsetPct = 0;
  if (!Number.isNaN(t0) && !Number.isNaN(start)) {
    offsetPct = (((start - t0) * 1_000_000) / tDur) * 100;
  }
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
  const ta = Date.parse(a.timestamp);
  const tb = Date.parse(b.timestamp);
  if (ta !== tb) {
    return ta - tb;
  }
  return a.span_id.localeCompare(b.span_id);
}

function clamp(n: number, lo: number, hi: number): number {
  return Math.min(hi, Math.max(lo, n));
}
