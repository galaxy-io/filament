import Square, { SquareSize } from "@galaxy-io/dls/shapes/Square";

import { type Hue, hueToSquareMark } from "@/utils/hue";

interface HueSquareProps {
  hue: Hue | null;
  size?: SquareSize;
}

const HueSquare = ({ hue, size = SquareSize.SMALL }: HueSquareProps) => (
  <Square size={size} {...hueToSquareMark(hue)} />
);

export default HueSquare;
