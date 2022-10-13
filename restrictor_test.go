package hclconfig_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mashiike/hclconfig"
	"github.com/stretchr/testify/require"
)

func TestRestrictRequiredBlock(t *testing.T) {
	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type: "test",
			},
			{
				Type: "case",
			},
		},
	}
	cases := []struct {
		src        string
		blockTypes []string
		expected   string
	}{
		{
			src: `
			test {
				any = true
			}
			case {
				any = false
			}`,
			blockTypes: []string{"test"},
			expected:   "no diagnostics",
		},
		{
			src: `
			case {
				any = false
			}`,
			blockTypes: []string{"test"},
			expected:   `temp.hcl:1,1-1: Missing "test" block; A "test" block is required.`,
		},
		{
			src: `
			test {
				any = true
			}`,
			blockTypes: []string{"test"},
			expected:   "no diagnostics",
		},
		{
			src: `
			test {
				any = true
			}`,
			blockTypes: []string{"test", "case"},
			expected:   `temp.hcl:1,1-1: Missing "case" block; A "case" block is required.`,
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("case.%d", i), func(t *testing.T) {
			file, diags := hclsyntax.ParseConfig([]byte(c.src), "temp.hcl", hcl.InitialPos)
			require.False(t, diags.HasErrors())
			content, diags := file.Body.Content(schema)
			require.False(t, diags.HasErrors())
			diags = hclconfig.RestrictRequiredBlock(content, c.blockTypes...)
			require.EqualValues(t, c.expected, diags.Error())
		})
	}

}

func TestRestrictUniqueBlockLabels(t *testing.T) {
	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type:       "test",
				LabelNames: []string{"name"},
			},
			{
				Type:       "case",
				LabelNames: []string{"type", "name"},
			},
			{
				Type:       "study",
				LabelNames: []string{"loc", "type", "name"},
			},
		},
	}
	cases := []struct {
		src        string
		blockTypes []string
		expected   string
	}{
		{
			src: `
			test "test1" {}
			test "test2" {}
			`,
			blockTypes: []string{"test"},
			expected:   `no diagnostics`,
		},
		{
			src: `
			test "test1" {}
			test "test1" {}
			`,
			blockTypes: []string{"test"},
			expected:   `temp.hcl:3,4-16: Duplicate test declaration; A test named "test1" was already declared at temp.hcl:2,4-16. test names must unique within a configuration`,
		},
		{
			src: `
			case "hoge" "test1"{}
			case "hoge" "test2" {}
			case "fuga" "test1" {}
			case "fuga" "test2" {}
			`,
			blockTypes: []string{"case"},
			expected:   `no diagnostics`,
		},
		{
			src: `
			case "hoge" "test1"{}
			case "fuga" "test2" {}
			case "fuga" "test1" {}
			case "fuga" "test2" {}
			`,
			blockTypes: []string{"case"},
			expected:   `temp.hcl:5,4-23: Duplicate case "fuga" configuration; A fuga case named "test2" was already declared at temp.hcl:3,4-23. case names must unique per type in a configuration`,
		},
		{
			src: `
			study "tokyo" "hoge" "test1"{}
			study "tokyo" "hoge" "test2"{}
			study "tokyo" "fuga" "test1"{}
			study "tokyo" "fuga" "test2"{}
			study "osaka" "hoge" "test1"{}
			study "osaka" "hoge" "test2"{}
			study "osaka" "fuga" "test1"{}
			study "osaka" "fuga" "test2"{}
			`,
			blockTypes: []string{"study"},
			expected:   `no diagnostics`,
		},
		{
			src: `
			study "osaka" "hoge" "test1"{}
			study "tokyo" "hoge" "test2"{}
			study "tokyo" "fuga" "test1"{}
			study "tokyo" "fuga" "test2"{}
			study "osaka" "hoge" "test1"{}
			study "osaka" "hoge" "test1"{}
			study "osaka" "fuga" "test1"{}
			study "osaka" "fuga" "test2"{}
			`,
			blockTypes: []string{"study"},
			expected:   `temp.hcl:6,4-32: Duplicate study "osaka.hoge.test1" configuration; A study named "osaka.hoge.test1" was already declared at temp.hcl:2,4-32. study names must unique per labels, and 1 other diagnostic(s)`,
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("case.%d", i), func(t *testing.T) {
			file, diags := hclsyntax.ParseConfig([]byte(c.src), "temp.hcl", hcl.InitialPos)
			require.False(t, diags.HasErrors())
			content, diags := file.Body.Content(schema)
			require.False(t, diags.HasErrors())
			diags = hclconfig.RestrictUniqueBlockLabels(content, c.blockTypes...)
			require.EqualValues(t, c.expected, diags.Error())
		})
	}

}
