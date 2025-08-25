package config

import (
	"log"
	"os"
	"path/filepath"

	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
)

type Manager[T any] struct {
	kf     *koanf.Koanf
	format Format
}

func NewManager[T any]() (*Manager[T], error) {
	format := YAMLFormat
	m := &Manager[T]{
		kf:     koanf.New("."),
		format: format,
	}

	err := m.Load(YAMLFormat, rawbytes.Provider(defaultConfig))
	if err != nil {
		return nil, err
	}

	cp := os.Getenv("CONFIG_PATH")
	ce := filepath.Ext(cp)
	if cp != "" && ce != "" {
		format = Format(ce)
		if err := m.Load(format, file.Provider(cp)); err != nil {
			return nil, err
		}
	}

	cj := os.Getenv("CONFIG_JSON")
	if cj != "" {
		if err := m.Load(JSONFormat, rawbytes.Provider([]byte(cj))); err != nil {
			log.Fatalf("Error loading configuration from CONFIG_JSON: %v", err)
		} else {
			format = JSONFormat
		}
	}

	if m.kf.Bool("debug") {
		log.Printf("Current Configuration:\n%s", m.Print())
	}

	return m, nil
}

func (m *Manager[T]) Load(format Format, provider koanf.Provider) error {
	parser, err := GetParser(format)
	if err != nil {
		return err
	}
	return m.kf.Load(provider, parser)
}

func (m *Manager[T]) Get() T {
	var c T
	tag := "key"
	switch m.format {
	case YAMLFormat:
		tag = "key"
	case JSONFormat:
		tag = "json"
	}

	err := m.kf.UnmarshalWithConf("", &c, koanf.UnmarshalConf{Tag: tag, FlatPaths: false})

	if err != nil {
		log.Fatalf("Error unmarshaling configuration: %v", err)
	}
	return c
}

func (m *Manager[T]) Print() string {
	return m.kf.Sprint()
}
