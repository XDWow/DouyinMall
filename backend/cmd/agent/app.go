package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	agentconfig "github.com/XDWow/DouyinMall/backend/internal/agent/config"
	agentrepository "github.com/XDWow/DouyinMall/backend/internal/agent/infra/repository"
	agentioc "github.com/XDWow/DouyinMall/backend/internal/agent/ioc"
	customergraph "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator"
	grpcHandler "github.com/XDWow/DouyinMall/backend/internal/agent/transport/grpc"
	httpHandler "github.com/XDWow/DouyinMall/backend/internal/agent/transport/http"
	agentusecase "github.com/XDWow/DouyinMall/backend/internal/agent/usecase"
	pkglogger "github.com/XDWow/DouyinMall/backend/pkg/logger"
	agentservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/agent/v1/agentservice"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	kitexserver "github.com/cloudwego/kitex/server"
	"github.com/gin-gonic/gin"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/natefinch/lumberjack"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type App struct {
	httpServer *http.Server
	grpcServer kitexserver.Server
	logger     pkglogger.LoggerV1
	shutdowns  []func(context.Context) error
}

func NewApp(ctx context.Context, cfg agentconfig.Config) (*App, error) {
	log := initLogger(cfg.Observability.Log)
	pkglogger.SetGlobalLogger(log)

	db, err := initDB(ctx, cfg.DB)
	if err != nil {
		return nil, err
	}

	rdb, err := initRedis(ctx, cfg.Redis)
	if err != nil {
		return nil, err
	}

	dao := agentrepository.NewDAO(db)
	if err := dao.InitTables(ctx); err != nil {
		return nil, fmt.Errorf("init agent tables failed: %w", err)
	}

	tracer, traceShutdown, err := initTracer(ctx, cfg.Observability.Trace)
	if err != nil {
		return nil, err
	}

	workflowCfg := customergraph.DefaultConfig()
	overrideWorkflowConfig(&workflowCfg, cfg.Workflow, cfg.LLM.MaxTokens)
	workflowCfg.DefaultTenantID = defaultString(cfg.Tenant.DefaultID, workflowCfg.DefaultTenantID)
	workflowCfg.FeatureFlags = customergraph.FeatureFlags{
		OrderQuery:          cfg.FeatureFlags.OrderQuery,
		ReturnPolicy:        cfg.FeatureFlags.ReturnPolicy,
		Inventory:           cfg.FeatureFlags.Inventory,
		ProductInfo:         cfg.FeatureFlags.ProductInfo,
		ReturnExchangeApply: cfg.FeatureFlags.ReturnExchangeApply,
	}

	components, err := agentioc.InitComponents(ctx, cfg, dao, rdb)
	if err != nil {
		return nil, err
	}

	graphRuntime, err := customergraph.NewRuntime(ctx, workflowCfg, customergraph.Dependencies{
		Model:           components.Model,
		Embedder:        components.Embedder,
		KnowledgeBase:   components.KnowledgeBase,
		Skills:          components.Skills,
		Registry:        components.Registry,
		SessionService:  components.SessionService,
		ExactCache:      components.ExactCache,
		SemanticCache:   components.SemanticCache,
		RateLimiter:     components.RateLimiter,
		CheckpointStore: components.CheckpointStore,
		Prompts:         components.Prompts,
		Logger:          log,
		Metrics:         components.Metrics,
		Tracer:          tracer,
	})
	if err != nil {
		return nil, fmt.Errorf("init orchestrator runtime failed: %w", err)
	}
	usecaseFacade := agentusecase.New(graphRuntime)

	grpcServer, err := initGRPCServer(cfg, grpcHandler.NewHandler(usecaseFacade))
	if err != nil {
		return nil, err
	}

	httpServer := initHTTPServer(cfg, usecaseFacade)

	app := &App{
		httpServer: httpServer,
		grpcServer: grpcServer,
		logger:     log,
		shutdowns: []func(context.Context) error{
			func(context.Context) error { return rdb.Close() },
		},
	}
	if sqlDB, sqlErr := db.DB(); sqlErr == nil {
		app.shutdowns = append(app.shutdowns, func(context.Context) error { return sqlDB.Close() })
	}
	if traceShutdown != nil {
		app.shutdowns = append(app.shutdowns, traceShutdown)
	}
	return app, nil
}

func (a *App) Start() error {
	errCh := make(chan error, 2)
	var serverCount int

	if a.httpServer != nil {
		serverCount++
		go func() {
			a.logger.Info("agent http debug server starting", pkglogger.String("addr", a.httpServer.Addr))
			err := a.httpServer.ListenAndServe()
			if errors.Is(err, http.ErrServerClosed) {
				err = nil
			}
			errCh <- err
		}()
	}

	if a.grpcServer != nil {
		serverCount++
		go func() {
			a.logger.Info("agent grpc server starting")
			errCh <- a.grpcServer.Run()
		}()
	}

	if serverCount == 0 {
		return fmt.Errorf("no agent server configured")
	}

	for i := 0; i < serverCount; i++ {
		if err := <-errCh; err != nil {
			return err
		}
	}
	return nil
}

func (a *App) Shutdown(ctx context.Context) error {
	var errs []error

	if a.httpServer != nil {
		if err := a.httpServer.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs = append(errs, err)
		}
	}
	if a.grpcServer != nil {
		if err := a.grpcServer.Stop(); err != nil {
			errs = append(errs, err)
		}
	}
	for i := len(a.shutdowns) - 1; i >= 0; i-- {
		if err := a.shutdowns[i](ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

func initHTTPServer(cfg agentconfig.Config, service agentusecase.Service) *http.Server {
	addr := strings.TrimSpace(cfg.HTTP.Addr)
	if addr == "" {
		return nil
	}

	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery())
	registerRoutes(engine, cfg, service)

	return &http.Server{
		Addr:         addr,
		Handler:      engine,
		ReadTimeout:  secondsOrDefault(cfg.HTTP.ReadTimeoutSeconds, 30*time.Second),
		WriteTimeout: secondsOrDefault(cfg.HTTP.WriteTimeoutSeconds, 120*time.Second),
		IdleTimeout:  secondsOrDefault(cfg.HTTP.IdleTimeoutSeconds, 120*time.Second),
	}
}

func initGRPCServer(cfg agentconfig.Config, handler *grpcHandler.Handler) (kitexserver.Server, error) {
	port := cfg.GRPC.Server.Port
	if port == 0 {
		port = 8087
	}
	serviceName := defaultString(cfg.GRPC.Server.Name, "agent-service")

	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, fmt.Errorf("resolve grpc addr failed: %w", err)
	}

	options := []kitexserver.Option{
		kitexserver.WithServiceAddr(addr),
		kitexserver.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{
			ServiceName: serviceName,
		}),
	}

	if len(cfg.Etcd.Endpoints) > 0 {
		registry, err := etcd.NewEtcdRegistry(cfg.Etcd.Endpoints)
		if err != nil {
			return nil, fmt.Errorf("init etcd registry failed: %w", err)
		}
		options = append(options, kitexserver.WithRegistry(registry))
	}

	return agentservice.NewServer(handler, options...), nil
}

func registerRoutes(engine *gin.Engine, cfg agentconfig.Config, service agentusecase.Service) {
	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	metricsPath := strings.TrimSpace(cfg.Observability.Metrics.Path)
	if metricsPath == "" {
		metricsPath = "/metrics"
	}
	engine.GET(metricsPath, gin.WrapH(promhttp.Handler()))

	prefix := strings.TrimSpace(cfg.HTTP.Prefix)
	if prefix == "" {
		prefix = "/api/agent"
	}
	httpHandler.NewHandler(service).RegisterRoutes(engine.Group(prefix))
}

func initDB(ctx context.Context, cfg agentconfig.DBConfig) (*gorm.DB, error) {
	if strings.TrimSpace(cfg.DSN) == "" {
		return nil, fmt.Errorf("db.dsn is required")
	}

	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open mysql failed: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db failed: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping mysql failed: %w", err)
	}
	return db, nil
}

func initRedis(ctx context.Context, cfg agentconfig.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis failed: %w", err)
	}
	return client, nil
}

func initLogger(cfg agentconfig.LogConfig) pkglogger.LoggerV1 {
	level := parseZapLevel(cfg.Level)

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "time"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	cores := []zapcore.Core{
		zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderCfg),
			zapcore.AddSync(os.Stdout),
			level,
		),
	}
	if file := strings.TrimSpace(cfg.File); file != "" {
		cores = append(cores, zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderCfg),
			zapcore.AddSync(&lumberjack.Logger{
				Filename:   file,
				MaxSize:    50,
				MaxBackups: 3,
				MaxAge:     28,
				Compress:   true,
			}),
			level,
		))
	}

	return pkglogger.NewZapLogger(zap.New(zapcore.NewTee(cores...), zap.AddCaller()))
}

func parseZapLevel(level string) zapcore.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return zap.DebugLevel
	case "warn":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	default:
		return zap.InfoLevel
	}
}

func initTracer(ctx context.Context, cfg agentconfig.TraceConfig) (oteltrace.Tracer, func(context.Context) error, error) {
	serviceName := strings.TrimSpace(cfg.ServiceName)
	if serviceName == "" {
		serviceName = "agent-service"
	}
	if !cfg.Enabled || strings.TrimSpace(cfg.Endpoint) == "" {
		return otel.Tracer(serviceName), nil, nil
	}

	exporter, err := otlptracegrpc.New(ctx, traceExporterOptions(cfg)...)
	if err != nil {
		return nil, nil, fmt.Errorf("init otlp trace exporter failed: %w", err)
	}

	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceNameKey.String(serviceName)))
	if err != nil {
		return nil, nil, fmt.Errorf("init trace resource failed: %w", err)
	}

	ratio := cfg.SampleRatio
	if ratio <= 0 || ratio > 1 {
		ratio = 1
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(ratio)),
	)
	otel.SetTracerProvider(provider)
	return provider.Tracer(serviceName), provider.Shutdown, nil
}

func traceExporterOptions(cfg agentconfig.TraceConfig) []otlptracegrpc.Option {
	options := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		options = append(options, otlptracegrpc.WithInsecure())
	}
	return options
}

func overrideWorkflowConfig(dst *customergraph.Config, src agentconfig.WorkflowConfig, llmMaxTokens int) {
	if dst == nil {
		return
	}
	if src.RateLimitPerMinute > 0 {
		dst.RateLimitPerMinute = src.RateLimitPerMinute
	}
	if src.ConversationWindow > 0 {
		dst.ConversationWindow = src.ConversationWindow
	}
	if src.ExactCacheTTLSeconds > 0 {
		dst.ExactCacheTTL = time.Duration(src.ExactCacheTTLSeconds) * time.Second
	} else if src.L0CacheTTLSeconds > 0 {
		dst.ExactCacheTTL = time.Duration(src.L0CacheTTLSeconds) * time.Second
	}
	if src.SemanticCacheTTLSeconds > 0 {
		dst.SemanticCacheTTL = time.Duration(src.SemanticCacheTTLSeconds) * time.Second
	}
	if src.SemanticCacheScore > 0 {
		dst.SemanticCacheScore = src.SemanticCacheScore
	}
	if src.SemanticCacheTopK > 0 {
		dst.SemanticCacheTopK = src.SemanticCacheTopK
	}
	if src.RetrieveTopK > 0 {
		dst.RetrieveTopK = src.RetrieveTopK
	}
	if src.RetrieveMinScore > 0 {
		dst.RetrieveMinScore = src.RetrieveMinScore
	}
	if src.RerankTopK > 0 {
		dst.RerankTopK = src.RerankTopK
	}
	if src.ToolParallelism > 0 {
		dst.ToolParallelism = src.ToolParallelism
	}
	if src.ConfidenceThreshold > 0 {
		dst.ConfidenceThreshold = src.ConfidenceThreshold
	}
	if src.MaxAnswerTokens > 0 {
		dst.MaxAnswerTokens = src.MaxAnswerTokens
	} else if llmMaxTokens > 0 {
		dst.MaxAnswerTokens = llmMaxTokens
	}
	if src.StreamBuffer > 0 {
		dst.StreamBuffer = src.StreamBuffer
	}
	if len(src.InterruptBeforeNodes) > 0 {
		dst.InterruptBeforeNodes = append([]string(nil), src.InterruptBeforeNodes...)
	}
	if len(src.InterruptAfterNodes) > 0 {
		dst.InterruptAfterNodes = append([]string(nil), src.InterruptAfterNodes...)
	}
}

func secondsOrDefault(raw int, fallback time.Duration) time.Duration {
	if raw <= 0 {
		return fallback
	}
	return time.Duration(raw) * time.Second
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
