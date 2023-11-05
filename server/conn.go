package server

import (
	"context"
	"log"
	"net"
)

type Conn struct {
	svr     *Server
	conn    *net.TCPConn
	readCtx context.Context
	errRead context.CancelFunc
}

// conn *net.TCPConn, serverCtx context.Context, wg *sync.WaitGroup
func (c *Conn) handleConnection() {
	defer func() {
		c.conn.Close()
		c.svr.Wg.Done()
	}()

	// リクエストに対して処理を行う
	// 今回は単にリクエスト内容をレスポンスとして返している
	go c.handleRead()

	select {
	case <-c.readCtx.Done():
	case <-c.svr.ctx.Done():
	}
}

// conn *net.TCPConn, errRead context.CancelFunc
func (c *Conn) handleRead() {
	// クライアント側から接続終了された時
	defer c.errRead()

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

		n, err = c.conn.Write(buf[:n])
		if err != nil {
			log.Println("Write error: ", err)
			return
		}
	}
}
