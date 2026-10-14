import type { FC } from "react";

import { InfoIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import {
  EXECUTION_MODE_TO_DETAILS_MAP,
  EXECUTION_MODE_TO_LABEL_MAP,
} from "@/pages/pipelines/components/create/constants";
import CreatePipelineModalConnectionsExecutionModeOption from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalConnectionsExecutionModeOption";
import { PIPELINE_EXECUTION_MODES } from "@/pages/pipelines/constants";

const CreatePipelineModalConnectionsExecutionMode: FC = () => {
  const { executionMode } = useCreatePipelineModalState();
  const { setExecutionMode } = useCreatePipelineModalActions();

  return (
    <Box fillWidth>
      <Widget variant={WidgetVariant.PRIMARY}>
        <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={12} fillWidth>
          <Flex alignItems={AlignItems.CENTER} gap={4}>
            <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM} variant={TextVariant.TERTIARY}>
              Execution type
            </Text>
            <Icon
              component={InfoIcon}
              variant={IconVariant.TERTIARY}
              size={14}
              ariaLabel="About execution types"
              tooltip={
                <Flex
                  alignItems={AlignItems.START}
                  direction={FlexDirection.COLUMN}
                  gap={8}
                  maxWidth={320}
                >
                  {PIPELINE_EXECUTION_MODES.map((mode) => (
                    <Flex
                      alignItems={AlignItems.START}
                      key={mode}
                      direction={FlexDirection.COLUMN}
                      gap={2}
                    >
                      <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM}>
                        {EXECUTION_MODE_TO_LABEL_MAP[mode]}
                      </Text>
                      <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                        {EXECUTION_MODE_TO_DETAILS_MAP[mode]}
                      </Text>
                    </Flex>
                  ))}
                </Flex>
              }
            />
          </Flex>
          <Flex alignItems={AlignItems.STRETCH} gap={12} fillWidth>
            {PIPELINE_EXECUTION_MODES.map((mode) => (
              <CreatePipelineModalConnectionsExecutionModeOption
                key={mode}
                mode={mode}
                isSelected={executionMode === mode}
                onSelect={() => setExecutionMode(mode)}
              />
            ))}
          </Flex>
        </Flex>
      </Widget>
    </Box>
  );
};

export default CreatePipelineModalConnectionsExecutionMode;
