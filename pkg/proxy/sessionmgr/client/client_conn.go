package client

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/terror"
	"github.com/pingcap/tidb/metrics"
	"github.com/pingcap/tidb/util/arena"
	"github.com/pingcap/tidb/util/fastrand"
	"github.com/pingcap/tidb/util/logutil"
	"github.com/tidb-incubator/weir/pkg/proxy/driver"
	pnet "github.com/tidb-incubator/weir/pkg/proxy/net"
	"go.uber.org/zap"
)

type ClientConnectionImpl struct {
	queryCtx         driver.QueryCtx
	tlsConn          *tls.Conn // TLS connection, nil if not TLS.
	tlsConfig        *tls.Config
	collation        uint8
	pkt              *pnet.PacketIO         // a helper to read and write data in packet format.
	bufReadConn      *pnet.BufferedReadConn // a buffered-read net.Conn or buffered-read tls.Conn.
	connectionID     uint64
	serverCapability uint32
	capability       uint32 // final capability
	status           int32
	alloc            arena.Allocator
	user             string            // user of the client.
	dbname           string            // default database name.
	salt             []byte            // random bytes used for authentication.
	attrs            map[string]string // attributes parsed from client handshake response, not used for now.
	peerHost         string            // peer host
	peerPort         string            // peer port
}

func NewClientConnectionImpl(queryCtx driver.QueryCtx, conn net.Conn, connectionID uint64, tlsConfig *tls.Config, serverCapability uint32) driver.ClientConnection {
	bufReadConn := pnet.NewBufferedReadConn(conn)
	pkt := pnet.NewPacketIO(bufReadConn)
	return &ClientConnectionImpl{
		queryCtx:         queryCtx,
		tlsConfig:        tlsConfig,
		serverCapability: serverCapability,
		alloc:            arena.NewAllocator(32 * 1024),
		bufReadConn:      bufReadConn,
		pkt:              pkt,
		connectionID:     connectionID,
		salt:             fastrand.Buf(20),
	}
}

func (cc *ClientConnectionImpl) ConnectionID() uint64 {
	return cc.connectionID
}

func (cc *ClientConnectionImpl) Addr() string {
	return cc.bufReadConn.RemoteAddr().String()
}

func (cc *ClientConnectionImpl) Auth() error {
	return nil
}

func (cc *ClientConnectionImpl) Run(ctx context.Context) {
	if err := cc.handshake(ctx); err != nil {
		// Some keep alive services will send request to TiDB and disconnect immediately.
		// So we only record metrics.
		metrics.HandShakeErrorCounter.Inc()
		err = cc.Close()
		terror.Log(errors.Trace(err))
		return
	}
	logutil.Logger(ctx).Info("new connection", zap.String("remoteAddr", cc.bufReadConn.RemoteAddr().String()))

}

func (cc *ClientConnectionImpl) Close() error {
	return nil
}
