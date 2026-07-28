import { styled } from "@linaria/react";
import { BookOpenIcon, GithubLogoIcon } from "@phosphor-icons/react";
import { Link } from "@tanstack/react-router";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import GalaxyLogomark from "@galaxy-io/dls/icons/GalaxyLogomark";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import FilamentLogo from "@/assets/components/FilamentLogo";

import { NAV_ITEMS, NAVBAR_HEIGHT, type NavItem } from "@/layouts/main/constants";

import { useRouteMatch } from "@/hooks/useRouteMatch";

import { DOCUMENTATION_URL, GITHUB_REPO_URL } from "@/constants";

const NavbarWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: ${NAVBAR_HEIGHT}px;

  padding: 0 16px;

  display: flex;
  align-items: center;
  justify-content: space-between;

  background-color: ${({ theme }) => theme.color.background.base};
`);

const NavTabWrapper = withTheme(styled.div<PropsWithTheme<{ $isActive?: boolean }>>`
  padding-bottom: 8px;

  border-bottom: 2px solid
    ${({ theme, $isActive }) => ($isActive ? theme.color.text.primary : "transparent")};

  transition: border-color 100ms ease;
`);

const NavTabsWrapper = styled.div`
  height: 100%;

  padding: 16px 24px 0;

  display: flex;
  gap: 24px;
  align-items: flex-start;
`;

const MainLayoutNavTab = ({ item }: { item: NavItem }) => {
  const { isRouteMatch: isActive } = useRouteMatch({
    route: item.to,
    fuzzy: true,
  });

  return (
    <Link to={item.to}>
      <NavTabWrapper $isActive={isActive}>
        <Text
          size={TextSize.BODY_MD}
          variant={isActive ? TextVariant.PRIMARY : TextVariant.SECONDARY}
          weight={isActive ? TextWeight.MEDIUM : TextWeight.REGULAR}
          cursor="pointer"
        >
          {item.label}
        </Text>
      </NavTabWrapper>
    </Link>
  );
};

const MainLayoutNavbar = () => {
  const handleDocs = () => {
    window.open(DOCUMENTATION_URL, "_blank");
  };
  const handleStarRepository = () => {
    window.open(GITHUB_REPO_URL, "_blank");
  };

  return (
    <NavbarWrapper>
      <Link to={"/"}>
        <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
          <GalaxyLogomark height={12} />
          <FilamentLogo height={16} />
        </FlexWrapper>
      </Link>
      <NavTabsWrapper>
        {NAV_ITEMS.map((item) => (
          <MainLayoutNavTab key={item.to} item={item} />
        ))}
      </NavTabsWrapper>
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
        <Button
          label="Docs"
          icon={BookOpenIcon}
          onClick={handleDocs}
          variant={ButtonVariant.TERTIARY}
          size={ButtonSize.SMALL}
        />
        <Button
          icon={GithubLogoIcon}
          onClick={handleStarRepository}
          variant={ButtonVariant.SECONDARY}
          size={ButtonSize.SMALL}
        />
      </FlexWrapper>
    </NavbarWrapper>
  );
};

export default MainLayoutNavbar;
