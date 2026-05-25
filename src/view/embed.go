package view

import "embed"

// StaticFS 嵌入静态资源文件
//
//go:embed static
var StaticFS embed.FS

// TemplateFS 嵌入模板文件
//
//go:embed template
var TemplateFS embed.FS
