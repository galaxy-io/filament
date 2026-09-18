package server

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/transform"
)

// ListTransformFunctions serves the transform catalog, narrowed to the
// functions that accept a column of the requested logical type when one is
// given. Types cross the wire as the logical type names resource columns use.
func (a *Server) ListTransformFunctions(ctx context.Context, req *connect.Request[ingestionv1.ListTransformFunctionsRequest]) (*connect.Response[ingestionv1.ListTransformFunctionsResponse], error) {
	if _, err := tenantFromContext(ctx); err != nil {
		return nil, err
	}
	specs := transform.Functions()
	if t := req.Msg.GetLogicalType(); t != "" {
		specs = transform.FunctionsFor(rowmodel.LogicalType(t))
	}
	resp := &ingestionv1.ListTransformFunctionsResponse{
		Functions:      make([]*ingestionv1.TransformFunction, 0, len(specs)),
		GrammarVersion: int32(transform.GrammarVersion()), //nolint:gosec // small constant
		GrammarSchema:  string(transform.Grammar()),
	}
	for _, spec := range specs {
		fn := &ingestionv1.TransformFunction{
			Name: spec.Name, DisplayName: spec.DisplayName, Description: spec.Description,
			Returns: string(spec.Returns), SameType: spec.SameType, ReturnsInput: spec.ReturnsInput,
			OperatorSymbol: spec.OperatorSymbol, ConditionOperator: spec.ConditionOperator,
			VariadicAddLabel: spec.VariadicAddLabel, ConditionJoin: spec.ConditionJoin,
		}
		for _, arg := range spec.Args {
			types := make([]string, len(arg.Types))
			for i, t := range arg.Types {
				types[i] = string(t)
			}
			fn.Args = append(fn.Args, &ingestionv1.TransformArgument{
				Name: arg.Name, DisplayName: arg.DisplayName, LogicalTypes: types,
				IsColumn: arg.Column, IsLiteral: arg.Literal, IsOptional: arg.Optional, IsVariadic: arg.Variadic,
			})
		}
		resp.Functions = append(resp.Functions, fn)
	}
	return connect.NewResponse(resp), nil
}

// ValidateTransform parses and compiles a transform against one resource's
// schema, read from the source connection, exactly as a run would. Grammar
// and compile problems come back as issues addressed by path; on success the
// resource's post-transform columns come back instead.
func (a *Server) ValidateTransform(ctx context.Context, req *connect.Request[ingestionv1.ValidateTransformRequest]) (*connect.Response[ingestionv1.ValidateTransformResponse], error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	resource := req.Msg.GetResource()
	if resource == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("resource is required"))
	}
	if req.Msg.GetTransform() == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("transform is required"))
	}
	ctx, cancel := context.WithTimeout(ctx, resourceColumnsRPCTimeout)
	defer cancel()

	connector, source, err := a.openSource(ctx, tenant, "", req.Msg.GetSourceConnectionId(), nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = source.Teardown(ctx) }()
	provider, ok := source.(filament.SchemaProvider)
	if !ok {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("connector %q does not provide resource schemas", connector))
	}
	schema, err := provider.Schema(ctx, resource)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("schema for %q: %w", resource, err))
	}
	if schema.Resource == "" {
		schema.Resource = resource
	}

	// A Struct is JSON, and JSON is yaml, so the transform takes the same
	// path a hand-written one does.
	raw, err := req.Msg.GetTransform().MarshalJSON()
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	def, err := transform.Parse(raw)
	if err != nil {
		return transformIssues(err)
	}
	plan, err := transform.Compile(def, schema)
	if err != nil {
		return transformIssues(err)
	}
	out := plan.Schema()
	resp := &ingestionv1.ValidateTransformResponse{Valid: true, OutputColumns: make([]*ingestionv1.ResourceColumn, 0, len(out.Fields))}
	for _, f := range out.Fields {
		resp.OutputColumns = append(resp.OutputColumns, &ingestionv1.ResourceColumn{
			Name: f.Name, LogicalType: string(f.Logical), IsNullable: f.Nullable, IsPrimaryKey: slices.Contains(out.PrimaryKey, f.Name),
		})
	}
	return connect.NewResponse(resp), nil
}

// transformIssues renders a parse or compile failure as a response. Anything
// that is not a path-addressed transform problem is an argument error.
func transformIssues(err error) (*connect.Response[ingestionv1.ValidateTransformResponse], error) {
	var errs *transform.Errors
	if !errors.As(err, &errs) {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	resp := &ingestionv1.ValidateTransformResponse{Issues: make([]*ingestionv1.ValidationError, 0, len(errs.Issues))}
	for _, iss := range errs.Issues {
		resp.Issues = append(resp.Issues, &ingestionv1.ValidationError{Field: iss.Path, Message: iss.Message})
	}
	return connect.NewResponse(resp), nil
}
