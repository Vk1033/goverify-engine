package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Environment string `mapstructure:"ENVIRONMENT"`
	Port        int    `mapstructure:"PORT"`
	Kafka       KafkaConfig
	Milvus      MilvusConfig
	Redis       RedisConfig
	JWT         JWTConfig
	AI          AIConfig
}

type AIConfig struct {
	FaceModelPath   string `mapstructure:"AI_FACE_MODEL_PATH"`
	NameModelPath   string `mapstructure:"AI_NAME_MODEL_PATH"`
	LibraryPath     string `mapstructure:"AI_LIBRARY_PATH"`
}

type KafkaConfig struct {
	Brokers     []string `mapstructure:"BROKERS"`
	EnrollTopic string   `mapstructure:"ENROLL_TOPIC"`
	VerifyTopic string   `mapstructure:"VERIFY_TOPIC"`
}

type MilvusConfig struct {
	Address string `mapstructure:"ADDRESS"`
}

type RedisConfig struct {
	Address string `mapstructure:"ADDRESS"`
}

type JWTConfig struct {
	Secret string `mapstructure:"SECRET"`
}

func LoadConfig() (*Config, error) {
	viper.SetDefault("ENVIRONMENT", "development")
	viper.SetDefault("PORT", 8080)

	viper.SetDefault("KAFKA_BROKERS", "localhost:9092")
	viper.SetDefault("KAFKA_ENROLL_TOPIC", "kyc_enroll")
	viper.SetDefault("KAFKA_VERIFY_TOPIC", "kyc_verify")

	viper.SetDefault("MILVUS_ADDRESS", "localhost:19530")
	viper.SetDefault("REDIS_ADDRESS", "localhost:6379")
	viper.SetDefault("JWT_SECRET", "super-secret-key-for-hackathon")
	viper.SetDefault("AI_FACE_MODEL_PATH", "/app/models/face.onnx")
	viper.SetDefault("AI_NAME_MODEL_PATH", "/app/models/name.onnx")
	viper.SetDefault("AI_LIBRARY_PATH", "/usr/local/lib/libonnxruntime.so")

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	var cfg Config
	cfg.Environment = viper.GetString("ENVIRONMENT")
	cfg.Port = viper.GetInt("PORT")

	cfg.Kafka.Brokers = strings.Split(viper.GetString("KAFKA_BROKERS"), ",")
	cfg.Kafka.EnrollTopic = viper.GetString("KAFKA_ENROLL_TOPIC")
	cfg.Kafka.VerifyTopic = viper.GetString("KAFKA_VERIFY_TOPIC")

	cfg.Milvus.Address = viper.GetString("MILVUS_ADDRESS")
	cfg.Redis.Address = viper.GetString("REDIS_ADDRESS")
	cfg.JWT.Secret = viper.GetString("JWT_SECRET")

	cfg.AI.FaceModelPath = viper.GetString("AI_FACE_MODEL_PATH")
	cfg.AI.NameModelPath = viper.GetString("AI_NAME_MODEL_PATH")
	cfg.AI.LibraryPath = viper.GetString("AI_LIBRARY_PATH")

	return &cfg, nil
}
