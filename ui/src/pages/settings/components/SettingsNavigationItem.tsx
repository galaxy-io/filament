import { styled } from "@linaria/react";
import type { Icon as PhosphorIcon } from "@phosphor-icons/react";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import Text, { TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

const NavigationButton = withTheme(
  styled.button<PropsWithTheme<{ $isActive?: boolean }>>`
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    min-width: 0;
    min-height: 32px;
    padding: 6px 8px;
    color: ${({ theme, $isActive }) =>
      $isActive ? theme.color.text.primary : theme.color.text.secondary};
    background-color: ${({ theme, $isActive }) =>
      $isActive ? theme.color.background.tertiary : "transparent"};
    border: 0.5px solid
      ${({ theme, $isActive }) => ($isActive ? theme.color.border.primary : "transparent")};
    border-radius: 5px;
    text-align: left;
    cursor: pointer;

    &:hover,
    &:focus-visible {
      color: ${({ theme }) => theme.color.text.primary};
      background-color: ${({ theme }) => theme.color.background.tertiary};
    }

    &:focus-visible {
      outline: 1px solid ${({ theme }) => theme.color.border.secondary};
      outline-offset: 1px;
    }
  `,
);

interface SettingsNavigationItemProps {
  label: string;
  icon: PhosphorIcon;
  isActive?: boolean;
  onClick: () => void;
}

const SettingsNavigationItem = ({
  label,
  icon,
  isActive = false,
  onClick,
}: SettingsNavigationItemProps) => (
  <NavigationButton type="button" $isActive={isActive} onClick={onClick}>
    <Icon
      component={icon}
      size={16}
      variant={isActive ? IconVariant.PRIMARY : IconVariant.TERTIARY}
      weight={isActive ? IconWeight.FILL : IconWeight.REGULAR}
    />
    <FlexItem grow={1} minWidth={0}>
      <Text weight={isActive ? TextWeight.MEDIUM : TextWeight.REGULAR} isEllipsis>
        {label}
      </Text>
    </FlexItem>
  </NavigationButton>
);

export default SettingsNavigationItem;
