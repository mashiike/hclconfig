package hclconfig

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/tryfunc"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	ctyyaml "github.com/zclconf/go-cty-yaml"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

var defaultFunctions = map[string]function.Function{
	"abs":             stdlib.AbsoluteFunc,
	"add":             stdlib.AddFunc,
	"can":             tryfunc.CanFunc,
	"ceil":            stdlib.CeilFunc,
	"chomp":           stdlib.ChompFunc,
	"coalescelist":    stdlib.CoalesceListFunc,
	"compact":         stdlib.CompactFunc,
	"concat":          stdlib.ConcatFunc,
	"contains":        stdlib.ContainsFunc,
	"csvdecode":       stdlib.CSVDecodeFunc,
	"distinct":        stdlib.DistinctFunc,
	"element":         stdlib.ElementFunc,
	"env":             EnvFunc,
	"chunklist":       stdlib.ChunklistFunc,
	"flatten":         stdlib.FlattenFunc,
	"floor":           stdlib.FloorFunc,
	"format":          stdlib.FormatFunc,
	"formatdate":      stdlib.FormatDateFunc,
	"formatlist":      stdlib.FormatListFunc,
	"indent":          stdlib.IndentFunc,
	"index":           stdlib.IndexFunc,
	"join":            stdlib.JoinFunc,
	"jsondecode":      stdlib.JSONDecodeFunc,
	"jsonencode":      stdlib.JSONEncodeFunc,
	"keys":            stdlib.KeysFunc,
	"log":             stdlib.LogFunc,
	"lower":           stdlib.LowerFunc,
	"max":             stdlib.MaxFunc,
	"merge":           stdlib.MergeFunc,
	"min":             stdlib.MinFunc,
	"must_env":        MustEnvFunc,
	"parseint":        stdlib.ParseIntFunc,
	"pow":             stdlib.PowFunc,
	"range":           stdlib.RangeFunc,
	"regex":           stdlib.RegexFunc,
	"regexall":        stdlib.RegexAllFunc,
	"reverse":         stdlib.ReverseListFunc,
	"setintersection": stdlib.SetIntersectionFunc,
	"setproduct":      stdlib.SetProductFunc,
	"setsubtract":     stdlib.SetSubtractFunc,
	"setunion":        stdlib.SetUnionFunc,
	"signum":          stdlib.SignumFunc,
	"slice":           stdlib.SliceFunc,
	"sort":            stdlib.SortFunc,
	"split":           stdlib.SplitFunc,
	"strrev":          stdlib.ReverseFunc,
	"substr":          stdlib.SubstrFunc,
	"timeadd":         stdlib.TimeAddFunc,
	"title":           stdlib.TitleFunc,
	"trim":            stdlib.TrimFunc,
	"trimprefix":      stdlib.TrimPrefixFunc,
	"trimspace":       stdlib.TrimSpaceFunc,
	"trimsuffix":      stdlib.TrimSuffixFunc,
	"try":             tryfunc.TryFunc,
	"upper":           stdlib.UpperFunc,
	"values":          stdlib.ValuesFunc,
	"yamldecode":      ctyyaml.YAMLDecodeFunc,
	"yamlencode":      ctyyaml.YAMLEncodeFunc,
	"zipmap":          stdlib.ZipmapFunc,
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
