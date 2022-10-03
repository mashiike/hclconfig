package hclconfig_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/hcl/v2"
	"github.com/mashiike/hclconfig"
	"github.com/stretchr/testify/require"
)

type Config struct {
	Version   *string         `hcl:"version"`
	IOMode    string          `hcl:"io_mode"`
	General   *GeneralConfig  `hcl:"general,block"`
	Services  []ServiceConfig `hcl:"service,block"`
	Flexibles []*Flexible     `hcl:"flexible,block"`
}

type GeneralConfig struct {
	Env string `hcl:"env"`
}

type ServiceConfig struct {
	Type string `hcl:"type,label"`
	Name string `hcl:"name,label"`
	Addr string `hcl:"addr"`
	Port int    `hcl:"port"`

	Range string
}

type Flexible struct {
	Type string `hcl:"type,label"`
	Name string `hcl:"name,label"`

	Remain  hcl.Body `hcl:",remain"`
	Payload fmt.Stringer
}

type FlexibleString struct {
	Text string `hcl:"text"`
}

func (f *FlexibleString) String() string {
	return f.Text
}

type FlexibleInt struct {
	Number int `hcl:"number"`
}

func (f *FlexibleInt) String() string {
	return fmt.Sprintf("%d", f.Number)
}

func (cfg *Config) Restrict(content *hcl.BodyContent, ctx *hcl.EvalContext) hcl.Diagnostics {
	var diags hcl.Diagnostics
	if cfg.Version == nil {
		diags = append(diags, hclconfig.NewDiagnosticWarn(
			"Recommend specifying version",
			"I don't know the version of the configuration, so I'll load it as version 2",
			hclconfig.AttributeRange(content, "io_mode"),
		))
	}
	if cfg.IOMode != "readwrite" && cfg.IOMode != "readonly" {
		diags = append(diags, hclconfig.NewDiagnosticError(
			"Invalid io_mode",
			"Possible values for io_mode are readwrite or readonly",
			hclconfig.AttributeRange(content, "io_mode"),
		))
	}
	diags = append(diags, hclconfig.RestrictUniqueBlockLabels(content)...)
	return diags
}

func (cfg *GeneralConfig) Restrict(content *hcl.BodyContent, ctx *hcl.EvalContext) hcl.Diagnostics {
	var diags hcl.Diagnostics
	if cfg.Env == "" {
		diags = append(diags, hclconfig.NewDiagnosticError(
			"env need",
			"",
			hclconfig.AttributeRange(content, "env"),
		))
	}
	return diags
}

func (cfg *ServiceConfig) Restrict(content *hcl.BodyContent, ctx *hcl.EvalContext) hcl.Diagnostics {
	cfg.Range = content.MissingItemRange.String()
	return nil
}

func (cfg *Flexible) Restrict(content *hcl.BodyContent, ctx *hcl.EvalContext) hcl.Diagnostics {
	var diags hcl.Diagnostics
	switch cfg.Type {
	case "string":
		cfg.Payload = &FlexibleString{}
	case "integer":
		cfg.Payload = &FlexibleInt{}
	default:
		diags = append(diags, hclconfig.NewDiagnosticError(
			"Invalid type",
			"string or integer",
			content.MissingItemRange.Ptr(),
		))
		return diags
	}

	loadDiags := hclconfig.LoadWithBody(cfg.Remain, ctx, cfg.Payload)
	diags = append(diags, loadDiags...)
	return diags
}

func requireConfigEqual(t *testing.T, cfg1 *Config, cfg2 *Config) {
	t.Helper()
	diff := cmp.Diff(
		cfg1, cfg2,
		cmpopts.IgnoreUnexported(ServiceConfig{}, Config{}, Flexible{}),
		cmpopts.IgnoreFields(Flexible{}, "Remain"),
		cmpopts.EquateEmpty(),
		cmpopts.SortSlices(func(x, y ServiceConfig) bool {
			return x.Port < y.Port
		}),
	)
	if diff != "" {
		require.FailNow(t, diff)
	}
}

func ptr(str string) *string {
	return &str
}

func TestLoadNoError(t *testing.T) {
	os.Setenv("PORT", "8000")
	cases := []struct {
		path  string
		check func(t *testing.T, cfg *Config)
	}{
		{
			path: "testdata/sample",
			check: func(t *testing.T, cfg *Config) {
				requireConfigEqual(t,
					cfg,
					&Config{
						Version: ptr("1"),
						IOMode:  "readonly",
						Services: []ServiceConfig{
							{
								Type:  "http",
								Name:  "hoge",
								Addr:  "http://127.0.0.1",
								Port:  8000,
								Range: "testdata/sample/config.hcl:4,23-23",
							},
							{
								Type:  "http",
								Name:  "tora",
								Addr:  "http://127.0.0.1",
								Port:  8001,
								Range: "testdata/sample/config.hcl:9,23-23",
							},
							{
								Type:  "http",
								Name:  "piyo",
								Addr:  "http://127.0.0.1",
								Port:  8002,
								Range: "testdata/sample/extend.hcl:1,23-23",
							},
							{
								Type:  "http",
								Name:  "fuga",
								Addr:  "http://127.0.0.1",
								Port:  8004,
								Range: "testdata/sample/extend.hcl:6,23-23",
							},
						},
					})
			},
		},
		{
			path: "testdata/flexible",
			check: func(t *testing.T, cfg *Config) {
				requireConfigEqual(t,
					cfg,
					&Config{
						Version: ptr("1"),
						IOMode:  "readonly",
						Flexibles: []*Flexible{
							{
								Type: "string",
								Name: "hoge",
								Payload: &FlexibleString{
									Text: "hoge",
								},
							},
							{
								Type: "integer",
								Name: "tora",
								Payload: &FlexibleInt{
									Number: 1,
								},
							},
						},
					})
			},
		},
		{
			path: "testdata/local",
			check: func(t *testing.T, cfg *Config) {
				requireConfigEqual(t,
					cfg,
					&Config{
						Version: ptr("2"),
						IOMode:  "readonly",
						Flexibles: []*Flexible{
							{
								Type: "string",
								Name: "hoge",
								Payload: &FlexibleString{
									Text: "hoge",
								},
							},
							{
								Type: "integer",
								Name: "tora",
								Payload: &FlexibleInt{
									Number: 1,
								},
							},
						},
					})
			},
		},
	}

	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			loader := hclconfig.New()
			loader.DiagnosticWriter(hclconfig.DiagnosticWriterFunc(func(diag *hcl.Diagnostic) error {
				t.Log(convertDiagnosticToString(diag))
				return nil
			}))
			var cfg Config
			err := loader.Load(&cfg, c.path)
			require.NoError(t, err)
			if c.check != nil {
				c.check(t, &cfg)
			}
		})
	}
}

func TestLoadError(t *testing.T) {
	cases := []struct {
		path     string
		expected []string
	}{
		{
			path: "testdata/invalid",
			expected: []string{
				"[error] on testdata/invalid/config.hcl:2,1-8: Unsupported argument; An argument named \"ip_mode\" is not expected here. Did you mean \"io_mode\"?",
				"[error] on testdata/invalid/variable.hcl:1,1-9: Unsupported block type; Blocks of type \"variable\" are not expected here.",
				"[error] Missing required argument; The argument \"io_mode\" is required, but was not set.",
				"[error] on testdata/invalid/config.hcl:4,23-23: Missing required argument; The argument \"addr\" is required, but no definition was found.",
				"[error] on testdata/invalid/config.hcl:4,23-23: Missing required argument; The argument \"port\" is required, but no definition was found.",
				"[error] on testdata/invalid/config.hcl:5,5-16: Unsupported argument; An argument named \"listen_addr\" is not expected here.",
			},
		},
		{
			path: "testdata/restrict",
			expected: []string{
				"[warn] on testdata/restrict/config.hcl:1,1-19: Recommend specifying version; I don't know the version of the configuration, so I'll load it as version 2",
				"[error] on testdata/restrict/config.hcl:4,5-13: env need",
				"[error] on testdata/restrict/config.hcl:1,1-19: Invalid io_mode; Possible values for io_mode are readwrite or readonly",
				"[error] on testdata/restrict/config.hcl:12,1-22: Duplicate service \"http\" configuration; A http service named \"hoge\" was already declared at testdata/restrict/config.hcl:7,1-22. service names must unique per type in a configuration",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			loader := hclconfig.New()
			actual := make([]string, 0, len(c.expected))
			loader.DiagnosticWriter(hclconfig.DiagnosticWriterFunc(func(diag *hcl.Diagnostic) error {
				actual = append(actual, convertDiagnosticToString(diag))
				return nil
			}))
			var cfg Config
			err := loader.Load(&cfg, c.path)
			n := 0
			for _, str := range c.expected {
				if strings.HasPrefix(str, "[error]") {
					n++
				}
			}
			require.EqualError(t, err, fmt.Sprintf("%d errors occurred. See diagnostics for details", n))
			require.ElementsMatch(t, c.expected, actual)
		})
	}
}

func convertDiagnosticToString(diag *hcl.Diagnostic) string {
	if diag == nil {
		return "nil diagnostic"
	}
	var builder strings.Builder
	switch diag.Severity {
	case hcl.DiagError:
		fmt.Fprintf(&builder, "[error] ")
	case hcl.DiagWarning:
		fmt.Fprintf(&builder, "[warn] ")
	default:
		fmt.Fprintf(&builder, "[info] ")
	}

	if diag.Subject != nil {
		fmt.Fprintf(&builder, "on %s: ", diag.Subject.String())
	}
	fmt.Fprint(&builder, diag.Summary)

	if diag.Detail != "" {
		fmt.Fprintf(&builder, "; %s", diag.Detail)
	}
	return builder.String()
}

type testBodyDecoder struct {
	data map[string]interface{}
}

func (d *testBodyDecoder) DecodeBody(body hcl.Body, ctx *hcl.EvalContext) hcl.Diagnostics {
	attrs, diags := body.JustAttributes()
	d.data = make(map[string]interface{}, len(attrs))
	for key, attr := range attrs {
		var v interface{}
		decodeDiags := hclconfig.DecodeExpression(attr.Expr, ctx, &v)
		diags = append(diags, decodeDiags...)
		d.data[key] = v
	}
	return diags
}

func TestBodyDecoder(t *testing.T) {
	src := `
	name    = "hoge"
	age     = 82
	enabled = true
	`
	var d testBodyDecoder
	err := hclconfig.LoadWithBytes(&d, "config.hcl", []byte(src))
	require.NoError(t, err)
	require.EqualValues(t, map[string]interface{}{}, d.data)
}
