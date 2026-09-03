export interface AuthLayoutAsideFieldPalette {
  ink: string;
  tints: string[];
  halo: [number, number, number];
}

export interface AuthLayoutAsideFieldDot {
  x: number;
  y: number;
  alpha: number;
  radius: number;
  phase: number;
  tint: number;
}

export interface AuthLayoutAsideFieldPointer {
  x: number;
  y: number;
  targetX: number;
  targetY: number;
  presence: number;
  targetPresence: number;
  charge: number;
  targetCharge: number;
}

export interface AuthLayoutAsideFieldRipple {
  x: number;
  y: number;
  age: number;
  strength: number;
}

export interface AuthLayoutAsideFieldSprites {
  dots: HTMLCanvasElement[];
  halo: HTMLCanvasElement;
  pixelRatio: number;
}

export interface AuthLayoutAsideFieldState {
  width: number;
  height: number;
  columns: number;
  time: number;
  dots: AuthLayoutAsideFieldDot[];
  energy: Float32Array;
  offsetX: Float32Array;
  offsetY: Float32Array;
  velocityX: Float32Array;
  velocityY: Float32Array;
  ripples: AuthLayoutAsideFieldRipple[];
  pointer: AuthLayoutAsideFieldPointer;
}
