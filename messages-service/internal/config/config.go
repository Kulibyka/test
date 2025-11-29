package config

import (
	"github.com/ilyakaznacheev/cleanenv"
	"log"
	"os"
	"time"
)

type Config struct {
	Env string `yaml:"env" env-default:"local"`

	HTTPServer HTTPServerConfig `yaml:"http_server"`
	Kafka      KafkaConfig      `yaml:"kafka"`
	Retries    RetriesConfig    `yaml:"retries"`
	PostgreSQL PostgreConfig    `yaml:"postgresql"`
	Org        OrgConfig        `yaml:"org"`
}

type HTTPServerConfig struct {
	Address     string        `yaml:"address" env-default:"0.0.0.0:8080"`
	Timeout     time.Duration `yaml:"timeout" env-default:"5s"`
	IdleTimeout time.Duration `yaml:"idle_timeout" env-default:"60s"`
}

type KafkaConfig struct {
	Brokers         []string       `yaml:"brokers" env:"KAFKA_BROKERS"`
	InputTopic      string         `yaml:"input_topic" env-default:"messages_to_process"`
	OutputTopic     string         `yaml:"output_topic" env-default:"processed_messages"`
	DeadLetterTopic string         `yaml:"dead_letter_topic" env-default:"messages_failed"`
	Producer        ProducerConfig `yaml:"producer"`
}

type ProducerConfig struct {
	Acks    string        `yaml:"acks" env-default:"all"`
	Timeout time.Duration `yaml:"timeout" env-default:"3s"`
}

type RetriesConfig struct {
	MaxLLMAttempts int `yaml:"max_llm_attempts" env-default:"5"`
}

type PostgreConfig struct {
	Host     string `yaml:"host" env-default:"localhost"`
	Port     int    `yaml:"port" env-default:"5432"`
	User     string `yaml:"user" env-default:"postgres"`
	Password string `yaml:"password" env-default:"postgres"`
	DBName   string `yaml:"dbname" env-default:"postgres"`
	SSLMode  string `yaml:"sslmode" env-default:"disable"`
}

type OrgConfig struct {
	FilePath string `yaml:"file_path" env-default:"./configs/hierarchy.json"`
}

func MustLoad() *Config {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "./messages-service/configs/messages-service.yaml"
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("config file does not exist: %s", configPath)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		log.Fatalf("error reading config file: %s", err)
	}

	return &cfg
}
