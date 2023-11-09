package server

import (
	"context"
	"log"
	"net"
	"sync"
)

type Conn struct {
	svr       *Server
	conn      *net.TCPConn
	readCtx   context.Context
	stopRead  context.CancelFunc
	ctxWrite  context.Context
	stopWrite context.CancelFunc
	sem       chan struct{}
	wg        sync.WaitGroup
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
	case <-c.svr.ctxGracefulShutdown.Done():
		// 読み取りを終了する
		c.conn.CloseRead()
		// 書き込みが全て終わるまで待つ
		c.wg.Wait()
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
	defer c.wg.Done()
	// 何らかのデータベースに関する処理を行う
	// そのために、handleReadからhandleEchoを切り出して多重化可能にしている

	select {
	// 接続終了したら、handleEchoも終了する
	case <-c.ctxWrite.Done():
		return
	// 一度に一つのgoroutineが書き込みを行う
	// 書き込みが終了、または、失敗したら、handleEchoは終了する
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
		for {
			n, err := c.conn.Write(buf)
			if err != nil {
				if ne, ok := err.(net.Error); ok {
					// 一時的なエラーが発生した場合は、再送する
					if ne.Temporary() {
						buf = buf[n:]
						continue
					}
				}
				// 致命的なエラーが発生した場合は書き込みをせず、接続を終了する
				log.Println("Write error: ", err)
				c.stopRead()
				c.stopWrite()
			}
			return
		}
	}
}
