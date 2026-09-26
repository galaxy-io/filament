import { MoonIcon, SunIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { useGalaxyTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import { GalaxyTheme } from "@galaxy-io/dls/theme/types";

interface ThemeButtonProps {
  variant?: ButtonVariant;
  size?: ButtonSize;
}

const ThemeButton = ({
  variant = ButtonVariant.SECONDARY,
  size = ButtonSize.SMALL,
}: ThemeButtonProps) => {
  const { activeTheme, setTheme } = useGalaxyTheme();

  const isDark = activeTheme === GalaxyTheme.DARK;

  const handleTheme = () => {
    setTheme(isDark ? GalaxyTheme.LIGHT : GalaxyTheme.DARK);
  };

  return (
    <Button
      icon={isDark ? SunIcon : MoonIcon}
      variant={variant}
      size={size}
      onClick={handleTheme}
      isIconFilled
    />
  );
};

export default ThemeButton;
