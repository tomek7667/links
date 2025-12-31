package http

import (
	_ "embed"
	"html/template"
)

//go:embed ui/index.html
var indexHTML string

var indexTmpl = template.Must(template.New("index").Parse(indexHTML))
