import { styled } from "@linaria/react";
import { ArrowSquareOutIcon, type Icon as PhosphorIcon } from "@phosphor-icons/react";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

const NavigationAnchor = withTheme(styled.a<PropsWithTheme>`
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  min-width: 0;
  min-height: 32px;
  padding: 6px 8px;
  color: ${({ theme }) => theme.color.text.secondary};
  background-color: transparent;
  border: 0.5px solid transparent;
  border-radius: 5px;
  text-align: left;
  text-decoration: none;
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
`);

interface SettingsNavigationLinkProps {
  label: string;
  icon: PhosphorIcon;
  href: string;
}

const SettingsNavigationLink = ({ label, icon, href }: SettingsNavigationLinkProps) => (
  <NavigationAnchor href={href} target="_blank" rel="noreferrer">
    <Icon component={icon} size={16} variant={IconVariant.TERTIARY} />
    <FlexItem grow={1} minWidth={0}>
      <Text isEllipsis>{label}</Text>
    </FlexItem>
    <Icon component={ArrowSquareOutIcon} size={13} variant={IconVariant.TERTIARY} />
  </NavigationAnchor>
);

export default SettingsNavigationLink;
