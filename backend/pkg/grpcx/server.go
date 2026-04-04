package grpcx

import (
	"context"
	"net"
	"strconv"
	"time"

	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/netx"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/naming/endpoints"
	"google.golang.org/grpc"
)

type Server struct {
	*grpc.Server
	Port int
	// ETCD 鏈嶅姟娉ㄥ唽绉熺害 TTL
	EtcdTTL     int64
	EtcdClient  *clientv3.Client
	etcdManager endpoints.Manager
	etcdKey     string
	cancel      func()
	Name        string
	L           logger.LoggerV1
}

// Serve 鍚姩鏈嶅姟鍣ㄥ苟涓旈樆濉?
func (s *Server) Serve() error {
	// 鍒濆鍖栦竴涓帶鍒舵暣涓繃绋嬬殑 ctx
	// 浣犱篃鍙互鑰冭檻璁╁闈紶杩涙潵锛岃繖鏍风殑璇濆氨鏄?main 鍑芥暟鑷繁鍘绘帶鍒朵簡
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	port := strconv.Itoa(s.Port)
	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	// 瑕佸厛纭繚鍚姩鎴愬姛锛屽啀娉ㄥ唽鏈嶅姟锛岃繖閲屾槸鏈嶅姟鍙戠幇
	err = s.register(ctx, port)
	if err != nil {
		return err
	}
	return s.Server.Serve(l)
}

// 鏈嶅姟娉ㄥ唽
func (s *Server) register(ctx context.Context, port string) error {
	cli := s.EtcdClient
	serviceName := "service/" + s.Name
	em, err := endpoints.NewManager(cli, serviceName)
	if err != nil {
		return err
	}
	s.etcdManager = em
	ip := netx.GetOutboundIP()
	s.etcdKey = serviceName + "/" + ip
	addr := ip + ":" + port
	leaseResp, err := cli.Grant(ctx, s.EtcdTTL)
	if err != nil {
		return err
	}
	// 寮€鍚画绾︼紝鍙€氳繃 ctx 鏉ユ帶鍒剁画绾︼紝涔熷氨鎺у埗浜嗘湇鍔℃敞鍐?
	ch, err := cli.KeepAlive(ctx, leaseResp.ID)
	if err != nil {
		return err
	}
	go func() {
		// 鍙互棰勬湡锛屽綋鎴戜滑鐨?cancel 琚皟鐢ㄧ殑鏃跺€欙紝灏变細閫€鍑鸿繖涓惊鐜?
		for chResp := range ch {
			s.L.Debug("缁害锛?, logger.String("resp", chResp.String()))
		}
	}()
	// metadata 鎴戜滑杩欓噷娌″暐瑕佹彁渚涚殑
	return em.AddEndpoint(ctx, s.etcdKey, endpoints.Endpoint{Addr: addr}, clientv3.WithLease(leaseResp.ID))
}

func (s *Server) Close() error {
	s.cancel()
	if s.etcdManager != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err := s.etcdManager.DeleteEndpoint(ctx, s.etcdKey)
		if err != nil {
			return err
		}
	}
	err := s.EtcdClient.Close()
	if err != nil {
		return err
	}
	s.Server.GracefulStop()
	return nil
}


