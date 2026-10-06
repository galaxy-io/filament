import { styled } from "@linaria/react";
import { WarningIcon } from "@phosphor-icons/react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import ConnectorTile from "@/pages/connectors/components/ConnectorTile";
import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";

const TabsWrapper = styled.div`
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;

  padding: 0 8px;
  overflow-x: auto;
`;

// @dls-migrate tokens.background.alt: Inverse is a scope, not a token: render the opposite-theme surface as `<GalaxyProvider isScoped theme={…}>` around a `Box variant`, and read the normal roles inside it.
const TabButton = styled.button<{ $isActive: boolean }>`
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;

  padding: 10px;

  opacity: ${({ $isActive }) => ($isActive ? 1 : 0.5)};

  border: none;
  border-bottom: 2px solid
    ${({ $isActive }) => ($isActive ? t.color.background.primaryAlt : "transparent")};
  background-color: transparent;
  cursor: pointer;
`;

const CreatePipelineModalResourcesTabs = () => {
  const { sinks, activeSinkId, selectedCountBySink, issuesBySink } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  return (
    <TabsWrapper>
      {sinks.map((sink) => {
        const isActive = sink.connection.id === activeSinkId;
        const count = selectedCountBySink[sink.connection.id] ?? 0;
        const issues = issuesBySink[sink.connection.id] ?? [];

        return (
          <TabButton
            key={sink.connection.id}
            $isActive={isActive}
            onClick={() =>
              dispatch({
                type: CreatePipelineModalActionType.SET_ACTIVE_SINK,
                payload: sink.connection.id,
              })
            }
          >
            <ConnectorTile connector={sink.connection.connector} kind={sink.connection.kind} />
            <Flex alignItems={AlignItems.CENTER} gap={12}>
              <Text size={TextSize.BODY_SM} variant={TextVariant.PRIMARY}>
                {sink.connection.name}
              </Text>
              {issues.length ? (
                <Icon
                  component={WarningIcon}
                  variant={IconVariant.ERROR}
                  weight={IconWeight.FILL}
                />
              ) : (
                <Chip
                  hasBorder
                  isPill
                  count={count}
                  variant={ChipVariant.SECONDARY}
                  size={ChipSize.SMALL}
                />
              )}
            </Flex>
          </TabButton>
        );
      })}
    </TabsWrapper>
  );
};

export default CreatePipelineModalResourcesTabs;
