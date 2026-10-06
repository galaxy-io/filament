import { Children, type PropsWithChildren } from "react";

import { styled } from "@linaria/react";

import { t } from "@galaxy-io/dls/theme/tokens/t";

const ListWrapper = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  border-radius: 8px;
  border: 0.5px solid ${t.color.border.primary};
  background-color: ${t.color.background.primary};
  overflow: hidden;
`;

const ListItem = styled.div`
  padding: 12px;

  &:not(:last-child) {
    border-bottom: 0.5px solid ${t.color.border.primary};
  }
`;

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
