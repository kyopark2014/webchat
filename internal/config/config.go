package config

import (
	"encoding/json"
	"io/ioutil"
	"webchat/internal/logger"
)

// Config is the only one instance holding configuration
// of this service.
var (
	config *AppConfig
	log    *logger.Logger
)

func init() {
	log = logger.NewLogger("config")
}

// AppConfig is a structure into which config file
// (e.g., config/config.json) is loaded.
type AppConfig struct {
	Logging struct {
		Enable bool   `json:"Enable"`
		Level  string `json:"Level"`
		Host   string `json:"Host"`
	} `json:"Logging"`

	GracefulTermTimeMillis int64
	ChatConfig             struct {
		Host string `json:"Host"`
		Port int    `json:"Port"`
	} `json:"ChatConfig"`
	Redis  RedisConfig
	Dynamo DynamoConfig
	SQL    SQLConfig
}

// SQLConfig defines the parameters for SQL DB
type SQLConfig struct {
	Host     string `json:"Host"`
	Port     string `json:"Port"`
	Username string `json:"Username"`
	Password string `json:"Password"`
	Database string `json:"Database"`
	Protocol string `json:"Protocol"`
}

// DynamoConfig is for parameters of Dynamo
type DynamoConfig struct {
	Endpoint      string `json:"Endpoint"`
	Region        string `json:"Region"`
	ReadCapacity  int64  `json:"ReadCapacity"`
	WriteCapacity int64  `json:"WriteCapacity"`
}

// RedisConfig is for parameters of Redis
type RedisConfig struct {
	Host            string
	ReaderHost      string
	Port            string
	PoolMaxIdle     int
	PoolMaxActive   int
	PoolIdleTimeout int
	TTL             int
	Password        string
	ConnTimeout     int
}

// GetInstance returns the pointer to the singleton instance of Config
func GetInstance() *AppConfig {
	if config == nil {
		config = &AppConfig{}
	}
	return config
}

// Load reads config file (e.g., configs/config.json) and
// unmarshalls JSON string in it into Config structure
func (AppConfig) Load(fname string) bool {
	log.D("Load config from the file \"" + fname + "\".")

	b, err := ioutil.ReadFile(fname)
	if err != nil {
		log.E("%v", err)
		return false
	}

	errCode := json.Unmarshal(b, &config)
	log.D("config: %v , err: %v", config, errCode)

	return true
}
