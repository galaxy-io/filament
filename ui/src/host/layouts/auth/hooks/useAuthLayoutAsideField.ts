import { useEffect, useRef } from "react";

import { usePrefersReducedMotion } from "@galaxy-io/dls/hooks/usePrefersReducedMotion";
import { useGalaxyTheme } from "@galaxy-io/dls/theme/useGalaxyTheme";

import {
  AUTH_LAYOUT_ASIDE_FIELD_MAX_DELTA,
  AUTH_LAYOUT_ASIDE_FIELD_PALETTE_MAP,
} from "@/host/layouts/auth/constants";
import {
  createAuthLayoutAsideFieldSprite,
  createAuthLayoutAsideFieldState,
  drawAuthLayoutAsideField,
  resizeAuthLayoutAsideFieldState,
} from "@/host/layouts/auth/utils";

export const useAuthLayoutAsideField = () => {
  const { activeTheme } = useGalaxyTheme();
  const isMotionReduced = usePrefersReducedMotion();
  const wrapperRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const wrapper = wrapperRef.current;
    const canvas = canvasRef.current;
    const context = canvas?.getContext("2d");
    if (!wrapper || !canvas || !context) return;

    const pixelRatio = window.devicePixelRatio || 1;
    const palette = AUTH_LAYOUT_ASIDE_FIELD_PALETTE_MAP[activeTheme];
    const sprite = createAuthLayoutAsideFieldSprite(palette, pixelRatio);
    const state = createAuthLayoutAsideFieldState();

    const resize = () => {
      const { width, height } = wrapper.getBoundingClientRect();
      if (width === 0 || height === 0) return;

      canvas.width = Math.round(width * pixelRatio);
      canvas.height = Math.round(height * pixelRatio);
      context.setTransform(pixelRatio, 0, 0, pixelRatio, 0, 0);
      resizeAuthLayoutAsideFieldState(state, width, height);

      if (isMotionReduced) {
        drawAuthLayoutAsideField(context, state, palette, sprite, pixelRatio);
      }
    };

    const observer = new ResizeObserver(resize);
    observer.observe(wrapper);

    if (isMotionReduced) {
      return () => observer.disconnect();
    }

    let frame = 0;
    let previous = performance.now();

    const render = (now: number) => {
      state.time += Math.min((now - previous) / 1000, AUTH_LAYOUT_ASIDE_FIELD_MAX_DELTA);
      previous = now;
      drawAuthLayoutAsideField(context, state, palette, sprite, pixelRatio);
      frame = requestAnimationFrame(render);
    };

    frame = requestAnimationFrame(render);

    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
    };
  }, [activeTheme, isMotionReduced]);

  return { wrapperRef, canvasRef };
};
