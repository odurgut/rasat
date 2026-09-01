export type ServiceMapNode = {
  service: string;
  spans: number;
  errors: number;
};

export type ServiceMapEdge = {
  from: string;
  to: string;
  calls: number;
  errors: number;
  avg_duration_ns: number;
};

export type LaidMapNode = ServiceMapNode & {
  x: number;
  y: number;
  w: number;
  h: number;
};

export type LaidMapEdge = ServiceMapEdge & {
  path: string;
  tone: "ok" | "warn" | "err";
  labelX: number;
  labelY: number;
};

export type LaidMap = {
  nodes: LaidMapNode[];
  edges: LaidMapEdge[];
  width: number;
  height: number;
};

export type MapViewport = {
  width: number;
  height: number;
};

const NODE_W = 168;
const NODE_H = 64;
const GAP_X_MIN = 80;
const GAP_Y_MIN = 28;
const GAP_X_MAX = 200;
const GAP_Y_MAX = 96;
const PAD_MIN = 36;

export function layoutServiceMap(nodes: ServiceMapNode[], edges: ServiceMapEdge[], viewport?: MapViewport): LaidMap {
  if (nodes.length === 0) {
    return { nodes: [], edges: [], width: 360, height: 180 };
  }

  const names = new Set(nodes.map((n) => n.service));
  const outs = new Map<string, string[]>();
  const indeg = new Map<string, number>();
  for (const n of nodes) {
    outs.set(n.service, []);
    indeg.set(n.service, 0);
  }
  for (const e of edges) {
    if (!names.has(e.from) || !names.has(e.to) || e.from === e.to) {
      continue;
    }
    const list = outs.get(e.from) ?? [];
    if (!list.includes(e.to)) {
      list.push(e.to);
      outs.set(e.from, list);
      indeg.set(e.to, (indeg.get(e.to) ?? 0) + 1);
    }
  }

  const layerOf = new Map<string, number>();
  let frontier = nodes.filter((n) => (indeg.get(n.service) ?? 0) === 0).map((n) => n.service);
  if (frontier.length === 0) {
    frontier = nodes.map((n) => n.service);
  }
  let depth = 0;
  while (frontier.length > 0) {
    const next: string[] = [];
    for (const s of frontier) {
      if (layerOf.has(s)) {
        continue;
      }
      layerOf.set(s, depth);
      for (const t of outs.get(s) ?? []) {
        if (!layerOf.has(t)) {
          next.push(t);
        }
      }
    }
    frontier = next;
    depth += 1;
    if (depth > nodes.length + 1) {
      break;
    }
  }
  for (const n of nodes) {
    if (!layerOf.has(n.service)) {
      layerOf.set(n.service, depth);
    }
  }

  const columns = new Map<number, ServiceMapNode[]>();
  let maxLayer = 0;
  for (const n of nodes) {
    const layer = layerOf.get(n.service) ?? 0;
    maxLayer = Math.max(maxLayer, layer);
    const col = columns.get(layer) ?? [];
    col.push(n);
    columns.set(layer, col);
  }
  for (const col of columns.values()) {
    col.sort((a, b) => b.spans - a.spans || a.service.localeCompare(b.service));
  }

  let maxRows = 1;
  for (const col of columns.values()) {
    maxRows = Math.max(maxRows, col.length);
  }

  const cols = maxLayer + 1;
  const minW = PAD_MIN * 2 + cols * NODE_W + Math.max(0, cols - 1) * GAP_X_MIN;
  const minH = PAD_MIN * 2 + maxRows * NODE_H + Math.max(0, maxRows - 1) * GAP_Y_MIN;
  const viewW = Math.max(viewport?.width ?? 0, minW);
  const viewH = Math.max(viewport?.height ?? 0, minH);
  const spaced = spread(viewW, viewH, minW, minH, cols, maxRows);

  const usedH = maxRows * NODE_H + Math.max(0, maxRows - 1) * spaced.gapY;
  const laidNodes: LaidMapNode[] = [];
  const at = new Map<string, LaidMapNode>();
  for (let layer = 0; layer <= maxLayer; layer++) {
    const col = columns.get(layer) ?? [];
    const colH = col.length * NODE_H + Math.max(0, col.length - 1) * spaced.gapY;
    const originY = spaced.padY + (usedH - colH) / 2;
    for (let i = 0; i < col.length; i++) {
      const n = col[i];
      if (!n) {
        continue;
      }
      const node: LaidMapNode = {
        ...n,
        x: spaced.padX + layer * (NODE_W + spaced.gapX),
        y: originY + i * (NODE_H + spaced.gapY),
        w: NODE_W,
        h: NODE_H,
      };
      laidNodes.push(node);
      at.set(n.service, node);
    }
  }

  const maxAvg = edges.reduce((m, e) => Math.max(m, e.avg_duration_ns), 0);
  let height = viewH;
  const laidEdges: LaidMapEdge[] = [];
  for (const e of edges) {
    const a = at.get(e.from);
    const b = at.get(e.to);
    if (!a || !b) {
      continue;
    }
    const geom = edgeGeom(a, b);
    height = Math.max(height, geom.labelY + 18);
    laidEdges.push({
      ...e,
      path: geom.path,
      labelX: geom.labelX,
      labelY: geom.labelY,
      tone: edgeTone(e, maxAvg),
    });
  }

  return { nodes: laidNodes, edges: laidEdges, width: viewW, height };
}

function spread(
  viewW: number,
  viewH: number,
  minW: number,
  minH: number,
  cols: number,
  rows: number,
): { gapX: number; gapY: number; padX: number; padY: number } {
  const slotsX = Math.max(0, cols - 1);
  const slotsY = Math.max(0, rows - 1);
  let extraX = Math.max(0, viewW - minW);
  let extraY = Math.max(0, viewH - minH);
  let gapX = GAP_X_MIN;
  let gapY = GAP_Y_MIN;
  if (slotsX > 0) {
    const add = Math.min(GAP_X_MAX - GAP_X_MIN, extraX / slotsX);
    gapX = GAP_X_MIN + add;
    extraX -= add * slotsX;
  }
  if (slotsY > 0) {
    const add = Math.min(GAP_Y_MAX - GAP_Y_MIN, extraY / slotsY);
    gapY = GAP_Y_MIN + add;
    extraY -= add * slotsY;
  }
  return {
    gapX,
    gapY,
    padX: PAD_MIN + extraX / 2,
    padY: PAD_MIN + extraY / 2,
  };
}

function edgeGeom(a: LaidMapNode, b: LaidMapNode): { path: string; labelX: number; labelY: number } {
  const x1 = a.x + a.w;
  const y1 = a.y + a.h / 2;
  const x2 = b.x;
  const y2 = b.y + b.h / 2;
  if (x2 > x1 + 8) {
    const c = Math.max(36, (x2 - x1) / 2);
    const mid = cubicAt({ x: x1, y: y1 }, { x: x1 + c, y: y1 }, { x: x2 - c, y: y2 }, { x: x2, y: y2 }, 0.5);
    return {
      path: `M ${x1} ${y1} C ${x1 + c} ${y1}, ${x2 - c} ${y2}, ${x2} ${y2}`,
      labelX: mid.x,
      labelY: mid.y,
    };
  }
  const midY = Math.max(a.y + a.h, b.y + b.h) + 18;
  return {
    path: `M ${x1} ${y1} C ${x1 + 28} ${y1}, ${x1 + 28} ${midY}, ${(x1 + x2) / 2} ${midY} S ${x2 - 28} ${y2}, ${x2} ${y2}`,
    labelX: (x1 + x2) / 2,
    labelY: midY,
  };
}

function cubicAt(
  p0: { x: number; y: number },
  p1: { x: number; y: number },
  p2: { x: number; y: number },
  p3: { x: number; y: number },
  t: number,
): { x: number; y: number } {
  const u = 1 - t;
  return {
    x: u * u * u * p0.x + 3 * u * u * t * p1.x + 3 * u * t * t * p2.x + t * t * t * p3.x,
    y: u * u * u * p0.y + 3 * u * u * t * p1.y + 3 * u * t * t * p2.y + t * t * t * p3.y,
  };
}

function edgeTone(e: ServiceMapEdge, maxAvg: number): "ok" | "warn" | "err" {
  if (e.errors > 0) {
    return "err";
  }
  if (maxAvg >= 50_000_000 && e.avg_duration_ns >= maxAvg * 0.75) {
    return "warn";
  }
  return "ok";
}
