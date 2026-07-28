import { Children, type PropsWithChildren } from "react";

import { styled } from "@linaria/react";

import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

const ListWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;
  width: 100%;
  border-radius: 8px;
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  background-color: ${({ theme }) => theme.color.background.primary};
  overflow: hidden;
`);

const ListItem = withTheme(styled.div<PropsWithTheme>`
  padding: 12px;

  &:not(:last-child) {
    border-bottom: 0.5px solid ${({ theme }) => theme.color.border.primary};
  }
`);

const ConnectionDrawerList = ({ children }: PropsWithChildren) => {
  return (
    <ListWrapper>
      {Children.map(children, (child) => (
        <ListItem>{child}</ListItem>
      ))}
    </ListWrapper>
  );
};

export default ConnectionDrawerList;
