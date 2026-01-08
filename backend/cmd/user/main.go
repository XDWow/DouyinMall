package main

import (
	"log"

	"github.com/XDWow/DouyinMall/backend/internal/user/repo"
	"github.com/XDWow/DouyinMall/backend/internal/user/repo/dao"
	"github.com/XDWow/DouyinMall/backend/internal/user/service"
	userv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/user/v1/userservice"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	// 初始化日志
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to init logger: %v", err)
	}
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	// 初始化数据库连接
	// TODO: 从配置文件或环境变量读取数据库配置
	dsn := "root:password@tcp(127.0.0.1:3306)/douyin_mall?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// 自动迁移数据库表结构
	if err := db.AutoMigrate(&dao.User{}); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	// 依赖注入：初始化各层
	userDAO := dao.NewUserDAO(db)
	userRepo := repo.NewUserRepository(userDAO)
	userService := service.NewUserService(userRepo, logger)
	userHandler := NewUserServiceImpl(userService)

	// 创建并启动 Kitex 服务
	svr := userv1.NewServer(userHandler)

	err = svr.Run()
	if err != nil {
		log.Println(err.Error())
	}
}
