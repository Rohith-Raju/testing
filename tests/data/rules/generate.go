//go:build ignore
// +build ignore

package main

import (
	"log"
	"os"
	"path"
	"text/template"
	"time"

	"github.com/jasondellaluce/falco-testing/tests/data"
)

type headerInfo struct {
	Timestamp time.Time
	Package   string
}

var headerTemplate = template.Must(template.New("header").Parse(`// Code generated by go generate; DO NOT EDIT.
// This file was generated by robots at {{ .Timestamp }}

package {{ .Package }}

import (
	"github.com/jasondellaluce/falco-testing/pkg/utils"
)
`))

func die(err error) {
	if err != nil {
		log.Fatal(err.Error())
	}
}

func main() {
	out, err := os.Create("rules_gen.go")
	die(err)
	defer out.Close()

	headerTemplate.Execute(out, headerInfo{
		Timestamp: time.Now(),
		Package:   "rules",
	})
	err = data.GenCodeFromTextFilesDir(out, "./files/", true, func(s string) bool {
		return path.Ext(s) == ".yaml"
	})
	die(err)
}