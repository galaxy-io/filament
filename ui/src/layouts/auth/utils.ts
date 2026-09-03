import {
  AUTH_LAYOUT_ASIDE_FIELD_BASE,
  AUTH_LAYOUT_ASIDE_FIELD_DOT_RADIUS,
  AUTH_LAYOUT_ASIDE_FIELD_DRIFT_SCALE,
  AUTH_LAYOUT_ASIDE_FIELD_DRIFT_WEIGHT,
  AUTH_LAYOUT_ASIDE_FIELD_DRIFT_X,
  AUTH_LAYOUT_ASIDE_FIELD_DRIFT_Y,
  AUTH_LAYOUT_ASIDE_FIELD_JITTER,
  AUTH_LAYOUT_ASIDE_FIELD_PULSE,
  AUTH_LAYOUT_ASIDE_FIELD_RIPPLES,
  AUTH_LAYOUT_ASIDE_FIELD_SPACING,
  AUTH_LAYOUT_ASIDE_FIELD_WAVES,
} from "@/layouts/auth/constants";
import type {
  AuthLayoutAsideFieldDot,
  AuthLayoutAsideFieldPalette,
  AuthLayoutAsideFieldState,
} from "@/layouts/auth/types";

const AUTH_LAYOUT_ASIDE_FIELD_WAVE_VECTORS = AUTH_LAYOUT_ASIDE_FIELD_WAVES.map((wave) => {
  const magnitude = Math.hypot(wave.directionX, wave.directionY) || 1;
  const waveNumber = (Math.PI * 2) / wave.length;

  return {
    x: (wave.directionX / magnitude) * waveNumber,
    y: (wave.directionY / magnitude) * waveNumber,
    angularSpeed: waveNumber * wave.speed,
    weight: wave.weight,
  };
});

const AUTH_LAYOUT_ASIDE_FIELD_RIPPLE_VECTORS = AUTH_LAYOUT_ASIDE_FIELD_RIPPLES.map((ripple) => {
  const waveNumber = (Math.PI * 2) / ripple.length;

  return {
    originX: ripple.originX,
    originY: ripple.originY,
    waveNumber,
    angularSpeed: waveNumber * ripple.speed,
    weight: ripple.weight,
  };
});

const AUTH_LAYOUT_ASIDE_FIELD_SOURCE_SPEEDS = [
  ...AUTH_LAYOUT_ASIDE_FIELD_WAVE_VECTORS,
  ...AUTH_LAYOUT_ASIDE_FIELD_RIPPLE_VECTORS,
].map((source) => ({ angularSpeed: source.angularSpeed, weight: source.weight }));

const AUTH_LAYOUT_ASIDE_FIELD_SCALE =
  1 /
  Math.hypot(
    ...AUTH_LAYOUT_ASIDE_FIELD_SOURCE_SPEEDS.map((source) => source.weight),
    AUTH_LAYOUT_ASIDE_FIELD_DRIFT_WEIGHT,
  );

const hashCell = (x: number, y: number) => {
  let value = Math.imul(x, 374761393) + Math.imul(y, 668265263);
  value = Math.imul(value ^ (value >>> 13), 1274126177);

  return ((value ^ (value >>> 16)) >>> 0) / 4294967296;
};

const smoothStep = (value: number) => value * value * (3 - 2 * value);

const sampleDrift = (x: number, y: number) => {
  const cellX = Math.floor(x);
  const cellY = Math.floor(y);
  const weightX = smoothStep(x - cellX);
  const weightY = smoothStep(y - cellY);
  const topLeft = hashCell(cellX, cellY);
  const topRight = hashCell(cellX + 1, cellY);
  const bottomLeft = hashCell(cellX, cellY + 1);
  const bottomRight = hashCell(cellX + 1, cellY + 1);
  const top = topLeft + (topRight - topLeft) * weightX;
  const bottom = bottomLeft + (bottomRight - bottomLeft) * weightX;

  return top + (bottom - top) * weightY;
};

export const createAuthLayoutAsideFieldState = (): AuthLayoutAsideFieldState => ({
  width: 0,
  height: 0,
  time: 0,
  dots: [],
});

export const resizeAuthLayoutAsideFieldState = (
  state: AuthLayoutAsideFieldState,
  width: number,
  height: number,
) => {
  const columns = Math.ceil(width / AUTH_LAYOUT_ASIDE_FIELD_SPACING) + 1;
  const rows = Math.ceil(height / AUTH_LAYOUT_ASIDE_FIELD_SPACING) + 1;
  const dots: AuthLayoutAsideFieldDot[] = [];

  for (let row = 0; row < rows; row += 1) {
    for (let column = 0; column < columns; column += 1) {
      const x = column * AUTH_LAYOUT_ASIDE_FIELD_SPACING;
      const y = row * AUTH_LAYOUT_ASIDE_FIELD_SPACING;
      const jitter = hashCell(x, y) * AUTH_LAYOUT_ASIDE_FIELD_JITTER;

      dots.push({
        x,
        y,
        phases: [
          ...AUTH_LAYOUT_ASIDE_FIELD_WAVE_VECTORS.map(
            (vector) => x * vector.x + y * vector.y + jitter,
          ),
          ...AUTH_LAYOUT_ASIDE_FIELD_RIPPLE_VECTORS.map(
            (vector) =>
              Math.hypot(x - vector.originX * width, y - vector.originY * height) *
                vector.waveNumber +
              jitter,
          ),
        ],
      });
    }
  }

  state.width = width;
  state.height = height;
  state.dots = dots;
};

export const createAuthLayoutAsideFieldSprite = (
  palette: AuthLayoutAsideFieldPalette,
  pixelRatio: number,
) => {
  const size = Math.ceil(AUTH_LAYOUT_ASIDE_FIELD_DOT_RADIUS * 2 * pixelRatio) + 2;
  const canvas = document.createElement("canvas");
  canvas.width = size;
  canvas.height = size;

  const context = canvas.getContext("2d");
  if (context) {
    context.fillStyle = palette.color;
    context.beginPath();
    context.arc(
      size / 2,
      size / 2,
      AUTH_LAYOUT_ASIDE_FIELD_DOT_RADIUS * pixelRatio,
      0,
      Math.PI * 2,
    );
    context.fill();
  }

  return canvas;
};

export const drawAuthLayoutAsideField = (
  context: CanvasRenderingContext2D,
  state: AuthLayoutAsideFieldState,
  palette: AuthLayoutAsideFieldPalette,
  sprite: HTMLCanvasElement,
  pixelRatio: number,
) => {
  const { dots, time } = state;
  const size = sprite.width / pixelRatio;
  const offset = size / 2;
  const sourceTimes = AUTH_LAYOUT_ASIDE_FIELD_SOURCE_SPEEDS.map(
    (source) => time * source.angularSpeed,
  );
  const driftX = time * AUTH_LAYOUT_ASIDE_FIELD_DRIFT_X;
  const driftY = time * AUTH_LAYOUT_ASIDE_FIELD_DRIFT_Y;

  context.clearRect(0, 0, state.width, state.height);

  for (const dot of dots) {
    let level =
      (sampleDrift(
        (dot.x + driftX) * AUTH_LAYOUT_ASIDE_FIELD_DRIFT_SCALE,
        (dot.y + driftY) * AUTH_LAYOUT_ASIDE_FIELD_DRIFT_SCALE,
      ) *
        2 -
        1) *
      AUTH_LAYOUT_ASIDE_FIELD_DRIFT_WEIGHT;

    for (let index = 0; index < sourceTimes.length; index += 1) {
      level +=
        Math.sin(sourceTimes[index] - dot.phases[index]) *
        AUTH_LAYOUT_ASIDE_FIELD_SOURCE_SPEEDS[index].weight;
    }

    context.globalAlpha = Math.max(
      0,
      Math.min(
        1,
        palette.alpha *
          (AUTH_LAYOUT_ASIDE_FIELD_BASE +
            level * AUTH_LAYOUT_ASIDE_FIELD_SCALE * AUTH_LAYOUT_ASIDE_FIELD_PULSE),
      ),
    );
    context.drawImage(sprite, dot.x - offset, dot.y - offset, size, size);
  }

  context.globalAlpha = 1;
};
