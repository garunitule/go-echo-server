package server

import (
	"context"
	"log"
	"net"
	"strings"
	"sync"
)

type Server struct {
	addr      string
	listener  *net.TCPListener
	ctx       context.Context
	shutdown  context.CancelFunc
	Wg        *sync.WaitGroup
	ChClosed  chan struct{}
	AcceptCtx context.Context
	errAccept context.CancelFunc
}

func NewServer(addr string, ctx context.Context, shutdown context.CancelFunc, wg *sync.WaitGroup, chClosed chan struct{}, acceptCtx context.Context, errAccept context.CancelFunc) *Server {
	return &Server{
		addr:      addr,
		ctx:       ctx,
		shutdown:  shutdown,
		Wg:        wg,
		ChClosed:  chClosed,
		AcceptCtx: acceptCtx,
		errAccept: errAccept,
	}
}

func (s *Server) Listen() error {
	tcpAddr, err := net.ResolveTCPAddr("tcp", s.addr)
	if err != nil {
		return err
	}

	l, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return err
	}
	s.listener = l
	go s.handleListener()
	return nil
}

func (s *Server) Shutdown() {
	select {
	case <-s.ctx.Done():
		// already shutdown
	default:
		// 各クライアントとの接続の終了をトリガー
		// handleConnection: servetCtx, readCtxいずれかのDoneチャネルに通知されたら、接続を終了する
		// handleRead: handleConnectionのconn.Close()で終了される
		s.shutdown()

		// Listenerを閉じる
		// handleListener: serverCtx.Done()に通知されるまで待つ。つまり、接続やそこで行われる処理が全て終了するまで待つ
		// Close処理がないとAcceptTCPでブロックされ続ける
		s.listener.Close()
	}
}

func (s *Server) handleListener() {
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
			s.errAccept()
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
