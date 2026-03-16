// Package knowledge 提供知识库初始化工具。
//
// # 分片策略（Chunking）
//
// 每篇 RawDoc 按段落（空行分隔）切分，超过 chunkMaxRunes 字符的段落再按句号、换行符做细粒度切分。
// 目标 chunk 大小约 200 字（可调），保证每个向量语义聚焦。
//
// # 写入流程
//
//  1. 对每个 chunk 调用 Embedder 获取 float32 向量（批量，减少 API 调用）
//  2. 向量 + vector_id 写入 Milvus（domain.CollectionKnowledge）
//  3. KnowledgeItemDO（含 content 全文、title、category、vector_id）写入 MySQL
//  4. 序列化为 JSON 写入 Redis（key = agent:knowledge:<vector_id>，无 TTL）
//
// # 使用方式
//
//	indexer := knowledge.NewIndexer(embedder, vectorStore, db, redisClient, logger)
//	err := indexer.Run(ctx)
package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentcache "github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	agentdb "github.com/XDWow/DouyinMall/backend/internal/agent/infra/db"
	pkgai "github.com/XDWow/DouyinMall/backend/pkg/ai"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/google/uuid"
	sdkclient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	chunkMaxRunes   = 200                // 每个 chunk 最大字符数（rune）
	knowledgeKeyPfx = "agent:knowledge:" // Redis key 前缀，与 milvusKnowledge_repo 保持一致
	embedBatchSize  = 16                 // 每批 embedding 请求包含的 chunk 数
)

// chunk 内部表示一个已分片的知识片段
type chunk struct {
	docID    string // 所属 RawDoc.ID
	title    string
	category string
	content  string // 实际文本内容（已切分）
}

// Indexer 负责把 Docs 中的原始知识库分片、向量化、写入存储。
type Indexer struct {
	embedder    pkgai.Embedder
	vectorStore sdkclient.Client
	db          *gorm.DB
	redis       agentcache.AgentCache
	logger      logger.LoggerV1
}

// NewIndexer 构造 Indexer。各依赖均通过接口注入，便于测试替换。
func NewIndexer(
	embedder pkgai.Embedder,
	vectorStore sdkclient.Client,
	db *gorm.DB,
	redis agentcache.AgentCache,
	l logger.LoggerV1,
) *Indexer {
	return &Indexer{
		embedder:    embedder,
		vectorStore: vectorStore,
		db:          db,
		redis:       redis,
		logger:      l,
	}
}

// Run 对 Docs 执行一次完整索引：分片 → embedding → 写 Milvus/MySQL/Redis。
// 幂等：MySQL 使用 ON CONFLICT DO UPDATE，Redis 直接覆盖写，Milvus 按 vector_id 去重（需 Milvus 侧配置）。
func (idx *Indexer) Run(ctx context.Context) error {
	return idx.RunDocs(ctx, Docs)
}

// RunDocs 允许外部传入自定义文档列表，方便单元测试。
func (idx *Indexer) RunDocs(ctx context.Context, docs []RawDoc) error {
	idx.logger.Info("知识库索引开始", logger.Int("doc_count", len(docs)))

	// Step 1: 分片
	chunks := splitDocs(docs)
	idx.logger.Info("分片完成", logger.Int("chunk_count", len(chunks)))

	// Step 2: 批量 embedding
	vectors, err := idx.embedChunks(ctx, chunks)
	if err != nil {
		return fmt.Errorf("embedding 失败: %w", err)
	}

	// Step 3: 写存储（Milvus / MySQL / Redis）
	if err := idx.persist(ctx, chunks, vectors); err != nil {
		return fmt.Errorf("持久化失败: %w", err)
	}

	idx.logger.Info("知识库索引完成", logger.Int("chunk_count", len(chunks)))
	return nil
}

// ─────────────────────────────────────────────────────────
// Step 1: 分片
// ─────────────────────────────────────────────────────────

// splitDocs 将所有原始文档分片。
// 策略：先按空行切段落，每段落如超过 chunkMaxRunes 则按标点进一步细分。
func splitDocs(docs []RawDoc) []chunk {
	var result []chunk
	for _, doc := range docs {
		paragraphs := splitParagraphs(doc.Content)
		for _, para := range paragraphs {
			for _, c := range splitParagraph(doc.ID, doc.Title, doc.Category, para) {
				result = append(result, c)
			}
		}
	}
	return result
}

// splitParagraphs 按空行切段落（去除空白段）。
func splitParagraphs(content string) []string {
	raw := strings.Split(content, "\n")
	var paras []string
	var current strings.Builder
	for _, line := range raw {
		if strings.TrimSpace(line) == "" {
			if current.Len() > 0 {
				paras = append(paras, strings.TrimSpace(current.String()))
				current.Reset()
			}
			continue
		}
		if current.Len() > 0 {
			current.WriteByte('\n')
		}
		current.WriteString(line)
	}
	if current.Len() > 0 {
		paras = append(paras, strings.TrimSpace(current.String()))
	}
	return paras
}

// splitParagraph 对单个段落做进一步分片：超长段落按句号/问号/感叹号拆句。
func splitParagraph(docID, title, category, para string) []chunk {
	if utf8.RuneCountInString(para) <= chunkMaxRunes {
		return []chunk{{docID: docID, title: title, category: category, content: para}}
	}

	// 按中英文句末标点拆句
	sentences := splitSentences(para)

	var chunks []chunk
	var buf strings.Builder
	for _, sent := range sentences {
		sentRunes := utf8.RuneCountInString(sent)
		if buf.Len() > 0 && utf8.RuneCountInString(buf.String())+sentRunes > chunkMaxRunes {
			// 当前 buffer 已满，flush
			chunks = append(chunks, chunk{
				docID:    docID,
				title:    title,
				category: category,
				content:  strings.TrimSpace(buf.String()),
			})
			buf.Reset()
		}
		// 超长单句直接作为独立 chunk（截断会破坏语义，保留完整句）
		if sentRunes > chunkMaxRunes {
			if buf.Len() > 0 {
				chunks = append(chunks, chunk{
					docID:    docID,
					title:    title,
					category: category,
					content:  strings.TrimSpace(buf.String()),
				})
				buf.Reset()
			}
			chunks = append(chunks, chunk{
				docID:    docID,
				title:    title,
				category: category,
				content:  strings.TrimSpace(sent),
			})
			continue
		}
		if buf.Len() > 0 {
			buf.WriteByte(' ')
		}
		buf.WriteString(sent)
	}
	if buf.Len() > 0 {
		chunks = append(chunks, chunk{
			docID:    docID,
			title:    title,
			category: category,
			content:  strings.TrimSpace(buf.String()),
		})
	}
	return chunks
}

// splitSentences 按中英文句末标点切句（保留标点）。
func splitSentences(text string) []string {
	var sentences []string
	var buf strings.Builder
	for _, r := range text {
		buf.WriteRune(r)
		if r == '。' || r == '！' || r == '？' || r == '.' || r == '!' || r == '?' || r == '\n' {
			s := strings.TrimSpace(buf.String())
			if s != "" {
				sentences = append(sentences, s)
			}
			buf.Reset()
		}
	}
	if buf.Len() > 0 {
		if s := strings.TrimSpace(buf.String()); s != "" {
			sentences = append(sentences, s)
		}
	}
	return sentences
}

// ─────────────────────────────────────────────────────────
// Step 2: 批量 embedding
// ─────────────────────────────────────────────────────────

func (idx *Indexer) embedChunks(ctx context.Context, chunks []chunk) ([][]float32, error) {
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		// 用 "标题: 内容" 格式增强语义
		texts[i] = c.title + ": " + c.content
	}

	vectors := make([][]float32, len(texts))
	for start := 0; start < len(texts); start += embedBatchSize {
		end := start + embedBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch, err := idx.embedder.Embed(ctx, texts[start:end])
		if err != nil {
			return nil, fmt.Errorf("batch [%d:%d] embed 失败: %w", start, end, err)
		}
		copy(vectors[start:], batch)
		idx.logger.Info("embedding 批次完成",
			logger.Int("from", start),
			logger.Int("to", end),
		)
		// 简单限速：避免 API 过载（生产环境可换成 ratelimit.Limiter）
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return vectors, nil
}

// ─────────────────────────────────────────────────────────
// Step 3: 持久化
// ─────────────────────────────────────────────────────────

// knowledgeRedisVal 存入 Redis 的结构（与 milvusKnowledge_repo.go 中 domain.KnowledgeRef JSON 反序列化保持一致）
type knowledgeRedisVal struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Content   string  `json:"content"` // chunk 完整内容
	Category  string  `json:"category"`
	Relevance float32 `json:"relevance"` // 索引时填 0，检索时由 repo 填真实分数
}

func (idx *Indexer) persist(ctx context.Context, chunks []chunk, vectors [][]float32) error {
	for i, c := range chunks {
		vectorID := uuid.NewString() // 每个 chunk 独立 ID

		// 3a: Milvus
		idInsertCol := entity.NewColumnVarChar("id", []string{vectorID})
		vecInsertCol := entity.NewColumnFloatVector("vector", len(vectors[i]), [][]float32{vectors[i]})
		if _, err := idx.vectorStore.Insert(ctx, domain.CollectionKnowledge, "", idInsertCol, vecInsertCol); err != nil {
			return fmt.Errorf("Milvus Insert chunk[%d] docID=%s: %w", i, c.docID, err)
		}

		// 3b: MySQL（ON CONFLICT DO UPDATE 实现幂等）
		item := agentdb.KnowledgeItem{
			Title:    c.title,
			Content:  c.content, // 存分片后的文本
			Category: c.category,
			VectorID: vectorID,
			Status:   1,
		}
		if err := idx.db.WithContext(ctx).
			Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "vector_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"title", "content", "category", "status"}),
			}).
			Create(&item).Error; err != nil {
			return fmt.Errorf("MySQL Insert chunk[%d] docID=%s: %w", i, c.docID, err)
		}

		// 3c: Redis（无 TTL，知识库稳定，命中直接用）
		val := knowledgeRedisVal{
			ID:       vectorID,
			Title:    c.title,
			Content:  c.content,
			Category: c.category,
		}
		b, err := json.Marshal(val)
		if err != nil {
			return fmt.Errorf("Redis marshal chunk[%d]: %w", i, err)
		}
		if err := idx.redis.Set(ctx, knowledgeKeyPfx+vectorID, string(b), 0); err != nil {
			// Redis 失败不中断，下次检索会从 MySQL 回查并写回
			idx.logger.Warn("Redis 写入失败，可接受降级",
				logger.String("vector_id", vectorID),
				logger.Error(err),
			)
		}

		idx.logger.Info("chunk 已索引",
			logger.Int("index", i),
			logger.String("doc_id", c.docID),
			logger.String("vector_id", vectorID),
			logger.Int("content_runes", utf8.RuneCountInString(c.content)),
		)
	}
	return nil
}
