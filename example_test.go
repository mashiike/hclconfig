package hclconfig_test

import (
	"fmt"
	"os"

	"github.com/mashiike/hclconfig"
)

func Example() {
	os.Setenv("HCLCONFIG_VAR", "hoge")
	type ExampleConfig struct {
		Value  string `hcl:"value"`
		Groups []*struct {
			Type  string `hcl:"type,label"`
			Name  string `hcl:"name,label"`
			Value int    `hcl:"value"`
		} `hcl:"group,block"`
	}
	var cfg ExampleConfig
	err := hclconfig.LoadWithBytes(&cfg, "config.hcl", []byte(`
value = must_env("HCLCONFIG_VAR")

group "type1" "default" {
	value = 1
}

group "type2" "default" {
	value = group.type1.default.value + 1
}
	`))
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("value = ", cfg.Value)
	for _, g := range cfg.Groups {
		fmt.Printf("group.%s.%s.value = %v\n", g.Type, g.Name, g.Value)
	}
	//output:
	//value =  hoge
	//group.type1.default.value = 1
	//group.type2.default.value = 2
}
