import FlexWrapper, {
  AlignItems,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

interface ConnectorsPageEmptyProps {
  hasSearchQuery?: boolean;
}

const ConnectorsPageEmpty = ({ hasSearchQuery = false }: ConnectorsPageEmptyProps) => {
  return (
    <FlexWrapper
      fillWidth
      fillHeight
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
    >
      <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
        {hasSearchQuery
          ? "No connectors match your search."
          : "No connectors found."}
      </Text>
    </FlexWrapper>
  );
};

export default ConnectorsPageEmpty;
