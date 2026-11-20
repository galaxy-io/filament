package server

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"connectrpc.com/connect"

	functionsv1 "github.com/galaxy-io/alchemy/api/functions/v1"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/integrations/alchemy"
	"github.com/galaxy-io/filament/transform"
)

// WithAlchemy enables alchemy steps; a nil client leaves them disabled.
func WithAlchemy(client *alchemy.Client) Option {
	return func(s *Server) { s.alchemy = client }
}

// alchemyRuntimesTTL is how long the runtime list, static per alchemy deploy, is reused.
const alchemyRuntimesTTL = time.Minute

// alchemyRPCTimeout bounds a catalog call, so a stalled alchemy fails the picker instead of hanging it.
const alchemyRPCTimeout = 30 * time.Second

// alchemyRuntimesTimeout bounds the runtime lookup that only decorates a response.
const alchemyRuntimesTimeout = 5 * time.Second

// runtimeCache holds the last runtime list alchemy returned.
type runtimeCache struct {
	mu      sync.Mutex
	byImage map[string]*functionsv1.Runtime
	at      time.Time
}

// contracts resolves def's functions as tenant, or nil so compile reports alchemy as unconfigured.
func (a *Server) contracts(ctx context.Context, tenant filament.TenantID, def *transform.Definition) transform.Contracts {
	refs := def.Functions()
	if a.alchemy == nil || len(refs) == 0 {
		return nil
	}
	return a.alchemy.Resolve(ctx, tenant, refs)
}

// ListAlchemyFunctions pages the caller's alchemy functions for the step picker.
func (a *Server) ListAlchemyFunctions(ctx context.Context, req *connect.Request[ingestionv1.ListAlchemyFunctionsRequest]) (*connect.Response[ingestionv1.ListAlchemyFunctionsResponse], error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if a.alchemy == nil {
		return connect.NewResponse(&ingestionv1.ListAlchemyFunctionsResponse{}), nil
	}
	ctx, cancel := context.WithTimeout(alchemy.WithTenant(ctx, tenant), alchemyRPCTimeout)
	defer cancel()
	listReq := &functionsv1.ListFunctionsRequest{Search: req.Msg.GetSearch()}
	if p := req.Msg.GetPagination(); p != nil {
		listReq.Pagination = &functionsv1.PaginationRequest{PageSize: p.GetPageSize(), Cursor: p.Cursor}
	}
	out, err := a.alchemy.Functions().ListFunctions(ctx, connect.NewRequest(listReq))
	if err != nil {
		return nil, alchemyError("list alchemy functions", err)
	}
	runtimes := a.alchemyRuntimes(ctx)
	resp := &ingestionv1.ListAlchemyFunctionsResponse{
		Enabled:   true,
		Functions: make([]*ingestionv1.AlchemyFunction, 0, len(out.Msg.GetFunctions())),
	}
	if p := out.Msg.GetPagination(); p != nil {
		resp.Pagination = &ingestionv1.PaginationResponse{Total: p.GetTotal(), PreviousCursor: p.PreviousCursor, NextCursor: p.NextCursor}
	}
	for _, fn := range out.Msg.GetFunctions() {
		resp.Functions = append(resp.Functions, alchemyFunction(fn, runtimes))
	}
	return connect.NewResponse(resp), nil
}

// GetAlchemyFunction describes one function, for a step that pins it.
func (a *Server) GetAlchemyFunction(ctx context.Context, req *connect.Request[ingestionv1.GetAlchemyFunctionRequest]) (*connect.Response[ingestionv1.GetAlchemyFunctionResponse], error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if a.alchemy == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("alchemy is not configured"))
	}
	ctx, cancel := context.WithTimeout(alchemy.WithTenant(ctx, tenant), alchemyRPCTimeout)
	defer cancel()
	out, err := a.alchemy.Functions().GetFunction(ctx, connect.NewRequest(&functionsv1.GetFunctionRequest{Id: req.Msg.GetId()}))
	if err != nil {
		return nil, alchemyError("get alchemy function", err)
	}
	resp := &ingestionv1.GetAlchemyFunctionResponse{Function: alchemyFunction(out.Msg.GetFunction(), a.alchemyRuntimes(ctx))}
	if v := req.Msg.GetVersion(); v > 0 {
		version, err := a.alchemy.Functions().GetFunctionVersion(ctx, connect.NewRequest(&functionsv1.GetFunctionVersionRequest{FunctionId: req.Msg.GetId(), Version: v}))
		if err != nil {
			return nil, alchemyError("get alchemy function version", err)
		}
		resp.VersionStatus = versionStatus(version.Msg.GetVersion().GetStatus())
		resp.VersionStatusMessage = version.Msg.GetVersion().GetStatusMessage()
	}
	return connect.NewResponse(resp), nil
}

func versionStatus(s functionsv1.FunctionVersionStatus) ingestionv1.AlchemyVersionStatus {
	switch s {
	case functionsv1.FunctionVersionStatus_FUNCTION_VERSION_STATUS_READY:
		return ingestionv1.AlchemyVersionStatus_ALCHEMY_VERSION_STATUS_READY
	case functionsv1.FunctionVersionStatus_FUNCTION_VERSION_STATUS_FAILED:
		return ingestionv1.AlchemyVersionStatus_ALCHEMY_VERSION_STATUS_FAILED
	case functionsv1.FunctionVersionStatus_FUNCTION_VERSION_STATUS_UNSPECIFIED:
		return ingestionv1.AlchemyVersionStatus_ALCHEMY_VERSION_STATUS_UNSPECIFIED
	}
	return ingestionv1.AlchemyVersionStatus_ALCHEMY_VERSION_STATUS_BUILDING
}

// alchemyFunction maps a function onto the wire, decorated with its runtime when it has one.
func alchemyFunction(fn *functionsv1.Function, runtimes map[string]*functionsv1.Runtime) *ingestionv1.AlchemyFunction {
	current := fn.GetCurrentVersion()
	item := &ingestionv1.AlchemyFunction{
		Id:             fn.GetId(),
		Name:           fn.GetName(),
		Description:    fn.GetDescription(),
		CurrentVersion: current.GetVersion(),
		ContractKind:   contractKind(current.GetSpec().GetContract()),
	}
	if rt, ok := runtimes[current.GetSpec().GetImage()]; ok {
		item.Runtime = rt.GetLabel()
		item.RuntimeDarkIconUrl = rt.GetDarkIconUrl()
		item.RuntimeLightIconUrl = rt.GetLightIconUrl()
		item.RuntimeHandlerPath = rt.GetHandlerPath()
	}
	return item
}

func contractKind(c *functionsv1.Contract) ingestionv1.AlchemyContractKind {
	switch c.GetKind().(type) {
	case *functionsv1.Contract_Arrow:
		return ingestionv1.AlchemyContractKind_ALCHEMY_CONTRACT_KIND_ARROW
	case *functionsv1.Contract_JsonSchema:
		return ingestionv1.AlchemyContractKind_ALCHEMY_CONTRACT_KIND_JSON_SCHEMA
	}
	return ingestionv1.AlchemyContractKind_ALCHEMY_CONTRACT_KIND_UNSPECIFIED
}

// alchemyRuntimes indexes runtimes by image; they only decorate the list, so a failure is logged, not returned.
func (a *Server) alchemyRuntimes(ctx context.Context) map[string]*functionsv1.Runtime {
	a.runtimes.mu.Lock()
	cached, at := a.runtimes.byImage, a.runtimes.at
	a.runtimes.mu.Unlock()
	if cached != nil && time.Since(at) < alchemyRuntimesTTL {
		return cached
	}
	// A shorter deadline keeps a stalled lookup from spending the time the caller's real call needs.
	ctx, cancel := context.WithTimeout(ctx, alchemyRuntimesTimeout)
	defer cancel()
	out, err := a.alchemy.Functions().ListRuntimes(ctx, connect.NewRequest(&functionsv1.ListRuntimesRequest{}))
	if err != nil {
		if a.log != nil {
			a.log.Warn("alchemy runtimes unavailable",
				filament.Field{Key: "event.name", Value: "alchemy.runtimes.unavailable"},
				filament.Field{Key: "error", Value: err.Error()})
		}
		return nil
	}
	byImage := make(map[string]*functionsv1.Runtime, len(out.Msg.GetRuntimes()))
	for _, rt := range out.Msg.GetRuntimes() {
		byImage[rt.GetImage()] = rt
	}
	a.runtimes.mu.Lock()
	a.runtimes.byImage, a.runtimes.at = byImage, time.Now()
	a.runtimes.mu.Unlock()
	return byImage
}

// alchemyError keeps a caller's own codes, such as a canceled request, and reports anything else as alchemy being unavailable.
func alchemyError(op string, err error) error {
	switch connect.CodeOf(err) {
	case connect.CodeCanceled, connect.CodeDeadlineExceeded, connect.CodeNotFound, connect.CodeInvalidArgument:
		return err
	}
	return connect.NewError(connect.CodeUnavailable, fmt.Errorf("%s: %w", op, err))
}
