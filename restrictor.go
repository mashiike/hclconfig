package hclconfig

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
)

type Restrictor interface {
	Restrict(bodyContent *hcl.BodyContent, ctx *hcl.EvalContext) hcl.Diagnostics
}

var victimRestrictor Restrictor
var restrictorType = reflect.TypeOf(&victimRestrictor).Elem()

func asRestrictor(val reflect.Value) (Restrictor, bool) {
	ty := val.Type()
	if ty.Implements(restrictorType) {
		restrictor, ok := val.Interface().(Restrictor)
		return restrictor, ok
	}
	if ty.Kind() == reflect.Pointer {
		return asRestrictor(val.Elem())
	}
	valAddr := val.Addr()
	if valAddr.Type().Implements(restrictorType) {
		restrictor, ok := valAddr.Interface().(Restrictor)
		return restrictor, ok
	}
	return nil, false
}

func restrict(body hcl.Body, ctx *hcl.EvalContext, val interface{}) hcl.Diagnostics {
	rv := reflect.ValueOf(val)
	return restrictImpl(body, ctx, rv)
}

func restrictImpl(body hcl.Body, ctx *hcl.EvalContext, val reflect.Value) hcl.Diagnostics {
	et := val.Type()
	switch et.Kind() {
	case reflect.Map, reflect.Slice, reflect.Array:
		return nil
	case reflect.Pointer:
		return restrictImpl(body, ctx, val.Elem())
	case reflect.Struct:
		return restrictStruct(body, ctx, val)
	default:
		return nil
	}
}

func restrictStruct(body hcl.Body, ctx *hcl.EvalContext, val reflect.Value) hcl.Diagnostics {
	var diags hcl.Diagnostics
	schema, partial := gohcl.ImpliedBodySchema(val.Interface())
	var content *hcl.BodyContent
	var contntDiags hcl.Diagnostics
	if partial {
		content, _, contntDiags = body.PartialContent(schema)
	} else {
		content, contntDiags = body.Content(schema)
	}
	diags = append(diags, contntDiags...)
	restrictor, ok := asRestrictor(val)
	if ok {
		diags = append(diags, restrictor.Restrict(content, ctx)...)
	}

	if content == nil {
		return diags
	}

	ty := val.Type()
	numField := ty.NumField()
	for i := 0; i < numField; i++ {
		field := ty.Field(i)
		tag := field.Tag.Get("hcl")
		if tag == "" {
			continue
		}
		tagName, kind := getHCLTagNameKind(tag)
		if tagName == "" {
			continue
		}
		if kind != "block" {
			continue
		}
		fieldValue := val.Field(i)
		if field.Type.Kind() == reflect.Slice {
			numSlice := fieldValue.Len()
			for j := 0; j < numSlice; j++ {
				v := fieldValue.Index(j)
				labels := getLabels(v)
				k, block := getBlock(content.Blocks, tagName, labels)
				if k == -1 {
					break
				}
				restrictDiags := restrictImpl(block.Body, ctx, v)
				diags = append(diags, restrictDiags...)
			}
			continue
		}
		labels := getLabels(fieldValue)
		k, block := getBlock(content.Blocks, tagName, labels)
		if k == -1 {
			continue
		}
		restrictDiags := restrictImpl(block.Body, ctx, fieldValue)
		diags = append(diags, restrictDiags...)
	}
	return diags
}

// RestrictUniqueBlockLabels implements the restriction that labels for each Block be unique.
func RestrictUniqueBlockLabels(content *hcl.BodyContent) hcl.Diagnostics {
	var diags hcl.Diagnostics
	blockRanges := make(map[string]map[string]*hcl.Range, len(content.Blocks))
	for _, block := range content.Blocks {
		if len(block.Labels) == 0 {
			continue
		}
		ranges, ok := blockRanges[block.Type]
		if !ok {
			ranges = make(map[string]*hcl.Range, 1)
		}
		labels := strings.Join(block.Labels, ".")
		if r, ok := ranges[labels]; ok {
			parts := strings.Split(labels, ".")
			switch len(parts) {
			case 1:
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  fmt.Sprintf(`Duplicate %s declaration`, block.Type),
					Detail:   fmt.Sprintf(`A %s named "%s" was already declared at %s. %s names must unique within a configuration`, block.Type, parts[0], r.String(), block.Type),
					Subject:  block.DefRange.Ptr(),
				})
			case 2:
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  fmt.Sprintf(`Duplicate %s "%s" configuration`, block.Type, parts[0]),
					Detail:   fmt.Sprintf(`A %s %s named "%s" was already declared at %s. %s names must unique per type in a configuration`, parts[0], block.Type, parts[1], r.String(), block.Type),
					Subject:  block.DefRange.Ptr(),
				})
			default:
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  fmt.Sprintf(`Duplicate %s "%s" configuration`, block.Type, labels),
					Detail:   fmt.Sprintf(`A %s named "%s" was already declared at %s. %s names must unique per labels`, block.Type, labels, r.String(), block.Type),
					Subject:  block.DefRange.Ptr(),
				})
			}
		} else {
			ranges[labels] = block.DefRange.Ptr()
		}
		blockRanges[block.Type] = ranges
	}
	return diags
}

// RestrictOnlyOneBlock implements the restriction that unique block per block type
func RestrictOnlyOneBlock(content *hcl.BodyContent, blockTypes ...string) hcl.Diagnostics {
	var diags hcl.Diagnostics
	blockRanges := make(map[string]*hcl.Range, len(blockTypes))
	for _, block := range content.Blocks {
		contains := false
		for _, blockType := range blockTypes {
			if block.Type == blockType {
				contains = true
				break
			}
		}
		if !contains {
			continue
		}
		if blockRange, ok := blockRanges[block.Type]; ok {
			diags = append(diags, NewDiagnosticError(
				fmt.Sprintf(`Duplicate "%s" block`, block.Type),
				fmt.Sprintf(`Only one "%s" block is allowed. Another was defined at %s`, block.Type, blockRange.String()),
				block.DefRange.Ptr(),
			))
		} else {
			blockRanges[block.Type] = block.DefRange.Ptr()
		}
	}
	return diags
}

// RestrictRequiredBlock implements the restriction that required block per block type
func RestrictRequiredBlock(content *hcl.BodyContent, blockTypes ...string) hcl.Diagnostics {
	var diags hcl.Diagnostics
	blockExists := make(map[string]bool, len(blockTypes))
	for _, block := range content.Blocks {
		contains := false
		for _, blockType := range blockTypes {
			if block.Type == blockType {
				contains = true
				break
			}
		}
		if !contains {
			continue
		}
		blockExists[block.Type] = true
	}
	for _, blockType := range blockTypes {
		if _, ok := blockExists[blockType]; !ok {
			diags = append(diags, NewDiagnosticError(
				fmt.Sprintf(`Missing "%s" block`, blockType),
				fmt.Sprintf(`A "%s" block is required.`, blockType),
				content.MissingItemRange.Ptr(),
			))
		}
	}
	return diags
}
