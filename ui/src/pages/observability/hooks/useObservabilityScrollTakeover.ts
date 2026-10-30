import { useEffect, useRef } from "react";

import {
  getObservabilityWheelDeltaPx,
  isObservabilityScrolledToEnd,
} from "@/pages/observability/utils";

export const useObservabilityScrollTakeover = (isEnabled: boolean) => {
  const scrollRef = useRef<HTMLDivElement>(null);
  const takeoverRef = useRef<HTMLElement>(null);

  useEffect(() => {
    const scroll = scrollRef.current;
    if (!isEnabled || scroll === null) return;

    const handleWheel = (event: WheelEvent) => {
      const takeover = takeoverRef.current;
      if (event.deltaY <= 0 || event.ctrlKey || takeover === null) return;
      if (!(event.target instanceof Node) || !takeover.contains(event.target)) return;
      if (isObservabilityScrolledToEnd(scroll)) return;
      event.preventDefault();
      scroll.scrollBy({ top: getObservabilityWheelDeltaPx(event, scroll.clientHeight) });
    };

    scroll.addEventListener("wheel", handleWheel, { passive: false });
    return () => scroll.removeEventListener("wheel", handleWheel);
  }, [isEnabled]);

  return { scrollRef, takeoverRef };
};
