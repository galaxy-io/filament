import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import GalaxyLogomarkAnimation from "@galaxy-io/dls/icons/GalaxyLogomarkAnimation";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

interface PendingLayoutProps {
  message?: string;
}

const PendingLayout = ({ message }: PendingLayoutProps) => {
  return (
    <FlexWrapper
      fillWidth
      fillHeight
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
      gap={FlexGap.MEDIUM}
    >
      <GalaxyLogomarkAnimation height={32} speed={2} />
      {message && (
        <Text size={TextSize.BODY_MD} variant={TextVariant.SECONDARY}>
          {message}
        </Text>
      )}
    </FlexWrapper>
  );
};

export default PendingLayout;
