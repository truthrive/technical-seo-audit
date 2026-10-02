import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  forceCenter,
  forceLink,
  forceManyBody,
  forceSimulation,
  type SimulationLinkDatum,
  type SimulationNodeDatum,
} from "d3-force";

import * as SiteCrawlService from "@/../bindings/onescout/desktop/internal/tools/sitecrawl/service";
import { Spinner } from "@/components/ui/spinner";

interface SimNode extends SimulationNodeDatum {
  id: number;
  url: string;
  status: number;
  inlinks: number;
  internal: boolean;
}

// The tokens this canvas paints with. Canvas takes no CSS, so they are resolved
// off the element once per draw pass instead of being hardcoded — the edge
// stroke picked for a light bg-card was all but invisible in dark theme, leaving
// a cloud of unconnected dots. design.md §1/§2: colour always through tokens.
type Palette = {
  ok: string;
  redirect: string;
  error: string;
  unknown: string;
  external: string;
  edge: string;
};

function readPalette(el: HTMLElement): Palette {
  const cs = getComputedStyle(el);
  const token = (name: string, fallback: string) =>
    cs.getPropertyValue(name).trim() || fallback;
  return {
    ok: token("--success", "#17c964"),
    redirect: token("--warning", "#f5a524"),
    error: token("--destructive", "#f31260"),
    unknown: token("--muted-foreground", "#71717a"),
    external: token("--muted-foreground", "#a1a1aa"),
    edge: token("--border", "rgba(113,113,122,0.14)"),
  };
}

// Status → dot colour, the same 4 meanings as the grid's status chips.
function nodeColor(p: Palette, status: number, internal: boolean): string {
  if (!internal) return p.external; // external, context only
  if (status >= 200 && status < 300) return p.ok;
  if (status >= 300 && status < 400) return p.redirect;
  if (status >= 400) return p.error;
  return p.unknown;
}

function nodeRadius(inlinks: number): number {
  return Math.max(2.5, Math.min(11, 2.5 + Math.sqrt(inlinks)));
}

// Force-directed link graph on ONE canvas. No DOM nodes per page — at 3000
// nodes the browser would melt — and no d3-zoom dependency: wheel + drag are
// 30 lines by hand. The simulation runs in rAF and paints straight to canvas.
export function VisualizationPanel({ runId }: { runId: string }) {
  const { t } = useTranslation();
  const wrapRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [loading, setLoading] = useState(true);
  const [meta, setMeta] = useState<{ shown: number; total: number } | null>(null);
  const [hover, setHover] = useState<{ url: string; x: number; y: number } | null>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    const wrap = wrapRef.current;
    if (!canvas || !wrap) return;

    let disposed = false;
    let nodes: SimNode[] = [];
    let links: SimulationLinkDatum<SimNode>[] = [];
    let sim: ReturnType<typeof forceSimulation<SimNode>> | null = null;
    let raf = 0;

    // View transform, mutated directly by wheel/drag — never React state.
    const view = { x: 0, y: 0, k: 1 };

    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const resize = () => {
      const dpr = window.devicePixelRatio || 1;
      canvas.width = wrap.clientWidth * dpr;
      canvas.height = wrap.clientHeight * dpr;
      canvas.style.width = `${wrap.clientWidth}px`;
      canvas.style.height = `${wrap.clientHeight}px`;
    };
    resize();
    const ro = new ResizeObserver(() => {
      resize();
      draw();
    });
    ro.observe(wrap);

    const draw = () => {
      const palette = readPalette(wrap);
      const dpr = window.devicePixelRatio || 1;
      const w = canvas.width / dpr;
      const h = canvas.height / dpr;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, w, h);
      ctx.translate(w / 2 + view.x, h / 2 + view.y);
      ctx.scale(view.k, view.k);

      ctx.strokeStyle = palette.edge;
      ctx.lineWidth = 0.5;
      ctx.beginPath();
      for (const l of links) {
        const s = l.source as SimNode;
        const d = l.target as SimNode;
        if (s.x == null || d.x == null) continue;
        ctx.moveTo(s.x!, s.y!);
        ctx.lineTo(d.x!, d.y!);
      }
      ctx.stroke();

      for (const n of nodes) {
        if (n.x == null) continue;
        ctx.beginPath();
        ctx.arc(n.x!, n.y!, nodeRadius(n.inlinks), 0, Math.PI * 2);
        ctx.fillStyle = nodeColor(palette, n.status, n.internal);
        ctx.fill();
      }
    };

    // The loop stops once the layout settles. forceSimulation ends its own timer
    // when alpha decays, but this loop had no exit and only died on unmount — so
    // a 3000-node graph left open in the background kept clearing the canvas,
    // restroking every edge and issuing 3000 arcs at 60fps forever.
    const tick = () => {
      draw();
      if (sim && sim.alpha() > sim.alphaMin()) {
        raf = requestAnimationFrame(tick);
      } else {
        raf = 0;
      }
    };

    // Pan and zoom move the view without the simulation running, so they have to
    // ask for a repaint themselves.
    const wake = () => {
      if (raf !== 0 || disposed) return;
      raf = requestAnimationFrame(() => {
        raf = 0;
        draw();
      });
    };

    // --- interaction: wheel zoom, drag pan, hover tooltip ---
    const onWheel = (e: WheelEvent) => {
      e.preventDefault();
      const next = Math.max(0.1, Math.min(8, view.k * (e.deltaY < 0 ? 1.15 : 1 / 1.15)));
      view.k = next;
      wake();
    };
    let dragging = false;
    let lastX = 0;
    let lastY = 0;
    const onDown = (e: PointerEvent) => {
      dragging = true;
      lastX = e.clientX;
      lastY = e.clientY;
      canvas.setPointerCapture(e.pointerId);
    };
    const onMove = (e: PointerEvent) => {
      if (dragging) {
        view.x += e.clientX - lastX;
        view.y += e.clientY - lastY;
        lastX = e.clientX;
        lastY = e.clientY;
        wake();
        return;
      }
      // Hover: invert the transform, find the nearest node within its radius.
      const rect = canvas.getBoundingClientRect();
      const gx = (e.clientX - rect.left - rect.width / 2 - view.x) / view.k;
      const gy = (e.clientY - rect.top - rect.height / 2 - view.y) / view.k;
      let best: SimNode | null = null;
      let bestD = 12;
      for (const n of nodes) {
        if (n.x == null) continue;
        const d = Math.hypot(n.x! - gx, n.y! - gy);
        if (d < bestD + nodeRadius(n.inlinks) && d < (best ? bestD : Infinity)) {
          best = n;
          bestD = d;
        }
      }
      setHover(best ? { url: best.url, x: e.clientX - rect.left, y: e.clientY - rect.top } : null);
    };
    const onUp = () => {
      dragging = false;
    };
    // Named, so the cleanup below can actually remove it: as an inline arrow it
    // was unremovable, and the effect re-runs per runId while the canvas node
    // stays the same — so every finished crawl left another dead closure on it.
    const onLeave = () => setHover(null);
    canvas.addEventListener("wheel", onWheel, { passive: false });
    canvas.addEventListener("pointerdown", onDown);
    canvas.addEventListener("pointermove", onMove);
    canvas.addEventListener("pointerup", onUp);
    canvas.addEventListener("pointerleave", onLeave);

    setLoading(true);
    SiteCrawlService.Graph(runId, 3000)
      .then((g) => {
        if (disposed || !g) return;
        nodes = (g.nodes ?? []).map((n) => ({
          id: n.id,
          url: n.url,
          status: n.status,
          inlinks: n.inlinks,
          internal: n.internal,
        }));
        const byId = new Map(nodes.map((n) => [n.id, n]));
        links = (g.edges ?? [])
          .filter((e) => byId.has(e.src) && byId.has(e.dst))
          .map((e) => ({ source: byId.get(e.src)!, target: byId.get(e.dst)! }));
        setMeta({ shown: nodes.length, total: g.total });
        setLoading(false);

        sim = forceSimulation(nodes)
          .force("charge", forceManyBody().strength(-25).distanceMax(400))
          .force("link", forceLink(links).distance(40).strength(0.3))
          .force("center", forceCenter(0, 0))
          .alphaDecay(0.03);
        raf = requestAnimationFrame(tick);
      })
      .catch(() => setLoading(false));

    return () => {
      disposed = true;
      cancelAnimationFrame(raf);
      sim?.stop();
      ro.disconnect();
      canvas.removeEventListener("wheel", onWheel);
      canvas.removeEventListener("pointerdown", onDown);
      canvas.removeEventListener("pointermove", onMove);
      canvas.removeEventListener("pointerup", onUp);
      canvas.removeEventListener("pointerleave", onLeave);
    };
  }, [runId]);

  return (
    <div
      ref={wrapRef}
      className="relative min-h-0 flex-1 overflow-hidden rounded-xl bg-card shadow-soft"
    >
      <canvas ref={canvasRef} className="block h-full w-full cursor-grab active:cursor-grabbing" />
      {loading && (
        <div className="absolute inset-0 flex items-center justify-center">
          <Spinner className="size-5 text-muted-foreground" />
        </div>
      )}
      {meta && meta.shown < meta.total && (
        <span className="absolute bottom-2 left-3 text-xs text-muted-foreground tabular-nums">
          {t("siteCrawl.graphShowing", { shown: meta.shown, total: meta.total })}
        </span>
      )}
      {hover && (
        <span
          className="pointer-events-none absolute z-10 max-w-96 truncate rounded-md bg-foreground/90 px-2 py-1 text-xs text-background"
          style={{ left: hover.x + 12, top: hover.y + 12 }}
        >
          {hover.url}
        </span>
      )}
    </div>
  );
}
