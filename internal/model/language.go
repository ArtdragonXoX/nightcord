package model

// Language 对应lang.json中的每种编程语言
type Language struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	SourceFile string `json:"source_file"`
	CompileCmd string `json:"compile_cmd"`
	RunCmd     string `json:"run_cmd"`
}
