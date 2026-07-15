import FlexWrapper, {
  AlignItems,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

interface PipelinesPageEmptyProps {
  hasSearchQuery?: boolean;
}

const PipelinesPageEmpty = ({ hasSearchQuery = false }: PipelinesPageEmptyProps) => {
  return (
    <FlexWrapper
      fillWidth
      fillHeight
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
    >
      <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
        {hasSearchQuery
          ? "No pipelines match your search."
          : "No pipelines found."}
      </Text>
    </FlexWrapper>
  );
};

export default PipelinesPageEmpty;
