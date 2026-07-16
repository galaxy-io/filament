import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";

interface EmptyLayoutProps {
  icon?: React.ReactNode;
  header?: string;
  message?: string;
  actions?: React.ReactNode;
}

const EmptyLayout = ({ icon, header, message, actions }: EmptyLayoutProps) => {
  return (
    <FlexWrapper
      fillWidth
      fillHeight
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
      gap={24}
    >
      {icon && icon}
      <FlexWrapper direction={FlexDirection.COLUMN} alignItems={AlignItems.CENTER} gap={8}>
        {header && (
          <Text size={TextSize.HEADING_SM} weight={TextWeight.MEDIUM}>
            {header}
          </Text>
        )}
        {message && <Text variant={TextVariant.SECONDARY}>{message}</Text>}
      </FlexWrapper>
      {actions && actions}
    </FlexWrapper>
  );
};

export default EmptyLayout;
