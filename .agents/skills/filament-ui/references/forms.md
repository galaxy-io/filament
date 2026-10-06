# Forms and providers

Three kinds of form exist in Filament. Pick the lightest one that fits.

| Form | State | Example |
|---|---|---|
| A few fields saved by one button | `useState<XState>` in the component | `pages/pipelines/settings/PipelineSettingsPageGeneral.tsx`, `SettingsTeamPanelInvite.tsx` |
| Fields generated from a connector's schema | `ConnectionFormProvider` reducer + `components/fields/*` | `pages/connectors/components/form/ConnectionForm.tsx` |
| A multi-step wizard with cross-field rules | a provider folder with reducer, derived state and steps | `pages/pipelines/components/create/` |

## Local state form

```tsx
interface PipelineSettingsPageGeneralState {
  name: Pipeline["name"];
  description: Pipeline["description"];
}

const DEFAULT_STATE: PipelineSettingsPageGeneralState = { name: "", description: "" };

const PipelineSettingsPageGeneral = () => {
  const { toast } = useToast();
  const { id } = useParams({ from: "/_app/pipelines/$id" });
  const { data } = useSuspenseGetPipelineQuery({ input: create(GetPipelineRequestSchema, { id }) });
  const pipeline = data.pipeline;
  const { mutate: updatePipeline, isPending: isSaving } = useUpdatePipelineMutation();

  const createInitialState = (): PipelineSettingsPageGeneralState => ({
    ...DEFAULT_STATE,
    name: stripDeletedName(pipeline?.name ?? ""),
    description: pipeline?.description ?? "",
  });
  const [state, setState] = useState<PipelineSettingsPageGeneralState>(createInitialState);

  if (!pipeline) return null;

  const handleNameChange = (name: string) => setState((prev) => ({ ...prev, name }));
  const hasChanges = state.name.trim() !== stripDeletedName(pipeline.name) || state.description.trim() !== pipeline.description;
  const canSave = hasChanges && state.name.trim().length > 0;
  const handleCancel = () => setState(createInitialState());
  const handleSave = () => updatePipeline(create(UpdatePipelineRequestSchema, { … }), { onSuccess, onError });

  return (
    <Widget isCollapsible header="General" defaultIsOpen>
      <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={12} fillWidth>
        <TextInput value={state.name} onChange={handleNameChange} label="Name" fillWidth />
        <TextAreaInput value={state.description} onChange={handleDescriptionChange} label="Description" placeholder="Optional description" fillWidth />
        <Flex alignItems={AlignItems.START} justifyContent={JustifyContent.END} gap={8} fillWidth>
          <Button label="Cancel" variant={ButtonVariant.SECONDARY} isDisabled={!hasChanges || isSaving} onClick={handleCancel} />
          <Button label="Save" isDisabled={!canSave} isLoading={isSaving} onClick={handleSave} />
        </Flex>
      </Flex>
    </Widget>
  );
};
```

- One state object, a `DEFAULT_STATE`, a `createInitialState` seeded from server data and reused by Cancel. A multi-field patch handler is `(partial: Partial<XState>) => setState((prev) => ({ ...prev, ...partial }))`.
- Field types borrow from the proto (`Pipeline["name"]`, `InviteMemberRequest["email"]`).
- `hasChanges`, `canSave` and every error are computed at render. No effect syncs state to props. A refetch does not clobber what the user typed because the initialiser runs once. Reset from outside with a `key`.
- Inputs get `label` directly (or sit inside a DLS `Field` when they need a description or horizontal layout), `fillWidth`, and `onChange` receives the value.
- Trim before comparing and before sending.
- Write the fields out literally. A field-config array mapped to inputs is not a simplification for a handful of fields.
- Sections that save a message-shaped object can use the generated message type as the state type, `create(XSchema)` as the empty default and `Partial<X>` as the patch, instead of a parallel interface. Hand-rolled state earns its place when it decomposes the message (the schedule state that splits a cron into fields).

## Validation timing

There is no blur validation. Three timings exist, and new forms pick one.

| Timing | Mechanism | Where |
|---|---|---|
| Show errors only after the user tries to proceed, hide them again on edit | a `shouldShowErrors` flag set by Validate or Save, reset to `false` by every edit | the connection form, whose errors come from the server's `ValidateConfig` |
| Show an error once the field has been touched | an `isNameTouched` flag flipped by the first edit | the pipeline name in the create wizard |
| Validate continuously, but stay quiet while empty | `value !== "" && !isValid(value) ? "Use an absolute http or https URL" : undefined` | notifier URL, JSON editors, worker configuration |

- Validators are pure functions in the folder's `validation.ts` or `utils.ts`, named `isXValid(value)` and `getXError(value, …)`. They return `string | null` and callers pass `error={xError ?? undefined}`. `error` is a `string | undefined` prop, never `null` or `""`.
- The primary button is disabled by the matching predicate (`isDisabled={!canSave}`, `isNextDisabled`). A required field that is still empty blocks the button without showing an inline error.
- A component without an `error` slot (a `CodeEditor`) renders `<Text size={TextSize.BODY_SM} variant={TextVariant.ERROR}>` beneath itself.
- Messages are short and imperative, no period. "Name is required", "Enter valid JSON", "Use a Slack incoming webhook URL".
- Server validation errors come back as `ValidationError[]` with a field path. Turn them into a `Map<path, message>` and look up per field. Show the first one as a toast too.

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

- Every field component takes `FieldComponentProps` from `fields/types.ts`. `field`, `value: JsonValue`, `onChange(JsonValue)`, `variant`, `error`, `isDisabled`, `label`, `hasStoredSecret`.
- `FieldWrapper` is the house `Field`. Label from `formatFieldName(field.name)` (snake and camel case to Title Case, acronyms upper-cased), `field.help` as `labelTooltip`, `isRequired` from the schema, `fillWidth`. With `isSection` it becomes a collapsible `Widget` for nested objects.
- Placeholders are `Enter ${label}...` for text and `Select ${label}...` for selects.
- **Secrets.** `hasStoredSecret` is true when the path exists in `connection.secretRefs`, and the placeholder becomes "Leave blank to keep current value". The copy lives inside `FieldSecret`. Callers pass the boolean, never the string. A multi-line paste is written whole to state because a single-line input would strip it.
- **Lists** are a `TagInput` ("Press Enter or comma to add a value"), or a `MultiSelectInput` with a pinned "All <label>" option through `src/utils/select.ts` when the schema enumerates values.
- **Objects without sub-fields** are a JSON `CodeEditor` that keeps local text and a parse error and only emits valid objects.
- **Visibility and defaults.** `isFieldVisible(field, siblings)` honours `visibleWhen`, `getFieldDefaults` reads proto defaults, `updateConfigField` resets dependents whose visibility flips. `getConnectionScopedFields` / `getPipelineScopedFields` split a connector's schema by `FieldScope` so the connection form and the pipeline node config each render their half.
- One `InputVariant` flows to every input kind through the `INPUT_VARIANT_TO_*_INPUT_VARIANT_MAP` records in `fields/constants.ts`. Panels use `TERTIARY`.

The connection form's footer is a `match` on the phase. `IDLE` or `ERROR` shows "Validate", `VALIDATING` shows "Testing..." loading, `VALIDATED` shows a success `Beacon label="Connected"` next to the submit, `SUBMITTING` shows "Saving..." loading. Every edit drops the phase back to `IDLE`, so a changed config must be re-validated before it can be saved.

## The provider contract

A surface whose state has invariants across fields (the canvas graph, a wizard, the connection form) gets a provider folder. Four files, always.

```
create/
├── CreatePipelineModalProvider.tsx   contexts, accessor hooks, derived state
├── actions.ts                        enum + action interfaces + union, nothing else
├── reducer.ts                        one pure function per action + a delegating switch
└── types.ts                          State, DerivedState, enums
```

**`types.ts`**

```ts
export enum CreatePipelineModalStep { CONNECTIONS = "CONNECTIONS", RESOURCES = "RESOURCES", DELIVERY = "DELIVERY", DETAILS = "DETAILS" }

export interface CreatePipelineModalState {
  step: CreatePipelineModalStep;
  sourceConnection: Connection | null;
  sinkConnections: Connection[];
  resourceSelection: Record<Connection["id"], Record<Resource["name"], boolean>>;
  name: Pipeline["name"];
  isNameTouched: boolean;
  isSubmitting: boolean;
}

export interface CreatePipelineModalDerivedState { isNextDisabled: boolean; hints: string[]; stepIndex: number; isLastStep: boolean; … }

export type CreatePipelineModalContextValue = CreatePipelineModalState & CreatePipelineModalDerivedState;
```

**`actions.ts`**

```ts
export enum CreatePipelineModalActionType { SELECT_SOURCE = "SELECT_SOURCE", SET_NAME = "SET_NAME", GO_NEXT = "GO_NEXT", … }

export interface SelectSourceAction { type: CreatePipelineModalActionType.SELECT_SOURCE; payload: Connection }
export interface SetNameAction { type: CreatePipelineModalActionType.SET_NAME; payload: Pipeline["name"] }
export interface GoNextAction { type: CreatePipelineModalActionType.GO_NEXT }

export type CreatePipelineModalAction = SelectSourceAction | SetNameAction | GoNextAction | …;
```

**`reducer.ts`**

```ts
function selectSource(state: CreatePipelineModalState, action: SelectSourceAction): CreatePipelineModalState {
  return reconcileExecutionMode({
    ...state,
    sourceConnection: state.sourceConnection?.id === action.payload.id ? null : action.payload,
    manualResources: [],
    resourceSelection: {},
  });
}

const createPipelineModalReducer = (state: CreatePipelineModalState, action: CreatePipelineModalAction): CreatePipelineModalState => {
  switch (action.type) {
    case CreatePipelineModalActionType.SELECT_SOURCE:
      return selectSource(state, action);
    …
  }
};

export default createPipelineModalReducer;
```

**`XProvider.tsx`**

```tsx
const PipelineCanvasStateContext = createContext<PipelineCanvasState | null>(null);
PipelineCanvasStateContext.displayName = "PipelineCanvasStateContext";
const PipelineCanvasDispatchContext = createContext<Dispatch<PipelineCanvasAction> | null>(null);
PipelineCanvasDispatchContext.displayName = "PipelineCanvasDispatchContext";

export const usePipelineCanvasState = () => {
  const state = useContext(PipelineCanvasStateContext);
  if (!state) throw new Error("usePipelineCanvasState must be used within PipelineCanvasProvider");
  return state;
};

const usePipelineCanvasDispatch = () => { /* same shape, not exported */ };

export const usePipelineCanvasActions = () => {
  const dispatch = usePipelineCanvasDispatch();
  return useMemo(
    () => ({
      loadGraph: (graph: PipelineCanvasGraph) => dispatch({ type: PipelineCanvasActionType.LOAD_GRAPH, payload: graph }),
      addNode: (node: CanvasNode) => dispatch({ type: PipelineCanvasActionType.ADD_NODE, payload: node }),
      removeNode: (nodeId: PipelineNode["id"]) => dispatch({ type: PipelineCanvasActionType.REMOVE_NODE, payload: nodeId }),
    }),
    [dispatch],
  );
};
```

Rules.

- **Two contexts**, state and dispatch, dispatch outermost. Both `createContext<T | null>(null)` with a `displayName`. Accessor hooks throw outside the provider.
- **Dispatch stays private to the folder.** The provider exports `useXState` and `useXActions`, the latter a memoised object of semantic functions. Consumers call `addNode(node)`, never build `{ type, payload }` or import the action enum. (The create wizard predates this and still exports its dispatch hook. New providers follow the canvas.)
- **The reducer is the sole writer and the sole enforcer of invariants.** Selecting a new source clears the resource selection in `selectSource`, not in a component. Zero casts, no `default` branch, exhaustiveness from the union.
- **Derived state is a `useMemo` in the provider** merged into the context value. `isNextDisabled` is a `match(state.step)` over the step enum, `hints` lists the concrete blockers (server messages first, prefixed `[Sink: name]` when there are several sinks, then the step's hint from `CREATE_PIPELINE_MODAL_STEP_TO_HINT_MAP`).
- **Server-backed derived data lives in a hook next to the provider** (`create/hooks/useCreatePipelineResources.ts`) that takes the reducer state and runs the discover, columns and validate queries, with `PROBE_QUERY_OPTIONS` and `placeholderData: keepPreviousData` where a flash would hurt. Pure row builders go in `rows.ts`.
- **Initial state.** `DEFAULT_STATE` for fixed defaults, an exported `createInitialState(seed)` as the `useReducer` lazy initialiser for seeded ones. Re-seed on an identity change during render (`if (previousKey !== key) { setPreviousKey(key); dispatch(load) }`), never in an effect. Reset by remounting with a `key`.
- **Serialisation is pure.** `serialize.ts` holds `mapXStateToRequest(state): XRequest` built with `create(…)`. The footer calls the mutations with the mapped request, `mutateAsync` when steps chain.
- Each step and sub-part reads the context hooks itself instead of taking props. Ephemeral UI state that is not domain state (a search box, an "is creating" toggle) is local `useState` with a `DEFAULT_*_STATE`.

## Wizard composition

```tsx
const CreatePipelineModalContent = ({ onClose }: CreatePipelineModalProps) => {
  const { step } = useCreatePipelineModalState();
  const renderBody = () =>
    match(step)
      .with(CreatePipelineModalStep.CONNECTIONS, () => <CreatePipelineModalConnections />)
      .with(CreatePipelineModalStep.RESOURCES, () => <CreatePipelineModalResources />)
      .with(CreatePipelineModalStep.DELIVERY, () => <CreatePipelineModalDelivery />)
      .with(CreatePipelineModalStep.DETAILS, () => <CreatePipelineModalDetails />)
      .exhaustive();

  return (
    <Modal isOpen size={ModalSize.X_LARGE} header="Create a new pipeline" footer={<CreatePipelineModalFooter />} onOpenChange={(isOpen) => { if (!isOpen) onClose(); }}>
      <FrameWrapper>
        <CreatePipelineModalSidebar />
        <BodyWrapper $isPadded={CREATE_PIPELINE_MODAL_STEP_TO_IS_PADDED_MAP[step]}>{renderBody()}</BodyWrapper>
      </FrameWrapper>
    </Modal>
  );
};

const CreatePipelineModal = (props: CreatePipelineModalProps) => (
  <CreatePipelineModalProvider>
    <CreatePipelineModalContent {...props} />
  </CreatePipelineModalProvider>
);
```

- The default export only mounts the provider. An inner `*Content` component reads the context.
- Every step-keyed fact is an exhaustive `Record<Step, …>` in `constants.ts`. `_STEP_ORDER`, `_STEP_TO_TITLE_MAP`, `_STEP_TO_DESCRIPTION_MAP`, `_STEP_TO_HINT_MAP`, `_STEP_TO_IS_PADDED_MAP`.
- Steps live in `steps/CreatePipelineModal<Step>.tsx`, their parts in `steps/components/CreatePipelineModal<Step><Part>.tsx`.
- A step with sections stacks `Fieldset`s in a column with `gap={24}`.
- The footer renders Back `SECONDARY` on the left, the "Invalid" chip with hints, then Next (`ArrowRightIcon`, `isIconTrailing`) or the final verb ("Create pipeline", `PlusIcon`), and "Creating..." while submitting.
