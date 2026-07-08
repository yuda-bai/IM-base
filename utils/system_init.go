package utils

import (
	"context"
	"fmt"
	"ginchat/models"
	"log"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitConfig() {
	// 通过环境变量 GINCHAT_ENV 切换环境配置
	// GINCHAT_ENV=docker  → 加载 config/app.docker.yml（容器内用服务名）
	// 不设或设为 dev      → 加载 config/app.yml（本地开发用 127.0.0.1）
	env := os.Getenv("GINCHAT_ENV")
	configName := "app"
	if env == "docker" {
		configName = "app.docker"
		fmt.Println("📦 当前环境: Docker (使用容器服务名)")
	} else {
		fmt.Println("🖥️  当前环境: 本地开发 (使用 127.0.0.1)")
	}

	viper.SetConfigName(configName)
	viper.SetConfigType("yml")
	viper.AddConfigPath("config")
	viper.AddConfigPath("../config") // 从 test/ 等子目录运行时也能找到配置
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Println("配置文件读取错误:", err)
	}
	fmt.Println("config mysql:", viper.Get("mysql"))
}
func InitMySQL() {
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold: time.Second, // 慢 SQL 阈值
			LogLevel:      logger.Warn, // 只输出慢查询和错误，避免日志撑爆控制台/内存
			Colorful:      false,       // 生产环境关闭彩色输出
		},
	)
	dsn := viper.GetString("mysql.dns")
	if dsn == "" {
		panic("DSN 为空，请检查 config/app.yml 中 mysql.dns 配置")
	}

	// 打印 DSN（隐藏密码）
	masked := dsn
	if idx := strings.Index(dsn, ":"); idx != -1 {
		if end := strings.Index(dsn[idx:], "@"); end != -1 {
			masked = dsn[:idx+1] + "******" + dsn[idx+end:]
		}
	}
	fmt.Println("连接数据库:", masked)

	var err error
	models.DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: newLogger})
	if err != nil {
		panic("连接数据库失败: " + err.Error())
	}

	// 获取底层 sql.DB 验证连接
	sqlDB, err := models.DB.DB()
	if err != nil {
		panic("获取数据库实例失败: " + err.Error())
	}
	if err := sqlDB.Ping(); err != nil {
		panic("数据库 Ping 失败: " + err.Error())
	}
	fmt.Println("数据库连接验证成功")

	// ★ 限制连接池，防止高并发 WebSocket 时无限创建连接导致内存爆炸
	sqlDB.SetMaxOpenConns(25)                 // 最大打开连接数
	sqlDB.SetMaxIdleConns(10)                 // 最大空闲连接数
	sqlDB.SetConnMaxLifetime(5 * time.Minute) // 连接最大存活时间
	fmt.Println("数据库连接池已配置: MaxOpen=25 MaxIdle=10")

	if err := models.DB.AutoMigrate(&models.UserBasic{}, &models.Contact{}, &models.Message{}); err != nil {
		panic("数据表迁移失败: " + err.Error())
	}
	fmt.Println("数据表迁移完成")
}
func InitRedis() {
	addr := viper.GetString("redis.addr")
	password := viper.GetString("redis.password")
	db := viper.GetInt("redis.db")
	poolSize := viper.GetInt("redis.pool_size")
	minIdleConns := viper.GetInt("redis.min_idle_conns")

	fmt.Printf("Redis配置: addr=%s, password=***, db=%d, pool_size=%d, min_idle_conns=%d\n",
		addr, db, poolSize, minIdleConns)

	models.Redis = redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     poolSize,
		MinIdleConns: minIdleConns,
	})
	if err := models.Redis.Ping(context.Background()).Err(); err != nil {
		panic("连接 Redis 失败: " + err.Error())
	}
	fmt.Println("Redis 连接验证成功")
}
