import { MoonIcon, SunIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { GalaxyTheme } from "@galaxy-io/dls/theme/enums";
import { useGalaxyTheme } from "@galaxy-io/dls/theme/useGalaxyTheme";

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
      /* @dls-migrate button.isIconFilled: Removed: pass the filled icon in `leading` at the rung's icon size. */ isIconFilled
    />
  );
};

export default ThemeButton;
