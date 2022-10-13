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
