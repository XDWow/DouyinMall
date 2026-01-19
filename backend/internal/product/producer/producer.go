package producer

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/internal/product/repo/dao"
	"github.com/XDWow/DouyinMall/backend/internal/search/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/go-mysql-org/go-mysql/canal"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
	"github.com/go-mysql-org/go-mysql/schema"
)

type Producer interface {
	Start(ctx context.Context) error
}

const (
	// PositionKey 用于标识商品表的 binlog position
	PositionKey = "product"
	TopicName   = "sync_product_event"
	TableName   = "product"
)

// CanalProducer Canal 生产者，监听 MySQL binlog 并发送到 Kafka
type CanalProducer struct {
	canal         *canal.Canal
	kafkaProducer sarama.SyncProducer
	logger        logger.LoggerV1
	positionDao   dao.PositionDao
}

func NewCanalProducer(
	cfg *canal.Config,
	kafkaProducer sarama.SyncProducer,
	logger logger.LoggerV1,
	positionDao dao.PositionDao,
) (*CanalProducer, error) {
	c, err := canal.NewCanal(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建 Canal 失败: %w", err)
	}

	cp := &CanalProducer{
		canal:         c,
		kafkaProducer: kafkaProducer,
		logger:        logger,
		positionDao:   positionDao,
	}

	c.SetEventHandler(cp)

	return cp, nil
}

func (p *CanalProducer) Start(ctx context.Context) error {
	// 加载上次保存的 position
	pos, err := p.positionDao.LoadPosition(ctx, PositionKey)
	if err != nil {
		p.logger.Warn("加载 position 失败，从最新位置开始",
			logger.Error(err))
	} else if pos.Name != "" {
		p.logger.Info("从上次保存的 position 继续",
			logger.String("binlog_file", pos.Name),
			logger.Int32("binlog_pos", int32(pos.Pos)))
	}

	go func() {
		if pos.Name != "" {
			if err := p.canal.RunFrom(pos); err != nil {
				p.logger.Error("Canal 运行失败",
					logger.Error(err))
			}
		} else {
			if err := p.canal.Run(); err != nil {
				p.logger.Error("Canal 运行失败",
					logger.Error(err))
			}
		}
	}()

	p.logger.Info("Canal Producer 已启动")
	return nil
}

// OnRow 处理行变更事件
func (p *CanalProducer) OnRow(e *canal.RowsEvent) error {
	// 只处理 product 表
	if e.Table.Name != TableName {
		return nil
	}

	ctx := context.Background()

	var events []domain.SyncEvent

	for i, row := range e.Rows {
		var event domain.SyncEvent
		var err error

		switch e.Action {
		case canal.InsertAction:
			rowMap := p.rowToMap(e.Table.Columns, row)
			event, err = parseRowToSyncEvent(rowMap, domain.EventActionCreate)
			if err != nil {
				// 解析失败，返回错误让 Canal 重试整个 RowsEvent，避免数据不一致
				return fmt.Errorf("解析 INSERT 行数据失败 (row_index: %d): %w", i, err)
			}
		case canal.UpdateAction:
			rowMap := p.rowToMap(e.Table.Columns, row)
			event, err = parseRowToSyncEvent(rowMap, domain.EventActionUpdate)
			if err != nil {
				return fmt.Errorf("解析 UPDATE 行数据失败 (row_index: %d): %w", i, err)
			}
		case canal.DeleteAction:
			// DELETE 操作，只需要 ID
			id, err := p.extractIDFromRow(e.Table.Columns, row)
			if err != nil {
				return fmt.Errorf("提取 DELETE ID 失败 (row_index: %d): %w", i, err)
			}
			event = domain.SyncEvent{
				Type:   domain.EventTypeProduct,
				Action: domain.EventActionDelete,
				ID:     id,
			}
		default:
			p.logger.Warn("不支持的操作类型",
				logger.String("action", e.Action),
				logger.Int("row_index", i))
			continue
		}

		events = append(events, event)
	}

	// 批量发送
	if len(events) > 0 {
		if err := p.sendBatchToKafka(ctx, events); err != nil {
			p.logger.Error("批量发送事件到 Kafka 失败",
				logger.Error(err),
				logger.Int("event_count", len(events)))
			return fmt.Errorf("批量发送事件到 Kafka 失败: %w", err)
		}
	}

	return nil
}

// OnPosSynced position 同步回调，保存 position
func (p *CanalProducer) OnPosSynced(header *replication.EventHeader, pos mysql.Position, set mysql.GTIDSet, force bool) error {
	ctx := context.Background()
	if err := p.positionDao.SavePosition(ctx, PositionKey, pos); err != nil {
		p.logger.Error("保存 position 失败",
			logger.Error(err),
			logger.String("binlog_file", pos.Name),
			logger.Int32("binlog_pos", int32(pos.Pos)))
		return err
	}
	return nil
}

// OnTableChanged 表结构变更
func (p *CanalProducer) OnTableChanged(header *replication.EventHeader, schema string, table string) error {
	p.logger.Info("表结构变更",
		logger.String("schema", schema),
		logger.String("table", table))
	return nil
}

// OnDDL DDL 语句
func (p *CanalProducer) OnDDL(header *replication.EventHeader, nextPos mysql.Position, queryEvent *replication.QueryEvent) error {
	p.logger.Info("DDL 语句",
		logger.String("binlog_file", nextPos.Name),
		logger.Int32("binlog_pos", int32(nextPos.Pos)),
		logger.String("query", string(queryEvent.Query)))
	return nil
}

// OnGTID GTID 事件
func (p *CanalProducer) OnGTID(header *replication.EventHeader, gtidEvent mysql.BinlogGTIDEvent) error {
	return nil
}

// OnXID XID 事件（事务提交）
func (p *CanalProducer) OnXID(header *replication.EventHeader, pos mysql.Position) error {
	return nil
}

// OnRotate binlog rotate 事件
func (p *CanalProducer) OnRotate(header *replication.EventHeader, r *replication.RotateEvent) error {
	p.logger.Info("Binlog rotate",
		logger.String("next_binlog", string(r.NextLogName)),
		logger.Int64("position", int64(r.Position)))
	return nil
}

// OnRowsQueryEvent RowsQuery 事件
func (p *CanalProducer) OnRowsQueryEvent(event *replication.RowsQueryEvent) error {
	return nil
}

// String 返回字符串表示
func (p *CanalProducer) String() string {
	return "CanalProducer"
}

// rowToMap 将行数据转换为 map
func (p *CanalProducer) rowToMap(columns []schema.TableColumn, row []interface{}) map[string]interface{} {
	rowMap := make(map[string]interface{}, len(columns))
	for i, col := range columns {
		if i < len(row) {
			rowMap[col.Name] = row[i]
		}
	}
	return rowMap
}

// extractIDFromRow 从行数据中提取 ID
func (p *CanalProducer) extractIDFromRow(columns []schema.TableColumn, row []interface{}) (int64, error) {
	// 查找 id 列的索引
	for i, col := range columns {
		if col.Name == "id" && i < len(row) {
			// 尝试类型转换
			switch val := row[i].(type) {
			case int64:
				return val, nil
			case int32:
				return int64(val), nil
			case int:
				return int64(val), nil
			case string:
				// 字符串转 int64
				id, err := strconv.ParseInt(val, 10, 64)
				if err != nil {
					return 0, fmt.Errorf("解析 ID 失败: %w", err)
				}
				return id, nil
			default:
				return 0, fmt.Errorf("ID 类型不支持: %T", val)
			}
		}
	}
	return 0, fmt.Errorf("未找到 id 列")
}

// sendBatchToKafka 批量发送事件到 Kafka
// Producer 已配置 Retry.Max = 3，会自动重试可重试错误（无脑重试3次）
// 如果 SendMessages 返回错误，说明 Producer 的重试已经用尽（所有错误都不可重试，或重试后仍失败）
// 此时返回错误，让 Canal 重试整个 binlog 事件
func (p *CanalProducer) sendBatchToKafka(ctx context.Context, events []domain.SyncEvent) error {
	if len(events) == 0 {
		return nil
	}

	messages := make([]*sarama.ProducerMessage, 0, len(events))
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("序列化事件失败 (ID: %d): %w", event.ID, err)
		}

		messages = append(messages, &sarama.ProducerMessage{
			Topic: TopicName,
			Key:   sarama.StringEncoder(fmt.Sprintf("%s:%d", event.Type, event.ID)), // 保证同一个实体的变更的顺序性
			Value: sarama.ByteEncoder(data),
		})
	}

	if len(messages) == 0 {
		return fmt.Errorf("没有有效的消息可发送")
	}

	err := p.kafkaProducer.SendMessages(messages)
	if err != nil {
		if producerErrors, ok := err.(sarama.ProducerErrors); ok {
			failedCount := len(producerErrors)
			totalCount := len(messages)
			p.logger.Error("批量发送部分失败（Producer 已重试）",
				logger.Error(err),
				logger.Int("total_count", totalCount),
				logger.Int("failed_count", failedCount),
				logger.Int("success_count", totalCount-failedCount))
			for i, producerError := range producerErrors {
				if i < 10 {
					p.logger.Error("发送失败的消息详情",
						logger.Error(producerError.Err),
						logger.String("topic", producerError.Msg.Topic),
						logger.String("key", string(producerError.Msg.Key.(sarama.StringEncoder))))
				}
			}
			return fmt.Errorf("批量发送部分失败 (Producer已重试): %d/%d 失败: %w", failedCount, totalCount, err)
		}
		p.logger.Error("批量发送完全失败（Producer 已重试）",
			logger.Error(err),
			logger.Int("total_count", len(messages)))
		return fmt.Errorf("批量发送消息到 Kafka 完全失败 (Producer已重试): %w", err)
	}

	p.logger.Debug("批量事件已发送到 Kafka",
		logger.String("topic", TopicName),
		logger.Int("event_count", len(messages)))

	return nil
}
