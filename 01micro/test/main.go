package main

import (
	"fmt"
	"github.com/Pass-baci/common"
	"github.com/asim/go-micro/v3"
	"github.com/asim/go-micro/v3/registry"
	"github.com/go-micro/plugins/v3/registry/consul"
	microOpentracing "github.com/go-micro/plugins/v3/wrapper/trace/opentracing"
	"github.com/opentracing/opentracing-go"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"test/handler"
	pb "test/proto"

	"github.com/micro/micro/v3/service/logger"
)

func main() {
	// 配置consul
	consulClient := consul.NewRegistry(func(options *registry.Options) {
		options.Addrs = []string{
			"localhost:8500",
		}
	})

	// 配置中心consul
	conf, err := common.GetConsulConfig("localhost", 8500, "/micro/config")
	if err != nil {
		logger.Fatal(err)
	}

	// 使用配置中心获取数据库配置
	var mysqlConfig = &common.MysqlConfig{}
	if mysqlConfig, err = common.GetMysqlConfigFromConsul(conf, "mysql"); err != nil {
		logger.Fatal(err)
	}

	// 连接数据库
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		mysqlConfig.User, mysqlConfig.Pwd, mysqlConfig.Host, mysqlConfig.Port, mysqlConfig.Database)
	if _, err = gorm.Open(mysql.Open(dsn), &gorm.Config{}); err != nil {
		logger.Fatal(err)
	}

	// 使用配置中心获取Jaeger配置
	var jaegerConfig = &common.JaegerConfig{}
	if jaegerConfig, err = common.GetJaegerConfigFromConsul(conf, "jaeger"); err != nil {
		logger.Fatal(err)
	}

	// 添加链路追踪
	tracer, closer, err := common.NewTracer(jaegerConfig.ServiceName, jaegerConfig.Address)
	if err != nil {
		logger.Fatal(err)
	}
	defer closer.Close()
	opentracing.SetGlobalTracer(tracer)

	// Create service
	srv := micro.NewService(
		micro.Name("test"),
		micro.Version("1.0"),
		// 添加consul
		micro.Registry(consulClient),
		// 添加链路追踪
		micro.WrapHandler(microOpentracing.NewHandlerWrapper(opentracing.GlobalTracer())),
		micro.WrapClient(microOpentracing.NewClientWrapper(opentracing.GlobalTracer())),
	)

	// Register handler
	pb.RegisterTestHandler(srv.Server(), handler.New())

	// Run service
	if err = srv.Run(); err != nil {
		logger.Fatal(err)
	}
}
