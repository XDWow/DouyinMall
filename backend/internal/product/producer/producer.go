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
	// PositionKey 鐢ㄤ簬鏍囪瘑鍟嗗搧琛ㄧ殑 binlog position
	PositionKey = "product"
	TopicName   = "sync_product_event"
	TableName   = "product"
)

// CanalProducer Canal 鐢熶骇鑰咃紝鐩戝惉 MySQL binlog 骞跺彂閫佸埌 Kafka
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
		return nil, fmt.Errorf("鍒涘缓 Canal 澶辫触: %w", err)
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
	// 鍔犺浇涓婃淇濆瓨鐨?position
	pos, err := p.positionDao.LoadPosition(ctx, PositionKey)
	if err != nil {
		p.logger.Warn("鍔犺浇 position 澶辫触锛屼粠鏈€鏂颁綅缃紑濮?,
			logger.Error(err))
	} else if pos.Name != "" {
		p.logger.Info("浠庝笂娆′繚瀛樼殑 position 缁х画",
			logger.String("binlog_file", pos.Name),
			logger.Int32("binlog_pos", int32(pos.Pos)))
	}

	go func() {
		if pos.Name != "" {
			if err := p.canal.RunFrom(pos); err != nil {
				p.logger.Error("Canal 杩愯澶辫触",
					logger.Error(err))
			}
		} else {
			if err := p.canal.Run(); err != nil {
				p.logger.Error("Canal 杩愯澶辫触",
					logger.Error(err))
			}
		}
	}()

	p.logger.Info("Canal Producer 宸插惎鍔?)
	return nil
}

// OnRow 澶勭悊琛屽彉鏇翠簨浠?
func (p *CanalProducer) OnRow(e *canal.RowsEvent) error {
	// 鍙鐞?product 琛?
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
				// 瑙ｆ瀽澶辫触锛岃繑鍥為敊璇 Canal 閲嶈瘯鏁翠釜 RowsEvent锛岄伩鍏嶆暟鎹笉涓€鑷?
				return fmt.Errorf("瑙ｆ瀽 INSERT 琛屾暟鎹け璐?(row_index: %d): %w", i, err)
			}
		case canal.UpdateAction:
			rowMap := p.rowToMap(e.Table.Columns, row)
			event, err = parseRowToSyncEvent(rowMap, domain.EventActionUpdate)
			if err != nil {
				return fmt.Errorf("瑙ｆ瀽 UPDATE 琛屾暟鎹け璐?(row_index: %d): %w", i, err)
			}
		case canal.DeleteAction:
			// DELETE 鎿嶄綔锛屽彧闇€瑕?ID
			id, err := p.extractIDFromRow(e.Table.Columns, row)
			if err != nil {
				return fmt.Errorf("鎻愬彇 DELETE ID 澶辫触 (row_index: %d): %w", i, err)
			}
			event = domain.SyncEvent{
				Type:   domain.EventTypeProduct,
				Action: domain.EventActionDelete,
				ID:     id,
			}
		default:
			p.logger.Warn("涓嶆敮鎸佺殑鎿嶄綔绫诲瀷",
				logger.String("action", e.Action),
				logger.Int("row_index", i))
			continue
		}

		events = append(events, event)
	}

	// 鎵归噺鍙戦€?
	if len(events) > 0 {
		if err := p.sendBatchToKafka(ctx, events); err != nil {
			p.logger.Error("鎵归噺鍙戦€佷簨浠跺埌 Kafka 澶辫触",
				logger.Error(err),
				logger.Int("event_count", len(events)))
			return fmt.Errorf("鎵归噺鍙戦€佷簨浠跺埌 Kafka 澶辫触: %w", err)
		}
	}

	return nil
}

// OnPosSynced position 鍚屾鍥炶皟锛屼繚瀛?position
func (p *CanalProducer) OnPosSynced(header *replication.EventHeader, pos mysql.Position, set mysql.GTIDSet, force bool) error {
	ctx := context.Background()
	if err := p.positionDao.SavePosition(ctx, PositionKey, pos); err != nil {
		p.logger.Error("淇濆瓨 position 澶辫触",
			logger.Error(err),
			logger.String("binlog_file", pos.Name),
			logger.Int32("binlog_pos", int32(pos.Pos)))
		return err
	}
	return nil
}

// OnTableChanged 琛ㄧ粨鏋勫彉鏇?
func (p *CanalProducer) OnTableChanged(header *replication.EventHeader, schema string, table string) error {
	p.logger.Info("琛ㄧ粨鏋勫彉鏇?,
		logger.String("schema", schema),
		logger.String("table", table))
	return nil
}

// OnDDL DDL 璇彞
func (p *CanalProducer) OnDDL(header *replication.EventHeader, nextPos mysql.Position, queryEvent *replication.QueryEvent) error {
	p.logger.Info("DDL 璇彞",
		logger.String("binlog_file", nextPos.Name),
		logger.Int32("binlog_pos", int32(nextPos.Pos)),
		logger.String("query", string(queryEvent.Query)))
	return nil
}

// OnGTID GTID 浜嬩欢
func (p *CanalProducer) OnGTID(header *replication.EventHeader, gtidEvent mysql.BinlogGTIDEvent) error {
	return nil
}

// OnXID XID 浜嬩欢锛堜簨鍔℃彁浜わ級
func (p *CanalProducer) OnXID(header *replication.EventHeader, pos mysql.Position) error {
	return nil
}

// OnRotate binlog rotate 浜嬩欢
func (p *CanalProducer) OnRotate(header *replication.EventHeader, r *replication.RotateEvent) error {
	p.logger.Info("Binlog rotate",
		logger.String("next_binlog", string(r.NextLogName)),
		logger.Int64("position", int64(r.Position)))
	return nil
}

// OnRowsQueryEvent RowsQuery 浜嬩欢
func (p *CanalProducer) OnRowsQueryEvent(event *replication.RowsQueryEvent) error {
	return nil
}

// String 杩斿洖瀛楃涓茶〃绀?
func (p *CanalProducer) String() string {
	return "CanalProducer"
}

// rowToMap 灏嗚鏁版嵁杞崲涓?map
func (p *CanalProducer) rowToMap(columns []schema.TableColumn, row []interface{}) map[string]interface{} {
	rowMap := make(map[string]interface{}, len(columns))
	for i, col := range columns {
		if i < len(row) {
			rowMap[col.Name] = row[i]
		}
	}
	return rowMap
}

// extractIDFromRow 浠庤鏁版嵁涓彁鍙?ID
func (p *CanalProducer) extractIDFromRow(columns []schema.TableColumn, row []interface{}) (int64, error) {
	// 鏌ユ壘 id 鍒楃殑绱㈠紩
	for i, col := range columns {
		if col.Name == "id" && i < len(row) {
			// 灏濊瘯绫诲瀷杞崲
			switch val := row[i].(type) {
			case int64:
				return val, nil
			case int32:
				return int64(val), nil
			case int:
				return int64(val), nil
			case string:
				// 瀛楃涓茶浆 int64
				id, err := strconv.ParseInt(val, 10, 64)
				if err != nil {
					return 0, fmt.Errorf("瑙ｆ瀽 ID 澶辫触: %w", err)
				}
				return id, nil
			default:
				return 0, fmt.Errorf("ID 绫诲瀷涓嶆敮鎸? %T", val)
			}
		}
	}
	return 0, fmt.Errorf("鏈壘鍒?id 鍒?)
}

// sendBatchToKafka 鎵归噺鍙戦€佷簨浠跺埌 Kafka
// Producer 宸查厤缃?Retry.Max = 3锛屼細鑷姩閲嶈瘯鍙噸璇曢敊璇紙鏃犺剳閲嶈瘯3娆★級
// 濡傛灉 SendMessages 杩斿洖閿欒锛岃鏄?Producer 鐨勯噸璇曞凡缁忕敤灏斤紙鎵€鏈夐敊璇兘涓嶅彲閲嶈瘯锛屾垨閲嶈瘯鍚庝粛澶辫触锛?
// 姝ゆ椂杩斿洖閿欒锛岃 Canal 閲嶈瘯鏁翠釜 binlog 浜嬩欢
func (p *CanalProducer) sendBatchToKafka(ctx context.Context, events []domain.SyncEvent) error {
	if len(events) == 0 {
		return nil
	}

	messages := make([]*sarama.ProducerMessage, 0, len(events))
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("搴忓垪鍖栦簨浠跺け璐?(ID: %d): %w", event.ID, err)
		}

		messages = append(messages, &sarama.ProducerMessage{
			Topic: TopicName,
			Key:   sarama.StringEncoder(fmt.Sprintf("%s:%d", event.Type, event.ID)), // 淇濊瘉鍚屼竴涓疄浣撶殑鍙樻洿鐨勯『搴忔€?
			Value: sarama.ByteEncoder(data),
		})
	}

	if len(messages) == 0 {
		return fmt.Errorf("娌℃湁鏈夋晥鐨勬秷鎭彲鍙戦€?)
	}

	err := p.kafkaProducer.SendMessages(messages)
	if err != nil {
		if producerErrors, ok := err.(sarama.ProducerErrors); ok {
			failedCount := len(producerErrors)
			totalCount := len(messages)
			p.logger.Error("鎵归噺鍙戦€侀儴鍒嗗け璐ワ紙Producer 宸查噸璇曪級",
				logger.Error(err),
				logger.Int("total_count", totalCount),
				logger.Int("failed_count", failedCount),
				logger.Int("success_count", totalCount-failedCount))
			for i, producerError := range producerErrors {
				if i < 10 {
					p.logger.Error("鍙戦€佸け璐ョ殑娑堟伅璇︽儏",
						logger.Error(producerError.Err),
						logger.String("topic", producerError.Msg.Topic),
						logger.String("key", string(producerError.Msg.Key.(sarama.StringEncoder))))
				}
			}
			return fmt.Errorf("鎵归噺鍙戦€侀儴鍒嗗け璐?(Producer宸查噸璇?: %d/%d 澶辫触: %w", failedCount, totalCount, err)
		}
		p.logger.Error("鎵归噺鍙戦€佸畬鍏ㄥけ璐ワ紙Producer 宸查噸璇曪級",
			logger.Error(err),
			logger.Int("total_count", len(messages)))
		return fmt.Errorf("鎵归噺鍙戦€佹秷鎭埌 Kafka 瀹屽叏澶辫触 (Producer宸查噸璇?: %w", err)
	}

	p.logger.Debug("鎵归噺浜嬩欢宸插彂閫佸埌 Kafka",
		logger.String("topic", TopicName),
		logger.Int("event_count", len(messages)))

	return nil
}


