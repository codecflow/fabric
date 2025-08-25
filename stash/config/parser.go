package config

import (
	"errors"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/v2"
)

type ParserFunc func() (koanf.Parser, error)

var parsers map[Format]ParserFunc = map[Format]ParserFunc{
	JSONFormat: jsonParserFunc,
	YAMLFormat: yamlParserFunc,
	YMLFormat:  yamlParserFunc,
}

func jsonParserFunc() (koanf.Parser, error) {
	return json.Parser(), nil
}

func yamlParserFunc() (koanf.Parser, error) {
	return yaml.Parser(), nil
}

func GetParser(format Format) (koanf.Parser, error) {
	if parserFunc, ok := parsers[format]; ok {
		return parserFunc()
	}
	return nil, errors.New("parser not found for format" + string(format))
}
