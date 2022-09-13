package hclconfig

import (
	"fmt"
	"reflect"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/zclconf/go-cty/cty"
)

const impliedVariablesCircuitBreak = 100

func impliedVariables(body hcl.Body, ctx *hcl.EvalContext, val interface{}) (map[string]cty.Value, hcl.Diagnostics) {
	cloned := &hcl.EvalContext{
		Variables: mergeVariables(make(map[string]cty.Value, len(ctx.Variables)), ctx.Variables),
		Functions: ctx.Functions,
	}
	var variables map[string]cty.Value
	var isAllKnown bool
	for i := 0; i < impliedVariablesCircuitBreak; i++ {
		variables, isAllKnown = impliedVariablesImpl(body, cloned, reflect.TypeOf(val))
		if isAllKnown {
			return variables, nil
		}
		cloned = mergeEvalContextVariables(cloned, variables)
	}
	var diags hcl.Diagnostics
	diags = append(diags, NewDiagnosticWarn("Implied Variables", fmt.Sprintf("circit break! iterate %d eval, maybe cycle refrence", impliedVariablesCircuitBreak), nil))
	return variables, diags
}

func impliedVariablesImpl(body hcl.Body, ctx *hcl.EvalContext, ty reflect.Type) (map[string]cty.Value, bool) {
	if ty.Kind() == reflect.Slice {
		ty = ty.Elem()
	}

	if ty.Kind() == reflect.Ptr {
		ty = ty.Elem()
	}

	if ty.Kind() != reflect.Struct {
		return make(map[string]cty.Value), true
	}

	schema, partial := gohcl.ImpliedBodySchema(reflect.New(ty).Interface())
	var content *hcl.BodyContent
	var diags hcl.Diagnostics
	if partial {
		content, _, diags = body.PartialContent(schema)
	} else {
		content, diags = body.Content(schema)
	}
	if diags.HasErrors() {
		return make(map[string]cty.Value), true
	}
	isAllKnown := true
	variables := make(map[string]cty.Value, len(content.Attributes))
	for _, attr := range content.Attributes {
		variable, _ := attr.Expr.Value(ctx)
		if !variable.IsKnown() {
			isAllKnown = false
		}
		variables[attr.Name] = variable
	}

	blockTypes := make(map[string]reflect.Type, len(content.Blocks))
	num := ty.NumField()
	for i := 0; i < num; i++ {
		field := ty.Field(i)
		tag := field.Tag.Get("hcl")
		name, kind := getHCLTagNameKind(tag)
		if kind != "block" {
			continue
		}
		blockTypes[name] = field.Type
	}

	for _, block := range content.Blocks {
		bty, ok := blockTypes[block.Type]
		if !ok {
			continue
		}
		blockVarialbes, known := impliedVariablesImpl(block.Body, ctx, bty)
		if !known {
			isAllKnown = false
		}

		current := blockVarialbes
		for i := len(block.Labels) - 1; i >= 0; i-- {
			current = map[string]cty.Value{
				block.Labels[i]: cty.ObjectVal(current),
			}
		}
		current = map[string]cty.Value{
			block.Type: cty.ObjectVal(current),
		}
		variables = mergeVariables(variables, current)
	}

	return variables, isAllKnown
}

func mergeEvalContextVariables(ctx *hcl.EvalContext, variables map[string]cty.Value) *hcl.EvalContext {
	ctx.Variables = mergeVariables(ctx.Variables, variables)
	return ctx
}

func mergeVariables(dst map[string]cty.Value, src map[string]cty.Value) map[string]cty.Value {
	if dst == nil {
		dst = make(map[string]cty.Value, len(src))
	}
	for key, value := range src {
		dstValue, ok := dst[key]
		if !ok {
			dst[key] = value
			continue
		}
		if !dstValue.Type().IsObjectType() {
			dst[key] = value
			continue
		}
		if !value.Type().IsObjectType() {
			dst[key] = value
			continue
		}
		dstValueMap := dstValue.AsValueMap()
		srcValueMap := value.AsValueMap()
		dst[key] = cty.ObjectVal(mergeVariables(dstValueMap, srcValueMap))
	}
	return dst
}
