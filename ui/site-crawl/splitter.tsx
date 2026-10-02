import { useRef } from "react";

import { cn } from "@/lib/utils";

// Hand-rolled splitter (design.md data-grid section): pointer capture, live
// feedback written straight to a CSS variable on the page root — zero React
// renders per frame — and one store commit on pointerup. Arrow keys nudge for
// keyboard users. No dependency for 40 lines of code.
export function Splitter({
  orientation,
  cssVar,
  value,
  min,
  max,
  onCommit,
  rootRef,
  label,
}: {
  /** "vertical" separates left/right (drag changes a width); "horizontal" top/bottom (a height). */
  orientation: "vertical" | "horizontal";
  cssVar: string;
  value: number;
  min: number;
  max: number;
  onCommit: (v: number) => void;
  /** Element carrying the CSS variable. */
  rootRef: React.RefObject<HTMLElement | null>;
  label: string;
}) {
  const live = useRef(value);
  const dragging = useRef(false);

  const clamp = (v: number) => Math.max(min, Math.min(max, Math.round(v)));

  const start = (e: React.PointerEvent) => {
    e.preventDefault();
    const el = e.currentTarget as HTMLElement;
    el.setPointerCapture(e.pointerId);
    const startPos = orientation === "vertical" ? e.clientX : e.clientY;
    const startVal = live.current;
    dragging.current = true;

    const move = (ev: PointerEvent) => {
      const pos = orientation === "vertical" ? ev.clientX : ev.clientY;
      // The panel sits after the splitter (right / below), so dragging toward
      // it shrinks it.
      const v = clamp(startVal + (startPos - pos));
      live.current = v;
      rootRef.current?.style.setProperty(cssVar, `${v}px`);
    };
    const up = () => {
      el.removeEventListener("pointermove", move);
      el.removeEventListener("pointerup", up);
      el.removeEventListener("pointercancel", up);
      dragging.current = false;
      onCommit(live.current);
    };
    el.addEventListener("pointermove", move);
    el.addEventListener("pointerup", up);
    el.addEventListener("pointercancel", up);
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    const grow = orientation === "vertical" ? "ArrowLeft" : "ArrowUp";
    const shrink = orientation === "vertical" ? "ArrowRight" : "ArrowDown";
    if (e.key !== grow && e.key !== shrink) return;
    e.preventDefault();
    const v = clamp(live.current + (e.key === grow ? 16 : -16));
    live.current = v;
    rootRef.current?.style.setProperty(cssVar, `${v}px`);
    onCommit(v);
  };

  // Keep the live position in sync when the store changes underneath us — but
  // never mid-drag. This runs on every render, and during a drag the prop is
  // still the pre-drag store value (onCommit only fires on pointerup), so the
  // condition was always true and every render threw the drag away. A crawl
  // re-renders this page about twice a second, so pausing the pointer for a
  // moment was enough to lose the resize and make the next drag jump back.
  if (!dragging.current && live.current !== value) live.current = value;

  return (
    <div
      role="separator"
      aria-orientation={orientation}
      aria-label={label}
      aria-valuenow={value}
      aria-valuemin={min}
      aria-valuemax={max}
      tabIndex={0}
      onPointerDown={start}
      onKeyDown={onKeyDown}
      className={cn(
        "shrink-0 touch-none rounded-full transition-colors hover:bg-border focus-visible:bg-primary/40 focus-visible:outline-none",
        orientation === "vertical" ? "w-1 cursor-col-resize" : "h-1 cursor-row-resize",
      )}
    />
  );
}
