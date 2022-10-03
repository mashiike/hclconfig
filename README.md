# hclconfig

[![Documentation](https://godoc.org/github.com/mashiike/hclconfig?status.svg)](https://godoc.org/github.com/mashiike/hclconfig)
![Latest GitHub tag](https://img.shields.io/github/tag/mashiike/hclconfig.svg)
![Github Actions test](https://github.com/mashiike/hclconfig/workflows/Test/badge.svg?branch=main)
[![Go Report Card](https://goreportcard.com/badge/mashiike/hclconfig)](https://goreportcard.com/report/mashiike/hclconfig)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/mashiike/hclconfig/blob/master/LICENSE)

 go utility package for loading HCL(HashiCorp configuration language) files

-----

## Features
  * Local Variables
  * Implicit variables
  * Built-in functions using standard libraries 
  * Interfaces to implement additional restrictions

## Requirements
  * Go 1.18 or higher. support the 3 latest versions of Go.


See [godoc.org/github.com/mashiike/hclconfig](https://godoc.org/github.com/mashiike/hclconfig).

-----

## Installation

```shell
$ go get -u github.com/mashiike/hclconfig
```

## Usage

config/config.hcl
```hcl
io_mode = "readonly"

service "http" "hoge" {
  addr = "http://127.0.0.1"
  port = 8080
}
```

```go
package main

import (
	"github.com/mashiike/hclconfig"
)

type Config struct {
	IOMode   string          `hcl:"io_mode"`
	Services []ServiceConfig `hcl:"service,block"`
}

type ServiceConfig struct {
	Type string `hcl:"type,label"`
	Name string `hcl:"name,label"`
	Addr string `hcl:"addr"`
	Port int    `hcl:"port"`
}

func main() {
	var cfg Config
	if err := hclconfig.Load(&cfg, "./config"); err != nil {
		panic(err)
	}
	//...
}
```
### Local Variables

For example, the following statements are possible

config/config.hcl
```hcl
locals {
    addr = "http://127.0.0.1"   
}
io_mode = "readonly"

service "http" "hoge" {
  addr = local.addr 
  port = 8080
}

service "http" "fuga" {
  addr = local.addr 
  port = 8081
}
```


### Implicit variables

For example, the following statements are possible

config/config.hcl
```hcl
io_mode = "readonly"

service "http" "hoge" {
  addr = "http://127.0.0.1"
  port = 8080
}

service "http" "fuga" {
  addr = "http://127.0.0.1"
  port = service.http.hoge.port + 1
}
```

This is the ability to refer to other blocks and attributes as implicit variables.  
The evaluation of this implicit variable is done recursively. Up to 100 nests can be evaluated.  

### Built-in functions

You can use the same functions that you use most often.

For example, you can use the `must_env` or `env` functions to read environment variables. To read from a file, you can use the `file` function.

```hcl
env   = must_env("ENV")
shell = env("SHELL", "bash")
text  = file("memo.txt")
```

You can also use other functions from "github.com/zclconf/go-cty/cty/function/stdlib" such as `jsonencode` and `join`.

### Additional restrictions  

If the following interfaces are met, functions can be called after decoding to implement additional restrictions.

```go
type Restrictor interface {
	Restrict(bodyContent *hcl.BodyContent, ctx *hcl.EvalContext) hcl.Diagnostics
}
```

example code 
```go
package main

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/mashiike/hclconfig"
)

type Config struct {
	IOMode   string          `hcl:"io_mode"`
	Services []ServiceConfig `hcl:"service,block"`
}

type ServiceConfig struct {
	Type string `hcl:"type,label"`
	Name string `hcl:"name,label"`
	Addr string `hcl:"addr"`
	Port int    `hcl:"port"`
}

func (cfg *Config) Restrict(content *hcl.BodyContent, ctx *hcl.EvalContext) hcl.Diagnostics {
	var diags hcl.Diagnostics
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

func main() {
	var cfg Config
	if err := hclconfig.Load(&cfg, "./config"); err != nil {
		panic(err)
	}
	//...
}
```

In this case, if anything other than readonly and readwrite is entered in the IOMode, the following message is output.

```hcl
io_mode = "public"

service "http" "hoge" {
  addr = "http://127.0.0.1"
  port = 8080
}

service "http" "hoge" {
  addr = "http://127.0.0.1"
  port = 8080
}
```

```
Error: Invalid io_mode

  on config/config.hcl line 1:
   1: io_mode = "public"

Possible values for io_mode are readwrite or readonly

Error: Duplicate service "http" configuration

  on config/config.hcl line 8, in service "http" "hoge":
   8: service "http" "hoge" {

A http service named "hoge" was already declared at config/config.hcl:3,1-22. service names must unique per type in a configuration
```

### Custom Decode

If the given Config satisfies the following interfaces, call the customized decoding process after calculating Local Variables and Implicit Variables

```go
type BodyDecoder interface {
	DecodeBody(hcl.Body, *hcl.EvalContext) hcl.Diagnostics
}
```

In this case, it should be noted that Restrictor does not work because all DecodeBody is replaced.
When using the BodyDecocder interface, additional restrictions, etc., should also be implemented in the DecodeBody function.

## LICENSE

MIT
