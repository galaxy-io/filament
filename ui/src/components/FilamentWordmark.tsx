import { useGalaxyTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import SvgFilamentLogoDark from "@/assets/components/FilamentLogoDark";
import SvgFilamentLogoLight from "@/assets/components/FilamentLogoLight";
import { match } from "ts-pattern";
import { GalaxyTheme } from "@galaxy-io/dls/theme/constants";

const FilamentWordmark = ({
  width,
  height,
}: {
  width?: number;
  height?: number;
}) => {
  const { activeTheme } = useGalaxyTheme();

  return match(activeTheme)
    .with(GalaxyTheme.DARK, () => (
      <SvgFilamentLogoDark width={width} height={height} />
    ))
    .with(GalaxyTheme.LIGHT, () => (
      <SvgFilamentLogoLight width={width} height={height} />
    ))
    .exhaustive();
};

export default FilamentWordmark;
