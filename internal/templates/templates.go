// Package templates embeds HTML templates for GoPage.
package templates

import "embed"

//go:embed all:files
var FS embed.FS
