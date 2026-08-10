import { match } from "ts-pattern";

import { useGalaxyTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import { GalaxyTheme } from "@galaxy-io/dls/theme/types";

import SvgFilamentLogoDark from "@/assets/components/FilamentLogoDark";
import SvgFilamentLogoLight from "@/assets/components/FilamentLogoLight";

const FilamentWordmark = ({ width, height }: { width?: number; height?: number }) => {
  const { activeTheme } = useGalaxyTheme();

  return match(activeTheme)
    .with(GalaxyTheme.DARK, () => <SvgFilamentLogoDark width={width} height={height} />)
    .with(GalaxyTheme.LIGHT, () => <SvgFilamentLogoLight width={width} height={height} />)
    .exhaustive();
};

export default FilamentWordmark;
