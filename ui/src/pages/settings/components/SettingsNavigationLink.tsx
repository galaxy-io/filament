import { styled } from "@linaria/react";
import { ArrowSquareOutIcon, type Icon as PhosphorIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";

const NavigationAnchor = styled.a`
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  min-width: 0;
  min-height: 32px;
  padding: 6px 8px;
  color: ${t.color.text.secondary};
  background-color: transparent;
  border: 0.5px solid transparent;
  border-radius: 5px;
  text-align: left;
  text-decoration: none;
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

interface SettingsNavigationLinkProps {
  label: string;
  icon: PhosphorIcon;
  href: string;
}

const SettingsNavigationLink = ({ label, icon, href }: SettingsNavigationLinkProps) => (
  <NavigationAnchor href={href} target="_blank" rel="noreferrer">
    <Icon component={icon} size={16} variant={IconVariant.TERTIARY} />
    <FlexItem grow={1} minWidth={0}>
      <Text lineClamp={1}>{label}</Text>
    </FlexItem>
    <Icon component={ArrowSquareOutIcon} size={13} variant={IconVariant.TERTIARY} />
  </NavigationAnchor>
);

export default SettingsNavigationLink;
