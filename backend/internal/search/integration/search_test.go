//go:build integration
// +build integration

// Package integration 集成测试
//
// 使用 Testcontainers 启动真实的 Elasticsearch 容器进行测试
// 需要本地安装 Docker
//
// 运行方式：
//
//	go test -tags=integration ./internal/search/integration/... -v
package integration

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/search/domain"
	"github.com/XDWow/DouyinMall/backend/internal/search/repo"
	"github.com/XDWow/DouyinMall/backend/internal/search/repo/es"
	"github.com/XDWow/DouyinMall/backend/internal/search/service"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// SearchIntegrationSuite 搜索服务集成测试套件
type SearchIntegrationSuite struct {
	suite.Suite

	// 容器
	esContainer testcontainers.Container

	// ES 客户端
	esClient *es.ESClient

	// 服务组件
	productRepo  repo.ProductRepo
	merchantRepo repo.MerchantRepo
	searchSvc    service.SearchService
}

func TestSearchIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（短模式）")
	}
	suite.Run(t, new(SearchIntegrationSuite))
}

// SetupSuite 测试套件开始前执行
func (s *SearchIntegrationSuite) SetupSuite() {
	ctx := context.Background()

	// 方案：使用 BuildFromDockerfile 构建预装 IK 插件的镜像
	// 这样就不需要在运行时安装插件和重启容器了
	// 问题：Testcontainers 的 Stop/Start 可能销毁重建容器，导致运行时安装的插件丢失
	// 解决：使用预装插件的镜像，插件已经"bake"到镜像中，容器重启不会丢失

	// 动态计算 Dockerfile 路径
	// 测试文件在：backend/internal/search/integration/search_test.go
	// Dockerfile 在：docker/elasticsearch/Dockerfile（项目根目录）
	// 从测试文件所在目录（integration）到项目根目录需要向上四级
	// integration -> search -> internal -> backend -> 项目根目录
	_, filename, _, _ := runtime.Caller(0)                        // 获取当前测试文件路径
	testDir := filepath.Dir(filename)                             // backend/internal/search/integration
	projectRoot := filepath.Join(testDir, "..", "..", "..", "..") // 向上四级到项目根目录
	projectRoot, err := filepath.Abs(projectRoot)                 // 转换为绝对路径，解析 .. 符号
	require.NoError(s.T(), err)
	dockerfilePath := filepath.Join(projectRoot, "docker", "elasticsearch")

	s.T().Logf("Dockerfile 路径: %s", dockerfilePath)

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    dockerfilePath,
			Dockerfile: "Dockerfile",
			// 可选：指定镜像名称，避免每次都重新构建（但首次需要构建）
			// ImageName: "es-ik:7.13.0",
		},
		ExposedPorts: []string{"9200/tcp"},
		Env: map[string]string{
			"discovery.type":         "single-node",
			"xpack.security.enabled": "false",
			"ES_JAVA_OPTS":           "-Xms256m -Xmx512m",
			"cluster.name":           "elasticsearch",
		},
		WaitingFor: wait.ForHTTP("/").
			WithPort("9200/tcp").
			WithStatusCodeMatcher(func(status int) bool {
				return status == 200
			}).
			WithStartupTimeout(60 * time.Second),
	}

	esContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(s.T(), err)
	s.esContainer = esContainer

	// 获取 ES 地址
	host, err := esContainer.Host(ctx)
	require.NoError(s.T(), err)

	port, err := esContainer.MappedPort(ctx, "9200")
	require.NoError(s.T(), err)

	esAddr := "http://" + host + ":" + port.Port()

	// 等待 ES 完全就绪（插件已经预装在镜像中，启动时就会加载）
	s.T().Logf("等待 ES 完全就绪...")
	time.Sleep(5 * time.Second)

	// 创建 ES 客户端
	esClient, err := es.NewESClient([]string{esAddr})
	require.NoError(s.T(), err)
	s.esClient = esClient

	// 验证插件是否加载成功（通过 ES API 检查插件列表）
	s.T().Logf("验证 IK 插件是否加载...")
	resp, err := http.Get(esAddr + "/_cat/plugins?format=json")
	if err == nil && resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		s.T().Logf("已安装的插件: %s", string(body))
		// 检查是否包含 IK 插件
		if !strings.Contains(string(body), "analysis-ik") {
			s.T().Fatalf("IK 插件未正确安装，已安装的插件: %s", string(body))
		}
	}

	// 通过测试分析器验证插件是否可用（必须验证成功才能继续）
	s.T().Logf("测试 IK 分析器是否可用...")
	for i := 0; i < 5; i++ {
		testAnalyzerResp, err := http.Post(esAddr+"/_analyze", "application/json",
			bytes.NewBufferString(`{"analyzer":"ik_max_word","text":"测试"}`))
		if err == nil && testAnalyzerResp.StatusCode == http.StatusOK {
			s.T().Logf("IK 分析器测试成功")
			testAnalyzerResp.Body.Close()
			break
		}
		if testAnalyzerResp != nil {
			body, _ := io.ReadAll(testAnalyzerResp.Body)
			testAnalyzerResp.Body.Close()
			if i == 4 {
				s.T().Fatalf("IK 分析器测试失败（已重试5次）: %s", string(body))
			}
			s.T().Logf("IK 分析器测试失败，重试中... (第 %d 次): %s", i+1, string(body))
		}
		time.Sleep(2 * time.Second)
	}

	// 初始化索引（创建 product_index 和 merchant_index）
	err = es.InitIndices(s.esClient)
	require.NoError(s.T(), err)
	s.T().Logf("索引初始化成功")

	// 初始化服务组件
	testLogger := logger.NewNopLogger()
	s.productRepo = repo.NewProductRepo(s.esClient, testLogger)
	s.merchantRepo = repo.NewMerchantRepo(s.esClient, testLogger)
	s.searchSvc = service.NewSearchService(s.productRepo, s.merchantRepo, testLogger)
}

// TearDownSuite 测试套件结束后执行
func (s *SearchIntegrationSuite) TearDownSuite() {
	ctx := context.Background()
	if s.esContainer != nil {
		_ = s.esContainer.Terminate(ctx)
	}
}

// SetupTest 每个测试前执行
func (s *SearchIntegrationSuite) SetupTest() {
	ctx := context.Background()

	// 清空索引数据（删除并重建索引）
	err := es.InitIndices(s.esClient)
	require.NoError(s.T(), err)

	// 等待索引创建完成
	time.Sleep(500 * time.Millisecond)

	// 插入测试数据
	s.insertTestData(ctx)
}

// insertTestData 插入测试数据
func (s *SearchIntegrationSuite) insertTestData(ctx context.Context) {
	// 插入商品数据
	products := []domain.ProductDocument{
		{
			ID:           1,
			Name:         "iPhone 15 Pro",
			Description:  "最新款 iPhone，A17 Pro 芯片",
			Price:        899900,
			Categories:   []string{"电子产品", "手机"},
			InStock:      true,
			MerchantID:   1001,
			MerchantName: "Apple 官方旗舰店",
		},
		{
			ID:           2,
			Name:         "iPhone 14",
			Description:  "iPhone 14，A15 芯片",
			Price:        599900,
			Categories:   []string{"电子产品", "手机"},
			InStock:      true,
			MerchantID:   1001,
			MerchantName: "Apple 官方旗舰店",
		},
		{
			ID:           3,
			Name:         "MacBook Pro",
			Description:  "MacBook Pro 14 英寸，M3 芯片",
			Price:        1499900,
			Categories:   []string{"电子产品", "电脑"},
			InStock:      true,
			MerchantID:   1001,
			MerchantName: "Apple 官方旗舰店",
		},
	}

	successCount, failedCount, errs := s.productRepo.BatchSyncProducts(ctx, products)
	require.Equal(s.T(), int64(3), successCount)
	require.Equal(s.T(), int64(0), failedCount)
	require.Empty(s.T(), errs)

	// 等待数据同步
	time.Sleep(1 * time.Second)
}

// ============================================================================
// 测试用例
// ============================================================================

func (s *SearchIntegrationSuite) TestSearchProducts() {
	ctx := context.Background()

	// 搜索商品
	req := &domain.SearchProductsReq{
		Keyword:  "iPhone",
		Page:     1,
		PageSize: 10,
	}

	resp, err := s.searchSvc.SearchProducts(ctx, req)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)

	// 输出详细的搜索结果
	s.T().Logf("=== 搜索结果详情 ===")
	s.T().Logf("关键词: %s", req.Keyword)
	s.T().Logf("总结果数: %d", resp.Total)
	s.T().Logf("当前页: %d, 每页: %d", resp.Page, resp.PageSize)
	s.T().Logf("实际返回结果数: %d", len(resp.Products))

	// 验证基本要求
	assert.GreaterOrEqual(s.T(), len(resp.Products), 2, "至少找到 2 个 iPhone")

	// 验证结果包含预期商品并输出详细信息
	productNames := make(map[string]bool)
	for i, p := range resp.Products {
		productNames[p.Name] = true
		s.T().Logf("  [%d] ID: %d, 名称: %s, 价格: %.2f元, 相关性分数: %.4f, 库存: %v",
			i+1, p.ID, p.Name, float64(p.Price)/100, p.Score, p.InStock)
	}

	// 验证包含预期商品
	assert.True(s.T(), productNames["iPhone 15 Pro"] || productNames["iPhone 14"],
		"应该找到 iPhone 15 Pro 或 iPhone 14")

	// 验证相关性分数（结果应该按相关性降序）
	if len(resp.Products) > 1 {
		for i := 1; i < len(resp.Products); i++ {
			assert.GreaterOrEqual(s.T(), resp.Products[i-1].Score, resp.Products[i].Score,
				"结果应该按相关性分数降序排列")
		}
	}

	// 验证分页信息
	assert.Equal(s.T(), req.Page, resp.Page)
	assert.Equal(s.T(), req.PageSize, resp.PageSize)
}

func (s *SearchIntegrationSuite) TestSearchProductsWithFilter() {
	ctx := context.Background()

	// 按价格区间筛选
	req := &domain.SearchProductsReq{
		Keyword:  "iPhone",
		MinPrice: 600000, // 6000元（单位：分）
		MaxPrice: 900000, // 9000元
		Page:     1,
		PageSize: 10,
	}

	resp, err := s.searchSvc.SearchProducts(ctx, req)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)

	s.T().Logf("=== 价格筛选测试 ===")
	s.T().Logf("关键词: %s", req.Keyword)
	s.T().Logf("价格区间: %.2f元 - %.2f元", float64(req.MinPrice)/100, float64(req.MaxPrice)/100)
	s.T().Logf("找到商品数: %d", len(resp.Products))

	// 验证所有结果都在价格区间内
	for i, p := range resp.Products {
		s.T().Logf("  [%d] %s: %.2f元", i+1, p.Name, float64(p.Price)/100)
		assert.GreaterOrEqual(s.T(), p.Price, req.MinPrice,
			"商品价格 %d 应该 >= %d", p.Price, req.MinPrice)
		assert.LessOrEqual(s.T(), p.Price, req.MaxPrice,
			"商品价格 %d 应该 <= %d", p.Price, req.MaxPrice)
	}

	s.T().Logf("价格筛选验证通过：所有商品都在指定价格区间内")
}

func (s *SearchIntegrationSuite) TestSearchProductSuggest() {
	ctx := context.Background()

	keyword := "iPh"
	limit := int64(10)

	// 搜索建议
	suggestions, err := s.searchSvc.SearchProductSuggest(ctx, keyword, limit)
	require.NoError(s.T(), err)
	assert.NotEmpty(s.T(), suggestions, "应该返回搜索建议")

	s.T().Logf("=== 搜索建议测试 ===")
	s.T().Logf("输入关键词: %s", keyword)
	s.T().Logf("返回建议数: %d", len(suggestions))
	s.T().Logf("建议列表:")

	// 验证建议包含关键词并输出详情
	for i, sug := range suggestions {
		s.T().Logf("  [%d] 关键词: %s, 来源: %s, 数量: %d",
			i+1, sug.Keyword, sug.Source, sug.Count)
		assert.Contains(s.T(), sug.Keyword, keyword,
			"建议关键词 '%s' 应该包含输入 '%s'", sug.Keyword, keyword)
	}

	// 验证数量限制
	assert.LessOrEqual(s.T(), len(suggestions), int(limit),
		"建议数量不应该超过限制 %d", limit)
}

func (s *SearchIntegrationSuite) TestSyncProduct() {
	ctx := context.Background()

	// 同步新商品
	doc := &domain.ProductDocument{
		ID:           100,
		Name:         "iPad Pro",
		Description:  "iPad Pro 12.9 英寸",
		Price:        799900,
		Categories:   []string{"电子产品", "平板"},
		InStock:      true,
		MerchantID:   1001,
		MerchantName: "Apple 官方旗舰店",
	}

	s.T().Logf("=== 商品同步测试 ===")
	s.T().Logf("同步商品: ID=%d, 名称=%s, 价格=%.2f元", doc.ID, doc.Name, float64(doc.Price)/100)

	err := s.productRepo.SyncProduct(ctx, "CREATE", doc)
	require.NoError(s.T(), err)
	s.T().Logf("商品同步成功")

	// 等待索引更新
	time.Sleep(500 * time.Millisecond)

	// 搜索验证
	req := &domain.SearchProductsReq{
		Keyword:  "iPad",
		Page:     1,
		PageSize: 10,
	}

	resp, err := s.searchSvc.SearchProducts(ctx, req)
	require.NoError(s.T(), err)
	assert.NotEmpty(s.T(), resp.Products, "应该能找到新同步的商品")

	s.T().Logf("搜索结果数: %d", len(resp.Products))

	// 验证找到了新商品并输出详情
	found := false
	for i, p := range resp.Products {
		s.T().Logf("  [%d] ID: %d, 名称: %s", i+1, p.ID, p.Name)
		if p.ID == 100 && p.Name == "iPad Pro" {
			found = true
			s.T().Logf("✓ 成功找到新同步的商品: %s (ID: %d)", p.Name, p.ID)
			// 验证商品信息完整性
			assert.Equal(s.T(), doc.Price, p.Price)
			assert.Equal(s.T(), doc.MerchantID, p.MerchantID)
			assert.Equal(s.T(), doc.MerchantName, p.MerchantName)
		}
	}
	assert.True(s.T(), found, "应该找到新同步的商品 (ID: 100, 名称: iPad Pro)")
}

func (s *SearchIntegrationSuite) TestDeleteProduct() {
	ctx := context.Background()

	productID := int64(1)
	s.T().Logf("=== 商品删除测试 ===")
	s.T().Logf("删除商品 ID: %d", productID)

	// 先验证商品存在
	beforeDeleteReq := &domain.SearchProductsReq{
		Keyword:  "iPhone 15 Pro",
		Page:     1,
		PageSize: 10,
	}
	beforeResp, err := s.searchSvc.SearchProducts(ctx, beforeDeleteReq)
	require.NoError(s.T(), err)

	foundBefore := false
	for _, p := range beforeResp.Products {
		if p.ID == productID {
			foundBefore = true
			s.T().Logf("删除前：找到商品 ID=%d, 名称=%s", p.ID, p.Name)
			break
		}
	}
	assert.True(s.T(), foundBefore, "删除前应该能找到商品 ID=%d", productID)

	// 删除商品
	err = s.productRepo.DeleteProduct(ctx, productID)
	require.NoError(s.T(), err)
	s.T().Logf("商品删除成功")

	// 等待索引更新
	time.Sleep(500 * time.Millisecond)

	// 搜索验证（应该找不到被删除的商品）
	req := &domain.SearchProductsReq{
		Keyword:  "iPhone 15 Pro",
		Page:     1,
		PageSize: 10,
	}

	resp, err := s.searchSvc.SearchProducts(ctx, req)
	require.NoError(s.T(), err)

	s.T().Logf("删除后搜索结果数: %d", len(resp.Products))

	// 验证被删除的商品不在结果中
	foundAfter := false
	for i, p := range resp.Products {
		s.T().Logf("  [%d] ID: %d, 名称: %s", i+1, p.ID, p.Name)
		if p.ID == productID {
			foundAfter = true
			s.T().Logf("✗ 错误：被删除的商品仍然出现在搜索结果中!")
		}
		assert.NotEqual(s.T(), productID, p.ID,
			"被删除的商品 ID=%d 不应该出现在搜索结果中", productID)
	}
	assert.False(s.T(), foundAfter, "删除后不应该找到商品 ID=%d", productID)
	s.T().Logf("✓ 验证通过：被删除的商品已从搜索结果中移除")
}

func (s *SearchIntegrationSuite) TestGetAggregations() {
	ctx := context.Background()

	// 使用更精确的关键词或空关键词来获取聚合
	req := &domain.SearchProductsReq{
		Keyword:  "iPhone", // 使用更精确的关键词确保能匹配到数据
		Page:     1,
		PageSize: 10,
	}

	resp, err := s.searchSvc.GetAggregations(ctx, req)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)

	s.T().Logf("=== 聚合查询测试 ===")
	s.T().Logf("关键词: %s", req.Keyword)

	// 验证聚合结果
	assert.NotEmpty(s.T(), resp.Categories, "应该有分类聚合")
	assert.NotEmpty(s.T(), resp.PriceRanges, "应该有价格区间聚合")

	// 输出分类聚合详情
	s.T().Logf("分类聚合 (%d 个):", len(resp.Categories))
	categoryMap := make(map[string]bool)
	for i, agg := range resp.Categories {
		categoryMap[agg.Category] = true
		s.T().Logf("  [%d] 分类: %s, 商品数: %d", i+1, agg.Category, agg.Count)
	}
	assert.True(s.T(), categoryMap["电子产品"], "应该包含'电子产品'分类")

	// 输出价格区间聚合详情
	s.T().Logf("价格区间聚合 (%d 个):", len(resp.PriceRanges))
	for i, agg := range resp.PriceRanges {
		s.T().Logf("  [%d] %s: %.2f元 - %.2f元, 商品数: %d",
			i+1, agg.Label, float64(agg.MinPrice)/100, float64(agg.MaxPrice)/100, agg.Count)
		// 验证价格区间有效
		if agg.MaxPrice > 0 {
			assert.Less(s.T(), agg.MinPrice, agg.MaxPrice,
				"价格区间应该有效: %d < %d", agg.MinPrice, agg.MaxPrice)
		}
	}
}
