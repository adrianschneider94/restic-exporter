package main

import (
	"github.com/prometheus/common/model"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type GroupBy struct {
	isSet bool
	Tags  bool
	Paths bool
	Host  bool
}

type Config struct {
	Global  GlobalConfig
	Targets []TargetConfig
}

type GlobalConfig struct {
	Version  string
	Duration model.Duration
	GroupBy  GroupBy `yaml:"groupBy"`
}

type TargetConfig struct {
	Alias    string
	Path     string
	Password string
	GroupBy  GroupBy `yaml:"groupBy"`
}

func (cc *ResticCollector) Load(path string) {
	yamlFile, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		panic(err)
	}
	err = yaml.UnmarshalStrict(yamlFile, &cc.config)
	if err != nil {
		panic(err)
	}
}

func (cc *ResticCollector) Validate() error {
	return nil
}

func (cc *ResticCollector) ApplyDefaults() error {
	return nil
}

func (cc *GroupBy) UnmarshalYAML(unmarshal func(interface{}) error) (err error) {
	var raw []string
	err = unmarshal(&raw)
	if err != nil {
		return err
	}

	for _, option := range raw {
		cc.isSet = true

		switch option {
		case "host":
			cc.Host = true
		case "paths":
			cc.Paths = true
		case "tags":
			cc.Tags = true
		case "":
			cc.isSet = false
		default:
			cc.isSet = false
			return err
		}
	}

	return
}
