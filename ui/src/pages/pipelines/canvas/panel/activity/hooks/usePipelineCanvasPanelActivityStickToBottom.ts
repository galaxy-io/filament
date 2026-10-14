import { type RefObject, useEffect, useState } from "react";

import {
  PIPELINE_CANVAS_PANEL_ACTIVITY_STICK_THRESHOLD_PX,
  PIPELINE_CANVAS_PANEL_ACTIVITY_UNPIN_KEYS,
} from "@/pages/pipelines/canvas/panel/activity/constants";

export const usePipelineCanvasPanelActivityStickToBottom = (
  scrollerRef: RefObject<HTMLElement | null>,
  contentRef: RefObject<HTMLElement | null>,
  isEnabled: boolean,
) => {
  const [isPinned, setIsPinned] = useState(true);

  useEffect(() => {
    const scroller = scrollerRef.current;
    const content = contentRef.current;
    const root = scroller?.parentElement;
    if (!isEnabled || !scroller || !content || !root) return;

    let isPinnedNow = true;
    const setPinned = (next: boolean) => {
      if (isPinnedNow === next) return;
      isPinnedNow = next;
      setIsPinned(next);
    };

    const isAtBottom = () =>
      scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight <=
      PIPELINE_CANVAS_PANEL_ACTIVITY_STICK_THRESHOLD_PX;
    const scrollToBottom = () => {
      if (isPinnedNow) scroller.scrollTop = scroller.scrollHeight;
    };
    const repinAtBottom = () => {
      if (isAtBottom()) setPinned(true);
    };
    const unpin = () => setPinned(false);
    const handleWheel = (event: WheelEvent) => {
      if (event.deltaY < 0) unpin();
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (PIPELINE_CANVAS_PANEL_ACTIVITY_UNPIN_KEYS.has(event.key)) unpin();
    };

    const observer = new ResizeObserver(scrollToBottom);
    observer.observe(content);
    observer.observe(scroller);
    scroller.addEventListener("wheel", handleWheel, { passive: true });
    scroller.addEventListener("keydown", handleKeyDown);
    scroller.addEventListener("scroll", repinAtBottom, { passive: true });
    root.addEventListener("pointerdown", unpin);
    root.addEventListener("pointerup", repinAtBottom);
    scrollToBottom();

    return () => {
      observer.disconnect();
      scroller.removeEventListener("wheel", handleWheel);
      scroller.removeEventListener("keydown", handleKeyDown);
      scroller.removeEventListener("scroll", repinAtBottom);
      root.removeEventListener("pointerdown", unpin);
      root.removeEventListener("pointerup", repinAtBottom);
      setIsPinned(true);
    };
  }, [scrollerRef, contentRef, isEnabled]);

  return isPinned;
};
