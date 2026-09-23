import type { Resource, ValidationError } from "@/gen/ingestion/v1/connectors_pb";
import type { TransformExpressionType } from "@/gen/ingestion/v1/transformations_pb";

import { getTransformOutputName } from "@/pages/pipelines/components/transform/grammar/serialize";
import type {
  TransformChain,
  TransformChainCall,
  TransformComputeOutput,
} from "@/pages/pipelines/components/transform/types";

const STEP_PATH_PATTERN = /^resources\["(?:[^"\\]|\\.)*"\]\.steps\[(\d+)\]/;

export const getTransformStepPath = (resource: Resource["name"], index: number): string =>
  `resources[${JSON.stringify(resource)}].steps[${index}]`;

export const getTransformOutputPath = (
  output: TransformComputeOutput,
  isMatchingRows: boolean,
): string => `compute[${JSON.stringify(getTransformOutputName(output, isMatchingRows))}]`;

export const getTransformRenamePath = (from: string): string => `rename[${JSON.stringify(from)}]`;

export const getTransformDropPath = (index: number): string => `drop[${index}]`;

export const getTransformInputPath = (callPath: string, call: TransformChainCall): string =>
  call.fn === "" ? callPath : `${callPath}.${call.fn}[0]`;

export const getTransformArgumentPath = (
  callPath: string,
  call: TransformChainCall,
  position: number,
): string => `${callPath}.${call.fn}[${position + 1}]`;

export const getTransformChainPaths = (
  basePath: string,
  chain: TransformChain,
): { root: string; calls: string[] } => {
  const calls: string[] = new Array(chain.calls.length);
  let path = basePath;
  for (let index = chain.calls.length - 1; index >= 0; index--) {
    calls[index] = path;
    path = getTransformInputPath(path, chain.calls[index]);
  }
  return { root: path, calls };
};

const getStepIndex = (field: string): number | undefined => {
  const match = STEP_PATH_PATTERN.exec(field);
  return match ? Number(match[1]) : undefined;
};

const isUnderPath = (field: string, path: string): boolean =>
  field === path || field.startsWith(`${path}.`);

export const groupTransformIssuesByStep = (issues: ValidationError[]): Map<number, string[]> => {
  const byStep = new Map<number, string[]>();
  for (const issue of issues) {
    const index = getStepIndex(issue.field);
    if (index === undefined) continue;
    byStep.set(index, [...(byStep.get(index) ?? []), issue.message]);
  }
  return byStep;
};

export const getTransformStepIssues = (
  issues: ValidationError[],
  stepPath: string,
): Map<string, string[]> => {
  const own = new Map<string, string[]>();
  for (const issue of issues) {
    if (!isUnderPath(issue.field, stepPath)) continue;
    const path = issue.field.slice(stepPath.length).replace(/^\./, "");
    own.set(path, [...(own.get(path) ?? []), issue.message]);
  }
  return own;
};

export const mapTransformExpressionTypes = (
  types: TransformExpressionType[],
  stepPath: string,
): Map<string, string> =>
  new Map(
    types
      .filter((type) => isUnderPath(type.path, stepPath))
      .map((type) => [type.path.slice(stepPath.length).replace(/^\./, ""), type.logicalType]),
  );
