import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import { IS_DEBUG } from "@/constants";

interface ErrorLayoutProps {
  icon?: React.ReactNode;
  header?: string;
  message?: string;
  error?: Error;
  actions?: React.ReactNode;
}

const ErrorLayout = ({ icon, header, message, error, actions }: ErrorLayoutProps) => {
  return (
    <FlexWrapper
      fillWidth
      fillHeight
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
      gap={FlexGap.MEDIUM}
    >
      {icon && icon}
      <FlexWrapper
        direction={FlexDirection.COLUMN}
        alignItems={AlignItems.CENTER}
        gap={FlexGap.SMALL}
      >
        {header && <Text size={TextSize.BODY_LG}>{header}</Text>}
        {message && (
          <Text size={TextSize.BODY_MD} variant={TextVariant.SECONDARY}>
            {message}
          </Text>
        )}
        {IS_DEBUG && error && (
          <Text size={TextSize.BODY_SM} variant={TextVariant.ERROR} isMonospace isSelectable>
            {error.message}
          </Text>
        )}
      </FlexWrapper>
      {actions}
    </FlexWrapper>
  );
};

export default ErrorLayout;
