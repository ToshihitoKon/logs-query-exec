package logsQueryExec

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

	SampleRequestJson string `yaml:"sample_request_json"`
}

var LqeConfig = &Config{}

func init() {
	configFile := os.Getenv("LQE_CONFIG")
	if configFile == "" {
		configFile = "config.yaml"
	}

	f, err := os.Open(configFile)
	if err != nil {
		log.Fatal(err)
	}

	confData, err := io.ReadAll(f)
	if err != nil {
		log.Fatal(err)
	}

	if err := yaml.Unmarshal(confData, LqeConfig); err != nil {
		log.Fatal(err)
	}
}
