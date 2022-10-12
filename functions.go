package hclconfig

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Songmu/flextime"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/tryfunc"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/lestrrat-go/strftime"
	ctyyaml "github.com/zclconf/go-cty-yaml"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

var defaultFunctions = map[string]function.Function{
	"abs":              stdlib.AbsoluteFunc,
	"add":              stdlib.AddFunc,
	"can":              tryfunc.CanFunc,
	"ceil":             stdlib.CeilFunc,
	"chomp":            stdlib.ChompFunc,
	"coalesce":         stdlib.CoalesceFunc,
	"coalescelist":     stdlib.CoalesceListFunc,
	"compact":          stdlib.CompactFunc,
	"concat":           stdlib.ConcatFunc,
	"contains":         stdlib.ContainsFunc,
	"csvdecode":        stdlib.CSVDecodeFunc,
	"duration":         DurationFunc,
	"distinct":         stdlib.DistinctFunc,
	"element":          stdlib.ElementFunc,
	"env":              EnvFunc,
	"chunklist":        stdlib.ChunklistFunc,
	"flatten":          stdlib.FlattenFunc,
	"floor":            stdlib.FloorFunc,
	"format":           stdlib.FormatFunc,
	"formatdate":       stdlib.FormatDateFunc,
	"formatlist":       stdlib.FormatListFunc,
	"indent":           stdlib.IndentFunc,
	"index":            stdlib.IndexFunc,
	"join":             stdlib.JoinFunc,
	"jsondecode":       stdlib.JSONDecodeFunc,
	"jsonencode":       stdlib.JSONEncodeFunc,
	"keys":             stdlib.KeysFunc,
	"log":              stdlib.LogFunc,
	"lower":            stdlib.LowerFunc,
	"max":              stdlib.MaxFunc,
	"merge":            stdlib.MergeFunc,
	"min":              stdlib.MinFunc,
	"must_env":         MustEnvFunc,
	"now":              NowFunc,
	"parseint":         stdlib.ParseIntFunc,
	"pow":              stdlib.PowFunc,
	"range":            stdlib.RangeFunc,
	"regex":            stdlib.RegexFunc,
	"regexall":         stdlib.RegexAllFunc,
	"reverse":          stdlib.ReverseListFunc,
	"setintersection":  stdlib.SetIntersectionFunc,
	"setproduct":       stdlib.SetProductFunc,
	"setsubtract":      stdlib.SetSubtractFunc,
	"setunion":         stdlib.SetUnionFunc,
	"signum":           stdlib.SignumFunc,
	"strftime":         StrftimeFunc,
	"strftime_in_zone": StrftimeInZoneFunc,
	"slice":            stdlib.SliceFunc,
	"sort":             stdlib.SortFunc,
	"split":            stdlib.SplitFunc,
	"strrev":           stdlib.ReverseFunc,
	"substr":           stdlib.SubstrFunc,
	"timeadd":          stdlib.TimeAddFunc,
	"title":            stdlib.TitleFunc,
	"trim":             stdlib.TrimFunc,
	"trimprefix":       stdlib.TrimPrefixFunc,
	"trimspace":        stdlib.TrimSpaceFunc,
	"trimsuffix":       stdlib.TrimSuffixFunc,
	"try":              tryfunc.TryFunc,
	"upper":            stdlib.UpperFunc,
	"values":           stdlib.ValuesFunc,
	"yamldecode":       ctyyaml.YAMLDecodeFunc,
	"yamlencode":       ctyyaml.YAMLEncodeFunc,
	"zipmap":           stdlib.ZipmapFunc,
}

func mergeFunctions(dst map[string]function.Function, src map[string]function.Function) map[string]function.Function {
	if dst == nil {
		dst = make(map[string]function.Function, len(src))
	}
	for name, f := range src {
		dst[name] = f
	}
	return dst
}

var MustEnvFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name:        "key",
			Type:        cty.String,
			AllowMarked: true,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		keyArg, keyMarks := args[0].Unmark()
		key := keyArg.AsString()
		value := os.Getenv(key)
		if value == "" {
			err := function.NewArgError(0, fmt.Errorf("env `%s` is not set", key))
			return cty.UnknownVal(cty.String), err
		}
		return cty.StringVal(value).WithMarks(keyMarks), nil
	},
})

var EnvFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name:        "key",
			Type:        cty.String,
			AllowMarked: true,
		},
		{
			Name:      "default",
			Type:      cty.String,
			AllowNull: true,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		keyArg, keyMarks := args[0].Unmark()
		key := keyArg.AsString()
		if value := os.Getenv(key); value != "" {
			return cty.StringVal(value).WithMarks(keyMarks), nil
		}
		if args[1].IsNull() {
			return cty.StringVal("").WithMarks(keyMarks), nil
		}
		return cty.StringVal(args[1].AsString()).WithMarks(keyMarks), nil
	},
})

func openFile(path string, basePaths ...string) ([]byte, error) {
	var targetPath string
	if filepath.IsAbs(path) {
		targetPath = path
	} else {
		if wd, err := os.Getwd(); err == nil {
			basePaths = append(basePaths, wd)
		}
		for _, basePath := range basePaths {
			path := filepath.Join(basePath, path)
			if _, err := os.Stat(path); err != nil {
				continue
			}
			targetPath = path
			break
		}
	}
	if targetPath == "" {
		return nil, fmt.Errorf("%s not found", path)
	}
	fp, err := os.Open(targetPath)
	if err != nil {
		return nil, err
	}
	defer fp.Close()
	bs, err := io.ReadAll(fp)
	if err != nil {
		return nil, err
	}
	return bs, nil
}

func MakeFileFunc(basePaths ...string) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:        "path",
				Type:        cty.String,
				AllowMarked: true,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			pathArg, pathMarks := args[0].Unmark()
			content, err := openFile(pathArg.AsString(), basePaths...)
			if err != nil {
				err = function.NewArgError(0, err)
				return cty.UnknownVal(cty.String), err
			}
			return cty.StringVal(string(content)).WithMarks(pathMarks), nil
		},
	})
}

func MakeTemplateFileFunc(newEvalContext func() *hcl.EvalContext, basePaths ...string) function.Function {
	render := func(args []cty.Value) (cty.Value, error) {
		if len(args) != 2 {
			return cty.UnknownVal(cty.DynamicPseudoType), errors.New("require argument length 2")
		}
		if ty := args[1].Type(); !ty.IsObjectType() && !ty.IsMapType() {
			return cty.UnknownVal(cty.DynamicPseudoType), errors.New("require second argument is map or object type")
		}
		pathArg, pathMarks := args[0].Unmark()
		targetFile := pathArg.AsString()
		src, err := openFile(targetFile, basePaths...)
		if err != nil {
			err = function.NewArgError(0, err)
			return cty.UnknownVal(cty.DynamicPseudoType), err
		}
		expr, diags := hclsyntax.ParseTemplate(src, targetFile, hcl.InitialPos)
		if diags.HasErrors() {
			return cty.UnknownVal(cty.DynamicPseudoType), diags
		}
		ctx := newEvalContext()
		ctx = ctx.NewChild()
		ctx.Variables = args[1].AsValueMap()
		value, diags := expr.Value(ctx)
		if diags.HasErrors() {
			return cty.UnknownVal(cty.DynamicPseudoType), diags
		}
		return value.WithMarks(pathMarks), nil
	}

	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:        "path",
				Type:        cty.String,
				AllowMarked: true,
			},
			{
				Name: "variables",
				Type: cty.DynamicPseudoType,
			},
		},
		Type: func(args []cty.Value) (cty.Type, error) {
			if !args[0].IsKnown() || args[1].IsKnown() {
				return cty.DynamicPseudoType, nil
			}
			val, err := render(args)
			return val.Type(), err
		},
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return render(args)
		},
	})
}

func StrftimeInZone(layout string, zone string, t time.Time) (string, error) {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return "", err
	}
	return Strftime(layout, loc, t)
}

func Strftime(layout string, loc *time.Location, t time.Time) (string, error) {
	t = t.In(loc)
	if strings.ContainsRune(layout, '%') {
		f, err := strftime.New(layout)
		if err != nil {
			return "", err
		}
		return f.FormatString(t), nil
	}
	if strings.EqualFold("rfc3399", layout) {
		return t.Format(time.RFC3339), nil
	}
	return t.Format(layout), nil
}

func nowUnixSeconds() float64 {
	now := flextime.Now()
	return float64(now.UnixNano()) / float64(time.Second)
}
func unixSecondsToTime(unixSeconds float64) time.Time {
	return time.Unix(0, int64(unixSeconds*float64(time.Second)))
}

var NowFunc = function.New(&function.Spec{
	Params: []function.Parameter{},
	Type:   function.StaticReturnType(cty.Number),
	Impl: func(_ []cty.Value, retType cty.Type) (cty.Value, error) {
		return cty.NumberFloatVal(nowUnixSeconds()), nil
	},
})

var DurationFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name:        "d",
			Type:        cty.String,
			AllowMarked: true,
		},
	},
	Type: function.StaticReturnType(cty.Number),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		durationArg, durationMarks := args[0].Unmark()
		durationStr := durationArg.AsString()
		d, err := time.ParseDuration(durationStr)
		if err != nil {
			return cty.UnknownVal(cty.Number), err
		}
		return cty.NumberFloatVal(float64(d) / float64(time.Second)).WithMarks(durationMarks), nil
	},
})

var StrftimeFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name:        "layout",
			Type:        cty.String,
			AllowMarked: true,
		},
		{
			Name:        "unixSeconds",
			Type:        cty.Number,
			AllowMarked: true,
			AllowNull:   true,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		layoutArg, layoutMarks := args[0].Unmark()
		layout := layoutArg.AsString()

		unixSecondsArg, unixSeconcsMarks := args[1].Unmark()
		var unixSeconds float64
		if unixSecondsArg.IsNull() {
			unixSeconds = nowUnixSeconds()
		} else {
			f := unixSecondsArg.AsBigFloat()
			unixSeconds, _ = f.Float64()
		}

		t, err := Strftime(layout, time.Local, unixSecondsToTime(unixSeconds))
		if err != nil {
			return cty.UnknownVal(cty.String), err
		}
		return cty.StringVal(t).WithMarks(layoutMarks, unixSeconcsMarks), nil
	},
})

var StrftimeInZoneFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name:        "layout",
			Type:        cty.String,
			AllowMarked: true,
		},
		{
			Name:        "timeZone",
			Type:        cty.String,
			AllowMarked: true,
		},
		{
			Name:        "unixSeconds",
			Type:        cty.Number,
			AllowMarked: true,
			AllowNull:   true,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		layoutArg, layoutMarks := args[0].Unmark()
		layout := layoutArg.AsString()

		zoneArg, zoneMarks := args[1].Unmark()
		zone := zoneArg.AsString()

		unixSecondsArg, unixSeconcsMarks := args[2].Unmark()
		var unixSeconds float64
		if unixSecondsArg.IsNull() {
			unixSeconds = nowUnixSeconds()
		} else {
			f := unixSecondsArg.AsBigFloat()
			unixSeconds, _ = f.Float64()
		}

		t, err := StrftimeInZone(layout, zone, unixSecondsToTime(unixSeconds))
		if err != nil {
			return cty.UnknownVal(cty.String), err
		}
		return cty.StringVal(t).WithMarks(layoutMarks, zoneMarks, unixSeconcsMarks), nil
	},
})
