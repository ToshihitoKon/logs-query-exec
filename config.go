package main

import (
	"io"
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Retry struct {
		MaxCount int `yaml:"max_count"`
	} `yaml:"retry"`

	TemporaryDirectory  string `yaml:"temporary_directory"`
	DeleteProcessedFile bool   `yaml:"delete_processed_file"`

	Aws struct {
		S3Bucket          string `yaml:"s3_bucket"`
		S3ObjectKeyPrefix string `yaml:"s3_object_key_prefix"`
	} `yaml:"aws"`
}

var config = &Config{}

func init() {
	f, err := os.Open("config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	confData, err := io.ReadAll(f)
	if err != nil {
		log.Fatal(err)
	}

	if err := yaml.Unmarshal(confData, conf); err != nil {
		log.Fatal(err)
	}
}
