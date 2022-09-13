package hclconfig

import (
	"reflect"
	"strings"

	"github.com/hashicorp/hcl/v2"
)

func getBlock(blocks hcl.Blocks, tagName string, labels []string) (int, *hcl.Block) {
	for k, block := range blocks {
		if block.Type != tagName {
			continue
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
