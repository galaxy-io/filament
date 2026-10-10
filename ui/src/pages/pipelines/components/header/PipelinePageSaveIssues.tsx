import type { FC } from "react";

import { styled } from "@linaria/react";
import pluralize from "pluralize";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import { FOCUS_RING, HAIRLINE_WIDTH, INTERACTIVE_RESET } from "@galaxy-io/dls/styles/mixins";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import type { PipelineCanvasValidationIssue } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasValidation";
import {
  PIPELINE_PAGE_MAX_VISIBLE_SAVE_ISSUES,
  PIPELINE_PAGE_SAVE_ISSUE_KIND_TO_ICON_MAP,
  PIPELINE_PAGE_SAVE_ISSUE_KIND_TO_LABEL_FIELD_MAP,
} from "@/pages/pipelines/components/header/constants";

const PipelinePageSaveIssueRow = styled.button`
  ${INTERACTIVE_RESET}
  ${FOCUS_RING}
  width: 100%;

  padding: ${t.space[8]};

  display: flex;
  align-items: center;

  border: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
  border-radius: ${t.radius.lg};

  background-color: ${t.color.background.primary};
  transition: background-color ${t.duration.fast};

  &:hover:not(:disabled) {
    background-color: ${t.color.background.hovered};
  }

  &:disabled {
    cursor: default;
  }
`;

interface PipelinePageSaveIssuesProps {
  issues: PipelineCanvasValidationIssue[];
  onSelectResource: (edgeId: string) => void;
}

const PipelinePageSaveIssues: FC<PipelinePageSaveIssuesProps> = ({ issues, onSelectResource }) => {
  const hidden = issues.length - PIPELINE_PAGE_MAX_VISIBLE_SAVE_ISSUES;
  return (
    <Flex
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.CENTER}
      gap={4}
      padding={8}
      minWidth={240}
      maxWidth={320}
    >
      {issues.slice(0, PIPELINE_PAGE_MAX_VISIBLE_SAVE_ISSUES).map(({ edgeId, ...issue }) => (
        <PipelinePageSaveIssueRow
          key={`${edgeId ?? ""}|${issue.resource ?? ""}|${issue.message}`}
          type="button"
          disabled={edgeId === undefined}
          onClick={edgeId === undefined ? undefined : () => onSelectResource(edgeId)}
        >
          <Flex
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.SPACE_BETWEEN}
            gap={16}
            fillWidth
          >
            <Flex gap={8} alignItems={AlignItems.CENTER} overflow="hidden">
              <Icon
                component={PIPELINE_PAGE_SAVE_ISSUE_KIND_TO_ICON_MAP[issue.kind]}
                size={16}
                variant={IconVariant.SECONDARY}
              />
              <FlexItem shrink={1} minWidth={0} overflow="hidden">
                <Text size={TextSize.BODY_MD} lineClamp={1}>
                  {issue[PIPELINE_PAGE_SAVE_ISSUE_KIND_TO_LABEL_FIELD_MAP[issue.kind]]}
                </Text>
              </FlexItem>
            </Flex>
            {issue.invalidSteps !== undefined && (
              <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM} variant={TextVariant.ERROR}>
                {issue.invalidSteps} {pluralize("issue", issue.invalidSteps)}
              </Text>
            )}
          </Flex>
        </PipelinePageSaveIssueRow>
      ))}
      {hidden > 0 && (
        <Flex alignItems={AlignItems.START} padding={[4, 0]}>
          <Text variant={TextVariant.TERTIARY} size={TextSize.BODY_SM}>
            +{hidden} more {pluralize("issue", hidden)}
          </Text>
        </Flex>
      )}
    </Flex>
  );
};

export default PipelinePageSaveIssues;
