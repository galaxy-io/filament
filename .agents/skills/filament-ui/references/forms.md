# Forms and providers

Three kinds of form exist in Filament. Pick the lightest one that fits.

| Form | State | Example |
|---|---|---|
| A few fields saved by one button | `useState<XState>` in the component | `pages/pipelines/settings/PipelineSettingsPageGeneral.tsx`, `pages/settings/components/team/SettingsTeamInviteModal.tsx`, `pages/pipelines/components/notifier/PipelineNotifierForm.tsx` |
| Fields generated from a connector's schema | `ConnectionFormProvider` reducer + `components/fields/*` | `pages/connections/components/form/` |
| A multi-step wizard or a graph editor with cross-field rules | a provider folder with actions, reducer and derived state | `pages/pipelines/components/create/`, `pages/pipelines/canvas/providers/canvas/` |

## Local state form

```tsx
interface PipelineSettingsPageGeneralState {
  name: Pipeline["name"];
  description: Pipeline["description"];
}

const DEFAULT_STATE: PipelineSettingsPageGeneralState = {
  name: "",
  description: "",
};

const PipelineSettingsPageGeneral: FC = () => {
  const { toast } = useToast();
  const { id } = usePipelineParams();

  const { data } = useSuspenseGetPipelineQuery({
    input: createGetPipelineInput(id),
  });
  const pipeline = data.pipeline;

  const { mutate: updatePipeline, isPending: isSaving } = useUpdatePipelineMutation();

  const createInitialState = (): PipelineSettingsPageGeneralState => ({
    ...DEFAULT_STATE,
    name: stripDeletedName(pipeline?.name ?? ""),
    description: pipeline?.description ?? "",
  });

  const [state, setState] = useState<PipelineSettingsPageGeneralState>(createInitialState);

  if (!pipeline) return null;

  const pipelineName = stripDeletedName(pipeline.name);

  const handleNameChange = (name: string) => {
    setState((prev) => ({ ...prev, name }));
  };

  const hasChanges =
    state.name.trim() !== pipelineName || state.description.trim() !== pipeline.description;

  const canSave = hasChanges && state.name.trim().length > 0;

  const handleCancel = () => {
    setState(createInitialState());
  };
  …
};
```

- One `interface XState` above the component, a module `DEFAULT_STATE`, and `useState<XState>`.
- Every setter is a `handleX` that spreads `prev`. A multi-field patch is `(partial: Partial<XState>) => setState((prev) => ({ ...prev, ...partial }))` (`PipelineNotifierForm`).
- Seed from server data with a lazy `createInitialState`. It runs once, so a background refetch never clobbers what the user typed. Cancel calls it again.
- Reset from outside with `key`, never by syncing state to props in an effect.
- `hasChanges`, `canSave` and every error are computed at render.
- Trim before comparing and before sending.
- Inputs get `label` directly (or sit in a DLS `Field` when they need a description), `fillWidth`, and `onChange` receives the value.
- Write the fields out literally. A field-config array mapped to inputs is not a simplification for a handful of fields.

### Proto messages are state

Field types borrow from the proto with indexed access (`Pipeline["name"]`, `InviteMemberRequest["email"]`, `PipelineScheduleConfig["cron"]`). Never hand-roll a parallel type for a message. A message can be the state itself, with `Partial<X>` as the patch. Hand-rolled state earns its place only when it decomposes or extends the message, the way `PipelineScheduleState` adds `isEnabled` beside the config's `cron` and `timezone`.

## Validation timing

There is no blur validation. Three timings exist, and new forms pick one.

| Timing | Mechanism | Where |
|---|---|---|
| Show errors only after the user tries to proceed, hide them again on edit | a `shouldShowErrors` flag set by Validate, cleared by every edit in the reducer | the connection form, whose field errors come from `ValidateConfig` |
| Show an error once the field has been touched | an `isNameTouched` flag flipped by the first edit | the pipeline name in the create wizard |
| Validate continuously, but stay quiet while empty | `state.url !== "" && !isPipelineNotifierUrlValid(state.url) ? "Use an absolute http or https URL" : undefined` | notifier URL, JSON editors, worker configuration |

- Validators are pure functions in the folder's `utils.ts` or `validation.ts`. Predicates are `isXValid(value)` (`isNameValid`, `isPipelineScheduleCronValid`, `isPipelineNotifierValid`). Message getters are `getXError(value, …)` and return `string | null` (`getNameError(name, shouldShowErrors)`). Callers pass `error={nameError ?? undefined}`, because `error` is a `string | undefined` prop.
- The primary button is disabled by the matching predicate (`isDisabled={!canSave}`, `isNextDisabled`). A required field that is still empty blocks the button without an inline error.
- A component without an `error` slot (a `CodeEditor`) renders `<Text size={TextSize.BODY_SM} variant={TextVariant.ERROR}>` beneath itself.
- Messages are short and imperative with no period. "Name is required", "Enter valid JSON", "Use a Slack incoming webhook URL".
- Server validation errors come back as `ValidationError[]` with a field path. `createRequiredFieldsValidationErrorMap` turns them into a `Map<field, message>` that a `getFieldError(path)` callback reads. The first one is also toasted.

## Schema-driven fields

Connectors describe their configuration as `ConfigField[]`. `components/fields/Field.tsx` renders one field by type through an exhaustive map and recurses into objects.

```tsx
const FIELD_TYPE_TO_FIELD_COMPONENT_MAP: Record<FieldType, FieldComponent> = {
  [FieldType.UNSPECIFIED]: FieldString,
  [FieldType.STRING]: FieldString,
  [FieldType.DURATION]: FieldString,
  [FieldType.INT]: FieldInt,
  [FieldType.BOOL]: FieldBoolean,
  [FieldType.SECRET]: FieldSecret,
  [FieldType.ENUM]: FieldEnum,
  [FieldType.OBJECT]: FieldObject,
  [FieldType.LIST]: FieldList,
};
```

- `Field` takes `field`, `value`, `onChange`, `variant`, `isDisabled`, `path`, `getError(path)` and `storedSecretRefs`. It computes the label and the per-path error and secret flag, then renders the leaf component.
- Every leaf takes `FieldComponentProps` from `fields/types.ts` (`field`, `value: JsonValue`, `onChange(JsonValue)`, `variant`, `error`, `isDisabled`, `label`, `hasStoredSecret`).
- Each `FieldX` is one DLS input with its own label. `label` comes from `formatFieldName(field.name)`, `labelTooltip={field.help || undefined}`, `isRequired={field.required}`, `error`, `fillWidth`. `FieldBoolean` and `FieldObject` wrap a DLS `Field` because the checkbox and the code editor have no label of their own. An object with sub-fields is a collapsible `Widget` in `Field.tsx` with its error line under the children, and the child paths are `${path}.${child.name}`.
- Placeholders are `Enter ${label}...` for text and `Select ${label}...` for selects.
- **Secrets.** `hasStoredSecret` is true when the path is in `storedSecretRefs`, and `FieldSecret` swaps its placeholder to "Leave blank to keep current value". A multi-line paste is written whole, because a single-line input would strip it.
- **Lists** are a `TagInput` ("Press Enter or comma to add a value"), or a `MultiSelectInput` with `selectAllLabel` when the schema enumerates values.
- **Objects without sub-fields** are a JSON `CodeEditor` that keeps local text and a parse error and only emits valid objects.
- **Visibility and defaults.** `isFieldVisible(field, siblings)` honours `visibleWhen`, `getFieldDefaults` reads proto defaults, and `updateConfigField` (used by `PipelineNodeConfigFields`) resets dependents whose visibility flips. The connection form writes one key at a time through `setConfigField` and does not reset dependents. `getConnectionScopedFields` and `getPipelineScopedFields` split a schema by scope so the connection form and the pipeline node config each render their half.
- One `InputVariant` flows to every input kind through the `INPUT_VARIANT_TO_*_INPUT_VARIANT_MAP` records in `fields/constants.ts`.

### Connection form phases

`ConnectionFormPhase` is `IDLE`, `VALIDATING`, `VALIDATED`, `SUBMITTING` and `ERROR`. The footer in `ConnectionForm.tsx` is a `match` on it.

```tsx
match(state.phase)
  .with(ConnectionFormPhase.IDLE, ConnectionFormPhase.ERROR, () => (
    <Button label="Validate" icon={ArrowRightIcon} onClick={handleTestConnection} isDisabled={isDisabled} isIconTrailing />
  ))
  .with(ConnectionFormPhase.VALIDATING, () => <Button label="Validate" isLoading />)
  .with(ConnectionFormPhase.VALIDATED, () => (
    <Flex alignItems={AlignItems.CENTER} gap={16}>
      <Beacon variant={BeaconVariant.SUCCESS} label="Connected" />
      <Button label={submitLabel} icon={CheckIcon} onClick={onSubmit} />
    </Flex>
  ))
  .with(ConnectionFormPhase.SUBMITTING, () => <Button label={submitLabel} isLoading />)
  .exhaustive();
```

- Validate sets `shouldShowErrors`, checks the name, moves to `VALIDATING` and calls `useValidateConfigMutation`. A valid response moves to `VALIDATED`, an invalid one stores the errors and moves to `ERROR`.
- Every edit runs `resetPhaseOnEdit` in the reducer, so `VALIDATED` or `ERROR` drops back to `IDLE` and a changed config must be validated again before it can be saved.
- The create and edit modals own the submit. They set `SUBMITTING`, call the mutation, and set `ERROR` on failure.

## The provider contract

A surface whose state has invariants across fields gets a provider folder. The model is `pages/pipelines/canvas/providers/canvas/`.

```
providers/canvas/
├── PipelineCanvasProvider.tsx   contexts, accessor hooks, the provider
├── actions.ts                   action enum, one interface per action, the union
├── reducer.ts                   one private function per action + a delegating switch
├── types.ts                     state and enums
└── utils.ts                     createInitialPipelineCanvasState
```

**`actions.ts`**

```ts
export enum PipelineCanvasActionType {
  LOAD_GRAPH = "LOAD_GRAPH",
  ADD_NODE = "ADD_NODE",
  REMOVE_NODE = "REMOVE_NODE",
  …
}

export interface AddNodeAction {
  type: PipelineCanvasActionType.ADD_NODE;
  payload: CanvasNode;
}

export type PipelineCanvasAction = LoadGraphAction | AddNodeAction | RemoveNodeAction | …;
```

**`reducer.ts`**

```ts
function removeNode(state: PipelineCanvasState, action: RemoveNodeAction): PipelineCanvasState {
  return {
    ...state,
    nodes: state.nodes.filter((node) => node.id !== action.payload),
    edges: state.edges.filter(
      (edge) => edge.source !== action.payload && edge.target !== action.payload,
    ),
  };
}

const pipelineCanvasReducer = (
  state: PipelineCanvasState,
  action: PipelineCanvasAction,
): PipelineCanvasState => {
  switch (action.type) {
    case PipelineCanvasActionType.REMOVE_NODE:
      return removeNode(state, action);
    …
  }
};

export default pipelineCanvasReducer;
```

**`PipelineCanvasProvider.tsx`**

```tsx
const PipelineCanvasStateContext = createContext<PipelineCanvasState | null>(null);
PipelineCanvasStateContext.displayName = "PipelineCanvasStateContext";

const PipelineCanvasDispatchContext = createContext<Dispatch<PipelineCanvasAction> | null>(null);
PipelineCanvasDispatchContext.displayName = "PipelineCanvasDispatchContext";

export const usePipelineCanvasState = () => {
  const state = useContext(PipelineCanvasStateContext);
  if (!state) {
    throw new Error("usePipelineCanvasState must be used within PipelineCanvasProvider");
  }
  return state;
};

const usePipelineCanvasDispatch = () => { … };

export const usePipelineCanvasActions = () => {
  const dispatch = usePipelineCanvasDispatch();

  return useMemo(
    () => ({
      addNode: (node: CanvasNode) =>
        dispatch({ type: PipelineCanvasActionType.ADD_NODE, payload: node }),
      removeNode: (nodeId: PipelineNode["id"]) =>
        dispatch({ type: PipelineCanvasActionType.REMOVE_NODE, payload: nodeId }),
      …
    }),
    [dispatch],
  );
};

const PipelineCanvasProvider: FC<PropsWithChildren<PipelineCanvasProviderProps>> = ({
  graph,
  graphKey,
  isReadOnly = false,
  children,
}) => {
  const [state, dispatch] = useReducer(pipelineCanvasReducer, graph, createInitialPipelineCanvasState);

  const [previousGraphKey, setPreviousGraphKey] = useState(graphKey);
  if (previousGraphKey !== graphKey) {
    setPreviousGraphKey(graphKey);
    dispatch({ type: PipelineCanvasActionType.LOAD_GRAPH, payload: graph });
  }
  …
};
```

Rules.

- **Two contexts**, state and dispatch, dispatch outermost. Both are `createContext<T | null>(null)` with a `displayName`, and the accessor hooks throw outside the provider.
- **Dispatch stays private.** The folder exports `useXState` and `useXActions`, the latter a memoised object of semantic functions. Consumers call `removeNode(id)` and never build `{ type, payload }` or import the action enum. All three providers follow this. `useCreatePipelineModalState` and `useCreatePipelineModalActions` for the wizard, `useConnectionFormState` and `useConnectionFormActions` for the connection form.
- **The reducer is the sole writer and the sole enforcer of invariants.** Selecting a new source clears the resource selection in `selectSource`, and an edit resets the phase in `resetPhaseOnEdit`, not in a component. Cases are private `function` helpers behind a `switch` with no `default`, so the union keeps it exhaustive.
- **Initial state.** A lazy `useReducer` initialiser seeds it. The canvas has `createInitialPipelineCanvasState(graph)` in `utils.ts`. The connection form has `createInitialConnectionFormState(initialState)` in `form/utils.ts`, and the wizard has `createInitialCreatePipelineModalState()` in `create/utils.ts`.
- **Re-seed on an identity change during render**, never in an effect. The canvas compares `previousGraphKey` to `graphKey` and dispatches `LOAD_GRAPH`. Everything else resets by remounting with `key`.
- **Derived state is a `useMemo` in the provider**, merged into the context value. The wizard's `isNextDisabled` is a `match(state.step)` over `CreatePipelineModalStep`, and `hints` lists the concrete blockers (server messages first, prefixed `[Sink: name]` when there are several sinks, then `CREATE_PIPELINE_MODAL_STEP_TO_HINT_MAP[state.step]`).
- **Server-backed derived data lives in a hook beside the provider.** `create/hooks/useCreatePipelineResources.ts` takes the reducer state and `isOpen`, runs discover, columns and validate with `PROBE_QUERY_OPTIONS`, and uses `placeholderData: keepPreviousData` where a flash would hurt. Pure row builders go in `rows.ts`.
- **Serialisation is pure.** `serialize.ts` holds `mapCreatePipelineStateToRequest`, `mapCreatePipelineStateToVersionRequest` and `mapCreatePipelineNotifierToRequest`, each built with `create(…)`.
- Each step reads the context hooks itself instead of taking props. UI state that is not domain state (a search box, an "is creating" toggle) is local `useState` with its own `DEFAULT_STATE`.

## Overlays

`module/FilamentLayout.tsx` keeps every URL-driven overlay mounted and drives it with `isOpen`. It passes the URL values in as props, so overlay content never reads search params.

```tsx
<ConnectionDrawer connectionId={connectionId} isOpen={!!connectionId} onClose={handleCloseDrawer} />
<CreateConnectionModal
  isOpen={flow === Flow.CREATE_CONNECTION}
  connector={connector}
  connectorKind={connectorKind}
  connectorSearch={connectorSearch}
  onClose={handleCloseFlow}
/>
<EditConnectionModal
  isOpen={flow === Flow.EDIT_CONNECTION && !!connectionId}
  connectionId={connectionId}
  onClose={handleCloseFlow}
/>
<CreatePipelineModal isOpen={flow === Flow.CREATE_PIPELINE} onClose={handleCloseFlow} />
```

- DLS `Drawer` and `Modal` freeze their children through the exit animation, so there is no retain hook.
- `useOverlaySession(isOpen)` in `src/hooks/useOverlaySession.ts` returns a key that is `0` before the first open and increments on every open. A modal renders nothing while it is `0`, then passes it as `key` to its provider so each open starts clean.

```tsx
const CreatePipelineModal: FC<CreatePipelineModalProps> = ({ isOpen, onClose }) => {
  const session = useOverlaySession(isOpen);

  if (!session) {
    return null;
  }

  return (
    <CreatePipelineModalProvider key={session} isOpen={isOpen}>
      <CreatePipelineModalContent isOpen={isOpen} onClose={onClose} />
    </CreatePipelineModalProvider>
  );
};
```

- The record arrives as a prop. `EditConnectionModal` fetches the connection by id with a plain query gated on `!!session && !!connectionId`, shows `PendingLayout` or `ErrorLayout` inside `ConnectionFormWrapper`, then mounts `ConnectionFormProvider key={session}` seeded from it and hands `connection` to `EditConnectionModalContent`.
- A key can carry more than the session. `CreateConnectionModal` keys the configure step on `` `${session}:${connectorKind}:${connector}` `` so switching connectors resets the form.
- A closed overlay fetches nothing. `CreatePipelineModalProvider` passes `isOpen` to `useCreatePipelineResources`, which folds it into every `enabled`.
- `ConnectionDrawer` needs no session key because it holds no form state. It fetches by `connectionId` with `enabled: !!connectionId, retry: false`.
- Overlays close with `replace: true`.
- Page-local dialogs follow the same contract. `SettingsTeamInviteModal` and `SettingsServiceAccountsCreateDialog` stay mounted with `isOpen`, and their page keys them with `useOverlaySession(isOpen)` so each open starts fresh.
- An overlay that swaps between two `Modal`s on a URL record (the create-connection selector and configure steps) holds that record through the exit with `useOverlayRecord(isOpen, value)` from `hooks/useOverlayRecord.ts`. It takes primitives only, keeps the last value while closed, and follows the URL while open. A single `Modal` needs nothing, because the DLS keeps its last children through the exit.

## Effects

No `.tsx` file calls `useEffect` or `useLayoutEffect`. An effect lives in a named `hooks/useX.ts` that syncs one external system and is exported as `export const useX`.

```ts
export const usePipelineRunNow = (isTicking: boolean) => {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!isTicking) return;
    const interval = setInterval(() => setNow(Date.now()), PIPELINE_RUN_DURATION_TICK_MS);
    return () => clearInterval(interval);
  }, [isTicking]);

  return now;
};
```

The others are `usePipelineCanvasSelectionReveal` (the xyflow viewport), `usePipelineCanvasNodeIslandMeasurements`, `usePipelineCanvasRoutesVirtualizer` and the host's `useAuthLayoutAsideField`. State that only mirrors props or server data is never an effect. Seed it lazily, re-seed during render, or remount with `key`.

## Wizard composition

```tsx
const CreatePipelineModalContent: FC<CreatePipelineModalProps> = ({ isOpen, onClose }) => {
  const { step } = useCreatePipelineModalState();

  const renderBody = () => {
    const body = match(step)
      .with(CreatePipelineModalStep.CONNECTIONS, () => <CreatePipelineModalConnections />)
      .with(CreatePipelineModalStep.RESOURCES, () => <CreatePipelineModalResources />)
      .with(CreatePipelineModalStep.DELIVERY, () => <CreatePipelineModalDelivery />)
      .with(CreatePipelineModalStep.DETAILS, () => <CreatePipelineModalDetails />)
      .exhaustive();
    …
  };

  return (
    <Modal
      isOpen={isOpen}
      size={ModalSize.X_LARGE}
      header="Create a new pipeline"
      footer={<CreatePipelineModalFooter />}
      onOpenChange={(isOpen) => {
        if (!isOpen) onClose();
      }}
    >
      …
    </Modal>
  );
};
```

- The default export only mounts the provider. An inner `*Content` component reads the context.
- Every step-keyed fact is an exhaustive `Record<CreatePipelineModalStep, …>` in `constants.ts` (`CREATE_PIPELINE_MODAL_STEP_TO_TITLE_MAP`, `…_TO_DESCRIPTION_MAP`, `…_TO_HINT_MAP`, `…_TO_IS_PADDED_MAP`), plus `CREATE_PIPELINE_MODAL_STEP_ORDER`.
- Steps live in `steps/CreatePipelineModal<Step>.tsx`, their parts in `steps/components/CreatePipelineModal<Step><Part>.tsx`.
- A step with sections stacks DLS `Fieldset`s in a column with `gap={24}`.
- The footer renders Back `SECONDARY` on the left, then an "Invalid" `Chip` whose tooltip lists the hints, then Next (`ArrowRightIcon`, `isIconTrailing`) or "Create pipeline" (`PlusIcon`), which reads "Creating..." while submitting.
