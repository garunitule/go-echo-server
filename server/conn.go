package server

import (
	"context"
	"log"
	"net"
)

type Conn struct {
	svr       *Server
	conn      *net.TCPConn
	readCtx   context.Context
	stopRead  context.CancelFunc
	ctxWrite  context.Context
	stopWrite context.CancelFunc
	sem       chan struct{}
}

// conn *net.TCPConn, serverCtx context.Context, wg *sync.WaitGroup
func (c *Conn) handleConnection() {
	defer func() {
		c.stopWrite()
		c.conn.Close()
		c.svr.Wg.Done()
	}()

	// リクエストに対して処理を行う
	// 今回は単にリクエスト内容をレスポンスとして返している
	go c.handleRead()

	select {
	case <-c.readCtx.Done():
	case <-c.svr.ctx.Done():
	case <-c.svr.AcceptCtx.Done():
	}
}

// conn *net.TCPConn, stopRead context.CancelFunc
func (c *Conn) handleRead() {
	// クライアント側から接続終了された時
	defer c.stopRead()

	buf := make([]byte, 4*1024)

	for {
		n, err := c.conn.Read(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok {
				if ne.Temporary() {
					continue
				}
			}
			log.Println("Read error: ", err)
			return
		}

		wBuf := make([]byte, n)
		copy(wBuf, buf[:n])
		go c.handleEcho(wBuf)
	}
}

func (c *Conn) handleEcho(buf []byte) {
	// 何らかのデータベースに関する処理を行う
	// そのために、handleReadからhandleEchoを切り出して多重化可能にしている

	select {
	case <-c.ctxWrite.Done():
		return
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
		for {
			n, err := c.conn.Write(buf)
			if err != nil {
				if ne, ok := err.(net.Error); ok {
					if ne.Temporary() {
						buf = buf[n:]
						continue
					}
				}
				log.Println("Write error: ", err)
				c.stopRead()
			}
			return
		}
	}
}
