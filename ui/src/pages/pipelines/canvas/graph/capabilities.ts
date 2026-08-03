import { IngestionType, Operation, ReplicationMode, WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

export const PIPELINE_CANVAS_INGESTION_TYPES: IngestionType[] = [
  IngestionType.SNAPSHOT_REPLACE,
  IngestionType.SNAPSHOT_UPSERT,
  IngestionType.APPEND,
  IngestionType.UPSERT,
  IngestionType.DELETE,
  IngestionType.CDC,
];

interface PipelineCanvasSourcePolicy {
  mode: ReplicationMode;
  emitsOps: Operation[];
  ordered: boolean;
}

interface PipelineCanvasWriteCapability {
  mode: WriteMode;
  requiresPk: boolean;
  requiresOrder: boolean;
  acceptsOps: Operation[];
}

const INGESTION_TYPE_TO_SOURCE_POLICY_MAP: Record<IngestionType, PipelineCanvasSourcePolicy> = {
  [IngestionType.UNSPECIFIED]: {
    mode: ReplicationMode.FULL,
    emitsOps: [Operation.INSERT],
    ordered: false,
  },
  [IngestionType.SNAPSHOT_REPLACE]: {
    mode: ReplicationMode.FULL,
    emitsOps: [Operation.INSERT],
    ordered: false,
  },
  [IngestionType.SNAPSHOT_UPSERT]: {
    mode: ReplicationMode.FULL,
    emitsOps: [Operation.INSERT],
    ordered: false,
  },
  [IngestionType.APPEND]: {
    mode: ReplicationMode.FULL,
    emitsOps: [Operation.INSERT],
    ordered: false,
  },
  [IngestionType.UPSERT]: {
    mode: ReplicationMode.INCREMENTAL,
    emitsOps: [Operation.INSERT, Operation.UPDATE],
    ordered: false,
  },
  [IngestionType.DELETE]: {
    mode: ReplicationMode.INCREMENTAL,
    emitsOps: [Operation.INSERT, Operation.UPDATE],
    ordered: false,
  },
  [IngestionType.CDC]: {
    mode: ReplicationMode.CDC,
    emitsOps: [Operation.INSERT, Operation.UPDATE, Operation.DELETE],
    ordered: true,
  },
};

const INGESTION_TYPE_TO_WRITE_CAPABILITY_MAP: Record<IngestionType, PipelineCanvasWriteCapability> =
  {
    [IngestionType.UNSPECIFIED]: {
      mode: WriteMode.REPLACE,
      requiresPk: false,
      requiresOrder: false,
      acceptsOps: [Operation.INSERT],
    },
    [IngestionType.SNAPSHOT_REPLACE]: {
      mode: WriteMode.REPLACE,
      requiresPk: false,
      requiresOrder: false,
      acceptsOps: [Operation.INSERT],
    },
    [IngestionType.SNAPSHOT_UPSERT]: {
      mode: WriteMode.UPSERT,
      requiresPk: true,
      requiresOrder: false,
      acceptsOps: [Operation.INSERT],
    },
    [IngestionType.APPEND]: {
      mode: WriteMode.APPEND,
      requiresPk: false,
      requiresOrder: false,
      acceptsOps: [Operation.INSERT],
    },
    [IngestionType.UPSERT]: {
      mode: WriteMode.UPSERT,
      requiresPk: true,
      requiresOrder: false,
      acceptsOps: [Operation.INSERT, Operation.UPDATE],
    },
    [IngestionType.DELETE]: {
      mode: WriteMode.DELETE,
      requiresPk: true,
      requiresOrder: false,
      acceptsOps: [Operation.DELETE],
    },
    [IngestionType.CDC]: {
      mode: WriteMode.MERGE,
      requiresPk: true,
      requiresOrder: true,
      acceptsOps: [Operation.INSERT, Operation.UPDATE, Operation.DELETE],
    },
  };

const REPLICATION_MODE_TO_NOUN_MAP: Record<ReplicationMode, string> = {
  [ReplicationMode.UNSPECIFIED]: "full",
  [ReplicationMode.FULL]: "full",
  [ReplicationMode.INCREMENTAL]: "incremental",
  [ReplicationMode.CDC]: "change data capture",
};

const WRITE_MODE_TO_VERB_MAP: Record<WriteMode, string> = {
  [WriteMode.UNSPECIFIED]: "replace",
  [WriteMode.APPEND]: "append",
  [WriteMode.REPLACE]: "replace",
  [WriteMode.UPSERT]: "upsert",
  [WriteMode.DELETE]: "delete",
  [WriteMode.MERGE]: "merge",
};

const acceptsOperations = (have: Operation[], want: Operation[]): boolean => {
  if (want.length === 0) return true;
  if (have.length === 0) return false;
  return want.every((op) => have.includes(op));
};

const isSourceSupported = (spec: ConnectorSpec, policy: PipelineCanvasSourcePolicy): boolean =>
  spec.capabilities?.sourcePolicies.some(
    (candidate) =>
      candidate.mode === policy.mode &&
      acceptsOperations(candidate.emitsOps, policy.emitsOps) &&
      (!policy.ordered || candidate.ordered),
  ) || spec.modes.includes(policy.mode);

const isSinkSupported = (
  spec: ConnectorSpec,
  capability: PipelineCanvasWriteCapability,
): boolean => {
  const matchesPolicy = spec.capabilities?.writePolicies.some(
    (candidate) =>
      candidate.mode === capability.mode &&
      (!capability.requiresPk || candidate.requiresPk) &&
      (!capability.requiresOrder || candidate.requiresOrder) &&
      acceptsOperations(candidate.acceptsOps, capability.acceptsOps),
  );
  if (matchesPolicy) return true;
  if (capability.mode === WriteMode.APPEND || capability.mode === WriteMode.REPLACE) return true;
  if (capability.mode === WriteMode.UPSERT) return Boolean(spec.capabilities?.upsertable);
  return false;
};

export interface PipelineCanvasIngestionTypeSupport {
  isSupported: boolean;
  reason?: string;
}

export const getIngestionTypeSupport = (
  sourceSpec: ConnectorSpec | undefined,
  sinkSpec: ConnectorSpec | undefined,
): Record<IngestionType, PipelineCanvasIngestionTypeSupport> => {
  const support = {} as Record<IngestionType, PipelineCanvasIngestionTypeSupport>;

  for (const type of PIPELINE_CANVAS_INGESTION_TYPES) {
    const sourcePolicy = INGESTION_TYPE_TO_SOURCE_POLICY_MAP[type];
    const writeCapability = INGESTION_TYPE_TO_WRITE_CAPABILITY_MAP[type];

    if (sourceSpec && !isSourceSupported(sourceSpec, sourcePolicy)) {
      support[type] = {
        isSupported: false,
        reason: `${sourceSpec.displayName || sourceSpec.name} cannot read in ${REPLICATION_MODE_TO_NOUN_MAP[sourcePolicy.mode]} mode`,
      };
      continue;
    }

    if (sinkSpec && !isSinkSupported(sinkSpec, writeCapability)) {
      support[type] = {
        isSupported: false,
        reason: `${sinkSpec.displayName || sinkSpec.name} cannot ${WRITE_MODE_TO_VERB_MAP[writeCapability.mode]} rows`,
      };
      continue;
    }

    support[type] = { isSupported: true };
  }

  return support;
};
