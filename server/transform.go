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
// and compile problems come back as issues addressed by path. The response
// also carries the columns the steps that compiled produce and the type of
// every sub-expression that compiled, on success and on failure alike.
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
		// Nothing compiled, but the builder still needs the source columns.
		return transformIssues(err, nil, outputColumns(schema))
	}
	_, analysis, err := transform.Analyze(def, schema)
	if err != nil {
		// The steps that did compile still leave a schema; a builder needs it
		// for the steps after the broken one.
		return transformIssues(err, analysis.Types, outputColumns(analysis.Layout))
	}
	resp := &ingestionv1.ValidateTransformResponse{
		Valid:           true,
		OutputColumns:   outputColumns(analysis.Layout),
		ExpressionTypes: expressionTypes(analysis.Types),
	}
	return connect.NewResponse(resp), nil
}

func outputColumns(out rowmodel.Schema) []*ingestionv1.ResourceColumn {
	cols := make([]*ingestionv1.ResourceColumn, 0, len(out.Fields))
	for _, f := range out.Fields {
		cols = append(cols, &ingestionv1.ResourceColumn{
			Name: f.Name, LogicalType: string(f.Logical), IsNullable: f.Nullable, IsPrimaryKey: slices.Contains(out.PrimaryKey, f.Name),
		})
	}
	return cols
}

// transformIssues renders a parse or compile failure as a response, with the
// types of whatever did compile and the columns the steps still produce.
// Anything that is not a path-addressed transform problem is an argument error.
func transformIssues(err error, types []transform.ExpressionType, columns []*ingestionv1.ResourceColumn) (*connect.Response[ingestionv1.ValidateTransformResponse], error) {
	var errs *transform.Errors
	if !errors.As(err, &errs) {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	resp := &ingestionv1.ValidateTransformResponse{
		Issues:          make([]*ingestionv1.ValidationError, 0, len(errs.Issues)),
		OutputColumns:   columns,
		ExpressionTypes: expressionTypes(types),
	}
	for _, iss := range errs.Issues {
		resp.Issues = append(resp.Issues, &ingestionv1.ValidationError{Field: iss.Path, Message: iss.Message})
	}
	return connect.NewResponse(resp), nil
}

// expressionTypes maps the compiler's per-path types onto the wire.
func expressionTypes(types []transform.ExpressionType) []*ingestionv1.TransformExpressionType {
	out := make([]*ingestionv1.TransformExpressionType, 0, len(types))
	for _, t := range types {
		out = append(out, &ingestionv1.TransformExpressionType{Path: t.Path, LogicalType: string(t.Logical)})
	}
	return out
}
