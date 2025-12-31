package config

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Config 应用配置结构
type Config struct {
	// 飞书配置
	FeishuAppID     string
	FeishuAppSecret string

	// 服务器配置
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration

	// 日志配置
	LogLevel string

	// 性能配置
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	IdleConnTimeout     time.Duration

	// Jenkins 配置
	JenkinsURL   string
	JenkinsUser  string
	JenkinsToken string

	//mysql 配置
	mysqlHost     string
	mysqlPort     int
	mysqlUser     string
	mysqlPassword string
	mysqlDatabase string
	debug         bool
	db            *gorm.DB
	lock          sync.Mutex
	Application   *application
}

// 应用服务

type application struct {
	host   string
	port   int
	domain string
	server *gin.Engine
	lock   sync.Mutex
	root   gin.IRouter
}

var (
	cfg  *Config
	once sync.Once
)

// LoadConfig 加载配置
func LoadConfig() (*Config, error) {
	var err error
	once.Do(func() {
		cfg = &Config{
			// 从环境变量加载配置
			FeishuAppID:     getEnv("FEISHU_APP_ID", ""),
			FeishuAppSecret: getEnv("FEISHU_APP_SECRET", ""),
			Port:            getEnv("PORT", "8080"),
			LogLevel:        getEnv("LOG_LEVEL", "info"),

			// 超时配置
			ReadTimeout:     getDurationEnv("READ_TIMEOUT", 10*time.Second),
			WriteTimeout:    getDurationEnv("WRITE_TIMEOUT", 10*time.Second),
			ShutdownTimeout: getDurationEnv("SHUTDOWN_TIMEOUT", 5*time.Second),

			// 连接池配置
			MaxIdleConns:        getIntEnv("MAX_IDLE_CONNS", 100),
			MaxIdleConnsPerHost: getIntEnv("MAX_IDLE_CONNS_PER_HOST", 10),
			IdleConnTimeout:     getDurationEnv("IDLE_CONN_TIMEOUT", 90*time.Second),

			// Jenkins 配置
			JenkinsURL:   getEnv("JENKINS_URL", "http://10.8.2.192:30008/"),
			JenkinsUser:  getEnv("JENKINS_USER", "admin"),
			JenkinsToken: getEnv("JENKINS_TOKEN", "admin123"),

			//mysql 配置
			mysqlHost:     getEnv("MYSQL_HOST", "localhost"),
			mysqlPort:     getIntEnv("MYSQL_PORT", 3306),
			mysqlUser:     getEnv("MYSQL_USER", "root"),
			mysqlPassword: getEnv("MYSQL_PASSWORD", ""),
			mysqlDatabase: getEnv("MYSQL_DATABASE", "feishu"),
			debug:         getEnv("DEBUG", "false") == "true",

			// 应用服务
			Application: &application{
				host:   "localhost",
				port:   8080,
				domain: "localhost",
			},
		}

		// 验证必需的配置
		if vErr := cfg.Validate(); vErr != nil {
			err = fmt.Errorf("config validation failed: %w", vErr)
		}
	})

	return cfg, err
}

func (a *application) GinServer() *gin.Engine {
	a.lock.Lock()
	defer a.lock.Unlock()

	if a.server == nil {
		a.server = gin.Default()
		// 加载全局CROS中间件
		// middleware.CROS
		a.server.Use(cors.Default())
	}

	return a.server
}

func (a *application) GinRootRouter() gin.IRouter {
	r := a.GinServer()

	if a.root == nil {
		a.root = r.Group("app").Group("api").Group("v1")
	}

	return a.root
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.FeishuAppID == "" {
		return fmt.Errorf("FEISHU_APP_ID is required")
	}
	if c.FeishuAppSecret == "" {
		return fmt.Errorf("FEISHU_APP_SECRET is required")
	}
	return nil
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getIntEnv 获取整数类型的环境变量
func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getDurationEnv 获取时间间隔类型的环境变量（秒）
func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return time.Duration(intValue) * time.Second
		}
	}
	return defaultValue
}

// DNS 数据库连接字符串
func (c *Config) DNS() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.mysqlUser, c.mysqlPassword, c.mysqlHost, c.mysqlPort, c.mysqlDatabase)
}

// 获取DB
func (c *Config) GetDB() *gorm.DB {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.db == nil {
		name, err := gorm.Open(mysql.Open(c.DNS()), &gorm.Config{})
		if err != nil {
			panic(fmt.Sprintf("failed to connect mysql: %v", err))
		}
		c.db = name

		if c.debug {
			c.db = c.db.Debug()
		}
	}

	return c.db
}
