import type { JsonValue } from "@bufbuild/protobuf";

import type { Resource } from "@/gen/ingestion/v1/connectors_pb";
import type { TransformFunction } from "@/gen/ingestion/v1/transformations_pb";

import { isJsonObject } from "@/components/fields/utils";
import {
  type TransformDefinition,
  type TransformExpr,
  TransformExprKind,
  TransformLiteralKind,
  type TransformStep,
  TransformStepKind,
} from "@/components/transform/types";

const literal = (literalKind: TransformLiteralKind, value: string): TransformExpr => ({
  kind: TransformExprKind.LITERAL,
  literalKind,
  value,
});

export const parseTransformExpr = (
  json: JsonValue,
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
): TransformExpr | null => {
  if (typeof json === "string") return literal(TransformLiteralKind.STRING, json);
  if (typeof json === "number") return literal(TransformLiteralKind.NUMBER, String(json));
  if (typeof json === "boolean") return literal(TransformLiteralKind.BOOLEAN, String(json));
  if (!isJsonObject(json)) return null;
  const entries = Object.entries(json);
  if (entries.length !== 1) return null;
  const [key, value] = entries[0];
  if (key === "col") {
    return typeof value === "string" && value !== ""
      ? { kind: TransformExprKind.COLUMN, name: value }
      : null;
  }
  if (!functionsByName.has(key)) return null;
  const args = (Array.isArray(value) ? value : [value]).map((arg) =>
    parseTransformExpr(arg, functionsByName),
  );
  if (args.length === 0 || !args.every((arg) => arg !== null)) return null;
  return { kind: TransformExprKind.CALL, fn: key, args };
};

export const parseTransformStep = (
  json: JsonValue,
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
): TransformStep => {
  const id = crypto.randomUUID();
  const raw: TransformStep = { id, kind: TransformStepKind.RAW, json };
  if (!isJsonObject(json)) return raw;
  const keys = Object.keys(json);

  if (keys.length === 1 && keys[0] === "rename" && isJsonObject(json.rename)) {
    const pairs = Object.entries(json.rename).map(([from, to]) =>
      from !== "" && typeof to === "string" && to !== "" ? { from, to } : null,
    );
    if (pairs.length === 0 || !pairs.every((pair) => pair !== null)) return raw;
    return { id, kind: TransformStepKind.RENAME, pairs };
  }

  if (keys.length === 1 && keys[0] === "drop") {
    const names = json.drop;
    if (
      !Array.isArray(names) ||
      names.length === 0 ||
      !names.every((name): name is string => typeof name === "string" && name !== "")
    ) {
      return raw;
    }
    return { id, kind: TransformStepKind.DROP, names };
  }

  if (
    keys.includes("compute") &&
    keys.every((key) => key === "compute" || key === "where") &&
    isJsonObject(json.compute)
  ) {
    const outputs = Object.entries(json.compute).map(([name, value]) => {
      const expr = name === "" ? null : parseTransformExpr(value, functionsByName);
      return expr ? { name, expr } : null;
    });
    if (outputs.length === 0 || !outputs.every((output) => output !== null)) return raw;
    const where = json.where === undefined ? null : parseTransformExpr(json.where, functionsByName);
    if (json.where !== undefined && where === null) return raw;
    return { id, kind: TransformStepKind.COMPUTE, outputs, where };
  }
  return raw;
};

const getResourceSteps = (
  definition: TransformDefinition | undefined,
  resource: Resource["name"],
): JsonValue[] => {
  const resources = definition?.resources;
  if (resources === undefined || !isJsonObject(resources)) return [];
  const entry = resources[resource];
  if (entry === undefined || !isJsonObject(entry) || !Array.isArray(entry.steps)) return [];
  return entry.steps;
};

export const parseTransformDefinition = (
  definition: TransformDefinition | undefined,
  resources: Resource["name"][],
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
): Map<Resource["name"], TransformStep[]> =>
  new Map(
    resources.map((resource) => [
      resource,
      getResourceSteps(definition, resource).map((step) =>
        parseTransformStep(step, functionsByName),
      ),
    ]),
  );
