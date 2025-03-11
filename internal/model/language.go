package model

// Language 对应lang.json中的每种编程语言
type Language struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`        // 语言名
	SourceFile string `json:"source_file"` // 源文件名
	CompileCmd string `json:"compile_cmd"` // 编译命令
	RunCmd     string `json:"run_cmd"`     // 运行命令
}
