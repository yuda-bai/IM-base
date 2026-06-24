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
	viper.SetConfigName("app")
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
			LogLevel:      logger.Info, // Log level
			Colorful:      true,        // 禁用彩色打印
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

	if err := models.DB.AutoMigrate(&models.UserBasic{}); err != nil {
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

const (
	PublishKey = "websocket"
)

// Publish 发布消息到Redis
func Publish(ctx context.Context, channel string, message string) error {
	var err error
	err = models.Redis.Publish(ctx, channel, message).Err()
	fmt.Println("发布消息")
	return err
}

// Subscribe 订阅Redis消息
func Subscribe(ctx context.Context, channel string) (string, error) {
	sub := models.Redis.PSubscribe(ctx, channel)
	fmt.Println("订阅成功")
	msg, err := sub.ReceiveMessage(ctx)
	return msg.Payload, err
}
