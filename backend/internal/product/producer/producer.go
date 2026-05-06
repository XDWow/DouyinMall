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
	PositionKey = "product"
	TopicName   = "sync_product_event"
	TableName   = "product"
)

type CanalProducer struct {
	canal         *canal.Canal
	kafkaProducer sarama.SyncProducer
	logger        logger.LoggerV1
	positionDao   dao.PositionDao
}

func NewCanalProducer(
	cfg *canal.Config,
	kafkaProducer sarama.SyncProducer,
	l logger.LoggerV1,
	positionDao dao.PositionDao,
) (*CanalProducer, error) {
	c, err := canal.NewCanal(cfg)
	if err != nil {
		return nil, fmt.Errorf("init canal: %w", err)
	}

	cp := &CanalProducer{
		canal:         c,
		kafkaProducer: kafkaProducer,
		logger:        l,
		positionDao:   positionDao,
	}
	c.SetEventHandler(cp)
	return cp, nil
}

func (p *CanalProducer) Start(ctx context.Context) error {
	pos, err := p.positionDao.LoadPosition(ctx, PositionKey)
	if err != nil {
		p.logger.Warn("load binlog position failed, start from current position", logger.Error(err))
	} else if pos.Name != "" {
		p.logger.Info("resume from saved binlog position",
			logger.String("binlog_file", pos.Name),
			logger.Int32("binlog_pos", int32(pos.Pos)))
	}

	go func() {
		if pos.Name != "" {
			if err := p.canal.RunFrom(pos); err != nil {
				p.logger.Error("run canal from saved position failed", logger.Error(err))
			}
			return
		}
		if err := p.canal.Run(); err != nil {
			p.logger.Error("run canal failed", logger.Error(err))
		}
	}()

	p.logger.Info("Canal producer started")
	return nil
}

func (p *CanalProducer) OnRow(e *canal.RowsEvent) error {
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
				return fmt.Errorf("parse INSERT row (row_index: %d): %w", i, err)
			}
		case canal.UpdateAction:
			rowMap := p.rowToMap(e.Table.Columns, row)
			event, err = parseRowToSyncEvent(rowMap, domain.EventActionUpdate)
			if err != nil {
				return fmt.Errorf("parse UPDATE row (row_index: %d): %w", i, err)
			}
		case canal.DeleteAction:
			id, err := p.extractIDFromRow(e.Table.Columns, row)
			if err != nil {
				return fmt.Errorf("extract DELETE id (row_index: %d): %w", i, err)
			}
			event = domain.SyncEvent{
				Type:   domain.EventTypeProduct,
				Action: domain.EventActionDelete,
				ID:     id,
			}
		default:
			p.logger.Warn("unsupported canal action",
				logger.String("action", e.Action),
				logger.Int("row_index", i))
			continue
		}

		events = append(events, event)
	}

	if len(events) > 0 {
		if err := p.sendBatchToKafka(ctx, events); err != nil {
			p.logger.Error("send product sync events to kafka failed",
				logger.Error(err),
				logger.Int("event_count", len(events)))
			return fmt.Errorf("send product sync events to kafka failed: %w", err)
		}
	}

	return nil
}

func (p *CanalProducer) OnPosSynced(header *replication.EventHeader, pos mysql.Position, set mysql.GTIDSet, force bool) error {
	ctx := context.Background()
	if err := p.positionDao.SavePosition(ctx, PositionKey, pos); err != nil {
		p.logger.Error("save binlog position failed",
			logger.Error(err),
			logger.String("binlog_file", pos.Name),
			logger.Int32("binlog_pos", int32(pos.Pos)))
		return err
	}
	return nil
}

func (p *CanalProducer) OnTableChanged(header *replication.EventHeader, schema string, table string) error {
	p.logger.Info("table changed",
		logger.String("schema", schema),
		logger.String("table", table))
	return nil
}

func (p *CanalProducer) OnDDL(header *replication.EventHeader, nextPos mysql.Position, queryEvent *replication.QueryEvent) error {
	p.logger.Info("ddl event",
		logger.String("binlog_file", nextPos.Name),
		logger.Int32("binlog_pos", int32(nextPos.Pos)),
		logger.String("query", string(queryEvent.Query)))
	return nil
}

func (p *CanalProducer) OnGTID(header *replication.EventHeader, gtidEvent mysql.BinlogGTIDEvent) error {
	return nil
}

func (p *CanalProducer) OnXID(header *replication.EventHeader, pos mysql.Position) error {
	return nil
}

func (p *CanalProducer) OnRotate(header *replication.EventHeader, r *replication.RotateEvent) error {
	p.logger.Info("binlog rotate",
		logger.String("next_binlog", string(r.NextLogName)),
		logger.Int64("position", int64(r.Position)))
	return nil
}

func (p *CanalProducer) OnRowsQueryEvent(event *replication.RowsQueryEvent) error {
	return nil
}

func (p *CanalProducer) String() string {
	return "CanalProducer"
}

func (p *CanalProducer) rowToMap(columns []schema.TableColumn, row []interface{}) map[string]interface{} {
	rowMap := make(map[string]interface{}, len(columns))
	for i, col := range columns {
		if i < len(row) {
			rowMap[col.Name] = row[i]
		}
	}
	return rowMap
}

func (p *CanalProducer) extractIDFromRow(columns []schema.TableColumn, row []interface{}) (int64, error) {
	for i, col := range columns {
		if col.Name == "id" && i < len(row) {
			switch val := row[i].(type) {
			case int64:
				return val, nil
			case int32:
				return int64(val), nil
			case int:
				return int64(val), nil
			case string:
				id, err := strconv.ParseInt(val, 10, 64)
				if err != nil {
					return 0, fmt.Errorf("parse string id failed: %w", err)
				}
				return id, nil
			default:
				return 0, fmt.Errorf("unsupported id type: %T", val)
			}
		}
	}
	return 0, fmt.Errorf("id column not found")
}

func (p *CanalProducer) sendBatchToKafka(ctx context.Context, events []domain.SyncEvent) error {
	if len(events) == 0 {
		return nil
	}

	messages := make([]*sarama.ProducerMessage, 0, len(events))
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshal sync event failed (ID: %d): %w", event.ID, err)
		}

		messages = append(messages, &sarama.ProducerMessage{
			Topic: TopicName,
			Key:   sarama.StringEncoder(fmt.Sprintf("%s:%d", event.Type, event.ID)),
			Value: sarama.ByteEncoder(data),
		})
	}

	if len(messages) == 0 {
		return fmt.Errorf("no kafka messages built")
	}

	err := p.kafkaProducer.SendMessages(messages)
	if err != nil {
		if producerErrors, ok := err.(sarama.ProducerErrors); ok {
			failedCount := len(producerErrors)
			totalCount := len(messages)
			p.logger.Error("kafka partial send failure",
				logger.Error(err),
				logger.Int("total_count", totalCount),
				logger.Int("failed_count", failedCount),
				logger.Int("success_count", totalCount-failedCount))
			for i, producerError := range producerErrors {
				if i >= 10 {
					break
				}
				p.logger.Error("kafka message send failure",
					logger.Error(producerError.Err),
					logger.String("topic", producerError.Msg.Topic),
					logger.String("key", string(producerError.Msg.Key.(sarama.StringEncoder))))
			}
			return fmt.Errorf("kafka partial send failure (%d/%d): %w", failedCount, totalCount, err)
		}

		p.logger.Error("kafka send failed",
			logger.Error(err),
			logger.Int("total_count", len(messages)))
		return fmt.Errorf("kafka send failed: %w", err)
	}

	p.logger.Debug("sent product sync events to kafka",
		logger.String("topic", TopicName),
		logger.Int("event_count", len(messages)))
	return nil
}
