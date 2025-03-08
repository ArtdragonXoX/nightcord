package language

import (
	"encoding/json"
	"log"
	"nightcord-server/internal/model"
	"os"
)

func init() {
	languages = LoadLanguages()
}

// LoadLanguages 读取并解析 lang.json，并为每种语言分配自增ID
func LoadLanguages() []model.Language {
	data, err := os.ReadFile("lang.json")
	if err != nil {
		log.Fatalf("读取lang.json错误: %v", err)
	}
	var langs []model.Language
	if err := json.Unmarshal(data, &langs); err != nil {
		log.Fatalf("解析lang.json错误: %v", err)
	}
	for i := range langs {
		langs[i].ID = i + 1
	}
	return langs
}

func GetLanguageByID(id int) model.Language {
	for _, lang := range languages {
		if lang.ID == id {
			return lang
		}
	}
	return model.Language{}
}

func GetLanguageByName(name string) model.Language {
	for _, lang := range languages {
		if lang.Name == name {
			return lang
		}
	}
	return model.Language{}
}

func GetLanguages() []model.Language {
	return languages
}
