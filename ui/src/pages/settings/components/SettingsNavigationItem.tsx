import { styled } from "@linaria/react";
import type { Icon as PhosphorIcon } from "@phosphor-icons/react";

import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextWeight } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";

const NavigationButton = styled.button<{ $isActive?: boolean }>`
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    min-width: 0;
    min-height: 32px;
    padding: 6px 8px;
    color: ${({ $isActive }) => ($isActive ? t.color.text.primary : t.color.text.secondary)};
    background-color: ${({ $isActive }) =>
      $isActive ? t.color.background.tertiary : "transparent"};
    border: 0.5px solid
      ${({ $isActive }) => ($isActive ? t.color.border.primary : "transparent")};
    border-radius: 5px;
    text-align: left;
    cursor: pointer;

    &:hover,
    &:focus-visible {
      color: ${t.color.text.primary};
      background-color: ${t.color.background.tertiary};
    }

    &:focus-visible {
      outline: 1px solid ${t.color.border.secondary};
      outline-offset: 1px;
    }
  `;

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
      <Text weight={isActive ? TextWeight.MEDIUM : TextWeight.REGULAR} lineClamp={1}>
        {label}
      </Text>
    </FlexItem>
  </NavigationButton>
);

export default SettingsNavigationItem;
