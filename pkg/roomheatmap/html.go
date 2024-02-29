package roomheatmap

import _ "embed"
import "html/template"

//go:embed roomheatmap.gohtml
var rhm_template_raw string

var rhm_template = template.Must(template.New("roomheatmap").Parse(rhm_template_raw))
