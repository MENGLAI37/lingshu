package config

import (
	"fmt"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	SchemaVersion string      `mapstructure:"schema_version" json:"schema_version"`
	Server        ServerConfig `mapstructure:"server" json:"server"`
	Database      DBConfig     `mapstructure:"database" json:"database"`
	Redis         RedisConfig  `mapstructure:"redis" json:"redis"`
	Log           LogConfig    `mapstructure:"log" json:"log"`
}

type ServerConfig struct {
	Host string `mapstructure:"host" json:"host"`
	Port int    `mapstructure:"port" json:"port"`
}

type DBConfig struct {
	Type         string `mapstructure:"type" json:"type"`
	Host         string `mapstructure:"host" json:"host"`
	Port         int    `mapstructure:"port" json:"port"`
	User         string `mapstructure:"user" json:"user"`
	Password     string `mapstructure:"password" json:"password"`
	DBName       string `mapstructure:"dbname" json:"dbname"`
	SSLMode      string `mapstructure:"sslmode" json:"sslmode"`
	MaxOpenConns int    `mapstructure:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns" json:"max_idle_conns"`
}

type RedisConfig struct {
	Mode         string   `mapstructure:"mode" json:"mode"`
	Addresses    []string `mapstructure:"addresses" json:"addresses"`
	MasterName   string   `mapstructure:"master_name" json:"master_name"`
	Password     string   `mapstructure:"password" json:"password"`
	DB           int      `mapstructure:"db" json:"db"`
	PoolSize     int      `mapstructure:"pool_size" json:"pool_size"`
	MinIdleConns int      `mapstructure:"min_idle_conns" json:"min_idle_conns"`
}

type LogConfig struct {
	Level  string `mapstructure:"level" json:"level"`
	Format string `mapstructure:"format" json:"format"`
}

var (
	instance *Config
	once     sync.Once
	mu       sync.RWMutex
)

func Load(configPath string) (*Config, error) {
	var err error
	once.Do(func() {
		v := viper.New()
		v.SetConfigType("yaml")
		v.SetConfigName("config")

		if configPath != "" {
			v.AddConfigPath(configPath)
		}
		v.AddConfigPath(".")
		v.AddConfigPath("./configs")
		v.AddConfigPath("/etc/lingshu")

		v.SetEnvPrefix("OPSAI")
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		v.AutomaticEnv()

		setDefaults(v)

		if err = v.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); ok {
				err = nil
			} else {
				return
			}
		}

		v.WatchConfig()
		v.OnConfigChange(func(e fsnotify.Event) {
			mu.Lock()
			defer mu.Unlock()
			var newCfg Config
			if err := v.Unmarshal(&newCfg); err == nil {
				instance = &newCfg
			}
		})

		var cfg Config
		if err = v.Unmarshal(&cfg); err != nil {
			return
		}

		if err = validateSchemaVersion(cfg.SchemaVersion); err != nil {
			return
		}

		instance = &cfg
	})

	if err != nil {
		return nil, err
	}

	mu.RLock()
	defer mu.RUnlock()
	return instance, nil
}

func Get() *Config {
	mu.RLock()
	defer mu.RUnlock()
	return instance
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("schema_version", "v2.3.0")
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)

	v.SetDefault("database.type", "postgres")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "opsai")
	v.SetDefault("database.password", "opsai")
	v.SetDefault("database.dbname", "opsai")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_open_conns", 20)
	v.SetDefault("database.max_idle_conns", 5)

	v.SetDefault("redis.mode", "single")
	v.SetDefault("redis.addresses", []string{"localhost:6379"})
	v.SetDefault("redis.master_name", "mymaster")
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 10)
	v.SetDefault("redis.min_idle_conns", 3)

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
}

func validateSchemaVersion(version string) error {
	if version == "" {
		return fmt.Errorf("schema_version is required")
	}
	validVersions := map[string]bool{
		"v1.8.0": true,
		"v1.9.0": true,
		"v2.0.0": true,
		"v2.1.0": true,
		"v2.2.0": true,
		"v2.3.0": true,
	}
	if !validVersions[version] {
		return fmt.Errorf("unsupported schema_version: %s", version)
	}
	return nil
}
