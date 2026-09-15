package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config 全局配置根结构体
type Config struct {
	App        AppConfig        `mapstructure:"app"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Redis      RedisConfig      `mapstructure:"redis"`
	JWT        JWTConfig        `mapstructure:"jwt"`
	CORS       CORSConfig       `mapstructure:"cors"`
	MCP        MCPConfig        `mapstructure:"mcp"`
	Plugin     PluginConfig     `mapstructure:"plugin"`
	File       FileConfig       `mapstructure:"file"`
	SuperAdmin SuperAdminConfig `mapstructure:"super_admin"`
	Pagination PaginationConfig `mapstructure:"pagination"`
	Security   SecurityConfig   `mapstructure:"security"`
	Log        LogConfig        `mapstructure:"log"`
}

type AppConfig struct {
	Name      string `mapstructure:"name"`
	Version   string `mapstructure:"version"`
	Debug     bool   `mapstructure:"debug"`
	APIPrefix string `mapstructure:"api_prefix"`
	AdminPath string `mapstructure:"admin_path"`
}

type DatabaseConfig struct {
	Type         string `mapstructure:"type"`
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	DBName       string `mapstructure:"dbname"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	LogLevel     string `mapstructure:"log_level"`
	AutoMigrate  bool   `mapstructure:"auto_migrate"`
}

type RedisConfig struct {
	URL     string `mapstructure:"url"`
	Enabled bool   `mapstructure:"enabled"`
}

type JWTConfig struct {
	Secret              string `mapstructure:"secret"`
	Algorithm           string `mapstructure:"algorithm"`
	ExpireMinutes       int    `mapstructure:"expire_minutes"`
	RefreshExpireDays   int    `mapstructure:"refresh_expire_days"`
	BlacklistEnabled    bool   `mapstructure:"blacklist_enabled"`
}

type CORSConfig struct {
	Origins []string `mapstructure:"origins"`
}

type MCPConfig struct {
	Enabled       bool   `mapstructure:"enabled"`
	Prefix        string `mapstructure:"prefix"`
	Timeout       int    `mapstructure:"timeout"`
	MaxConcurrency int   `mapstructure:"max_concurrency"`
}

type PluginConfig struct {
	Enabled    bool           `mapstructure:"enabled"`
	BuiltinDir string         `mapstructure:"builtin_dir"`
	UploadDir  string         `mapstructure:"upload_dir"`
	ZipGuard   ZipGuardConfig `mapstructure:"zip_guard"`
}

type ZipGuardConfig struct {
	MaxZipSizeMB       int  `mapstructure:"max_zip_size_mb"`
	MaxDecompressedMB  int  `mapstructure:"max_decompressed_mb"`
	MaxEntries         int  `mapstructure:"max_entries"`
	DenySymlinks       bool `mapstructure:"deny_symlinks"`
}

type FileConfig struct {
	StorageDir string `mapstructure:"storage_dir"`
	MaxSizeMB  int    `mapstructure:"max_size_mb"`
}

type SuperAdminConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type PaginationConfig struct {
	DefaultPageSize int `mapstructure:"default_page_size"`
	MaxPageSize     int `mapstructure:"max_page_size"`
}

type SecurityConfig struct {
	LoginGuard LoginGuardConfig `mapstructure:"login_guard"`
	RateLimit  RateLimitConfig  `mapstructure:"rate_limit"`
	Upload     UploadConfig     `mapstructure:"upload"`
}

type LoginGuardConfig struct {
	MaxAttempts   int `mapstructure:"max_attempts"`
	WindowMinutes int `mapstructure:"window_minutes"`
	LockMinutes   int `mapstructure:"lock_minutes"`
}

type RateLimitConfig struct {
	Enabled           bool `mapstructure:"enabled"`
	RequestsPerMinute int  `mapstructure:"requests_per_minute"`
	Burst             int  `mapstructure:"burst"`
}

type UploadConfig struct {
	AllowedExts []string `mapstructure:"allowed_exts"`
}

type LogConfig struct {
	Level         string `mapstructure:"level"`
	Format        string `mapstructure:"format"`
	Output        string `mapstructure:"output"`
	FilePath      string `mapstructure:"file_path"`
	OpLogQueueSize int   `mapstructure:"op_log_queue_size"`
}

// Load 加载配置文件并支持环境变量覆盖
func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")
	viper.AutomaticEnv()
	viper.SetEnvPrefix("GA")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 默认 secret 熔断检测（详见 14.6）
	if cfg.JWT.Secret == "change-me-in-production-please-32chars!" && !cfg.App.Debug {
		return nil, fmt.Errorf("生产环境（debug=false）必须修改 jwt.secret，当前为默认值")
	}

	return &cfg, nil
}
