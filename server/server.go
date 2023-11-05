package server

import (
	"context"
	"log"
	"net"
	"strings"
	"sync"
)

type Server struct {
	addr     string
	listener *net.TCPListener
	ctx      context.Context
	shutdown context.CancelFunc
	Wg       *sync.WaitGroup
	ChClosed chan struct{}
}

func NewServer(addr string, listener net.Listener, ctx context.Context, shutdown context.CancelFunc, wg *sync.WaitGroup, chClosed chan struct{}) *Server {
	return &Server{
		addr:     addr,
		listener: listener.(*net.TCPListener),
		ctx:      ctx,
		shutdown: shutdown,
		Wg:       wg,
		ChClosed: chClosed,
	}
}

func (s *Server) HandleListener() {
	defer func() {
		// これ不要な気がする
		s.listener.Close()
		close(s.ChClosed)
	}()

	for {
		// 接続要求があればAcceptする
		conn, err := s.listener.AcceptTCP()
		if err != nil {
			if ne, ok := err.(net.Error); ok {
				// Temporaryは非推奨だが使用する
				// 非推奨な理由は、Temporary()の挙動がライブラリによって異なるから
				// 標準パッケージであるAcceptTCPの場合は、Temporary()がエラーになるのはシステムコールエラーの場合のみだとすぐわかるので、s、い要している
				if ne.Temporary() {
					continue
				}
			}
			if listenerCloseError(err) {
				select {
				case <-s.ctx.Done():
					return
				default:
					// 何もしない
					// ブロックしたくないので、defaultを定義している
				}
			}
			log.Println("AcceptTCP", err)
			return
		}

		readCtx, errRead := context.WithCancel(context.Background())
		c := &Conn{
			svr:     s,
			conn:    conn,
			readCtx: readCtx,
			errRead: errRead,
		}
		s.Wg.Add(1)

		// Accept後、接続をハンドリングする
		go c.handleConnection()
	}
}

const (
	listenerCloseMatcher = "use of closed network connection"
)

func listenerCloseError(err error) bool {
	return strings.Contains(err.Error(), listenerCloseMatcher)
}
