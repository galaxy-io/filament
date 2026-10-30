export interface AuthLayoutAsideFieldPalette {
  color: string;
  alpha: number;
}

export interface AuthLayoutAsideFieldWave {
  directionX: number;
  directionY: number;
  length: number;
  speed: number;
  weight: number;
}

export interface AuthLayoutAsideFieldRipple {
  originX: number;
  originY: number;
  length: number;
  speed: number;
  weight: number;
}

export interface AuthLayoutAsideFieldDot {
  x: number;
  y: number;
  phases: number[];
}

export interface AuthLayoutAsideFieldState {
  width: number;
  height: number;
  time: number;
  dots: AuthLayoutAsideFieldDot[];
}
