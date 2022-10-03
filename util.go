package hclconfig

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/zclconf/go-cty/cty"
)

func getBlock(blocks hcl.Blocks, tagName string, labels []string) (int, *hcl.Block) {
	for k, block := range blocks {
		if block.Type != tagName {
			continue
		}
		if len(labels) == 0 {
			return k, block
		}
		if len(labels) != len(block.Labels) {
			return -1, nil
		}
		var notEqual bool
		for i, l := range block.Labels {
			if labels[i] != l {
				notEqual = true
				break
			}
		}
		if notEqual {
			continue
		}
		return k, block
	}
	return -1, nil
}

func getLabels(val reflect.Value) []string {
	ty := val.Type()
	if ty.Kind() == reflect.Pointer {
		ty = ty.Elem()
		val = val.Elem()
	}
	if ty.Kind() != reflect.Struct {
		return []string{}
	}
	num := ty.NumField()
	labels := make([]string, 0, num)
	for i := 0; i < num; i++ {
		field := ty.Field(i)
		tag := field.Tag.Get("hcl")
		if tag == "" {
			continue
		}
		_, kind := getHCLTagNameKind(tag)
		if kind != "label" {
			continue
		}
		labels = append(labels, val.Field(i).String())
	}
	return labels
}

func getHCLTagNameKind(tag string) (string, string) {
	comma := strings.Index(tag, ",")
	if comma != -1 {
		return tag[:comma], tag[comma+1:]
	}
	return tag, "attr"
}

func AttributeRange(content *hcl.BodyContent, tagName string) *hcl.Range {
	for _, attr := range content.Attributes {
		if attr.Name == tagName {
			return attr.Range.Ptr()
		}
	}
	return content.MissingItemRange.Ptr()
}

// ExpressionDecoder is used for special decoding. Note that it is not used with gohcl.DecodeExpression.
type ExpressionDecoder interface {
	DecodeExpression(expr hcl.Expression, ctx *hcl.EvalContext) hcl.Diagnostics
}

// DecodeExpression is an extension of gohcl.DecodeExpression, which supports Decode to interface{}, etc. when the ExpressionDecoder interface is satisfied.
func DecodeExpression(expr hcl.Expression, ctx *hcl.EvalContext, val interface{}) hcl.Diagnostics {
	if decoder, ok := val.(ExpressionDecoder); ok {
		return decoder.DecodeExpression(expr, ctx)
	}
	rv := reflect.ValueOf(val)
	if rv.Kind() != reflect.Pointer {
		panic(fmt.Errorf("given value must be pointer, not %T", val))
	}
	if rv.Elem().Kind() == reflect.Interface {
		value, diags := expr.Value(ctx)
		if diags.HasErrors() {
			return diags
		}
		v := ctyValueToInterface(value)
		rv.Elem().Set(reflect.ValueOf(v))
		return nil
	}
	return gohcl.DecodeExpression(expr, ctx, val)
}

func ctyValueToInterface(value cty.Value) interface{} {
	if value.IsNull() {
		return nil
	}
	if !value.IsKnown() {
		panic("value must be known")
	}
	t := value.Type()
	switch t {
	case cty.String:
		return value.AsString()
	case cty.Bool:
		return value.True()
	case cty.Number:
		bf := value.AsBigFloat()
		if bf.IsInt() {
			i, _ := bf.Int64()
			return i
		}
		f, _ := bf.Float64()
		return f
	}
	if t.IsObjectType() || t.IsMapType() {
		valueMap := value.AsValueMap()
		m := make(map[string]interface{}, len(valueMap))
		for k, v := range valueMap {
			m[k] = ctyValueToInterface(v)
		}
		return m
	}
	if t.IsListType() || t.IsTupleType() || t.IsSetType() {
		valueSlice := value.AsValueSlice()
		s := make([]interface{}, len(valueSlice))
		for i, v := range valueSlice {
			s[i] = ctyValueToInterface(v)
		}
		return s
	}
	return nil
}
