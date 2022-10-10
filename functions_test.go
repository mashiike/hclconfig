package hclconfig_test

import (
	"testing"
	"time"

	"github.com/Songmu/flextime"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mashiike/hclconfig"
	"github.com/stretchr/testify/require"
)

func TestFunctions(t *testing.T) {
	orginalLocal := time.Local
	defer func() {
		time.Local = orginalLocal
	}()
	time.Local = time.UTC
	now, err := time.Parse(time.RFC3339, "2022-11-11T11:11:11Z")
	require.NoError(t, err)
	restore := flextime.Fix(now)
	defer restore()
	ctx := hclconfig.NewEvalContext("testdata")
	cases := []struct {
		expr   string
		str    string
		number *float64
	}{
		{
			expr:   `now()`,
			number: ptr(float64(now.Unix())),
		},
		{
			expr:   `duration("24h")`,
			number: ptr(float64((24 * time.Hour).Seconds())),
		},
		{
			expr: `strftime("%Y-%m-%d %H:%M:%S", now())`,
			str:  "2022-11-11 11:11:11",
		},
		{
			expr: `strftime_in_zone("%Y-%m-%d %H:%M:%S","Asia/Tokyo", now())`,
			str:  "2022-11-11 20:11:11",
		},
		{
			expr: `strftime_in_zone("%Y-%m-%d %H:%M:%S","Asia/Tokyo", now()+49+60*48)`,
			str:  "2022-11-11 21:00:00",
		},
		{
			expr: `strftime_in_zone("%Y-%m-%d %H:%M:%S","Asia/Tokyo", now()+duration("48m49s"))`,
			str:  "2022-11-11 21:00:00",
		},
	}
	for _, c := range cases {
		t.Run(c.expr, func(t *testing.T) {
			expr, diags := hclsyntax.ParseExpression([]byte(c.expr), "expression.hcl", hcl.InitialPos)
			if diags.HasErrors() {
				require.FailNow(t, diags.Error())
			}
			value, diags := expr.Value(ctx)
			if diags.HasErrors() {
				require.FailNow(t, diags.Error())
			}
			if c.str != "" {
				require.Equal(t, c.str, value.AsString())
			}
			if c.number != nil {
				actual, _ := value.AsBigFloat().Float64()
				require.Equal(t, *c.number, actual)
			}
		})
	}
}
