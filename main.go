package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

const (
	listenerCloseMatcher = "use of closed network connection"
)

func handleConnection(conn *net.TCPConn, serverCtx context.Context, wg *sync.WaitGroup) {
	defer func() {
		conn.Close()
		wg.Done()
	}()

	readCtx, errRead := context.WithCancel(context.Background())

	// リクエストに対して処理を行う
	// 今回は単にリクエスト内容をレスポンスとして返している
	go handleRead(conn, errRead)

	select {
	case <-readCtx.Done():
	case <-serverCtx.Done():
	}
}

func handleRead(conn *net.TCPConn, errRead context.CancelFunc) {
	// クライアント側から接続終了された時
	defer errRead()

	buf := make([]byte, 4*1024)

	for {
		n, err := conn.Read(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok {
				if ne.Temporary() {
					continue
				}
			}
			log.Println("Read error: ", err)
			return
		}

		n, err = conn.Write(buf[:n])
		if err != nil {
			log.Println("Write error: ", err)
			return
		}
	}
}

func handleListener(l *net.TCPListener, serverCtx context.Context, wg *sync.WaitGroup, chClosed chan struct{}) {
	defer func() {
		log.Println("called handleListener defer")
		// これ不要な気がする
		l.Close()
		close(chClosed)
	}()

	for {
		// 接続要求があればAcceptする
		conn, err := l.AcceptTCP()
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
				case <-serverCtx.Done():
					return
				default:
					// 何もしない
					// ブロックしたくないので、defaultを定義している
				}
			}
			log.Println("AcceptTCP", err)
			return
		}

		wg.Add(1)

		// Accept後、接続をハンドリングする
		go handleConnection(conn, serverCtx, wg)
	}
}

func listenerCloseError(err error) bool {
	return strings.Contains(err.Error(), listenerCloseMatcher)
}

func main() {
	// ソケットにアドレスとポートをバインド
	tcpAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:12348")
	if err != nil {
		log.Println("ResolveTCPAddr", err)
		return
	}
	// Listenして外部からの接続を待つ
	l, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		log.Println("ListenTCP", err)
		return
	}

	sigChan := make(chan os.Signal, 1)
	signal.Ignore()
	signal.Notify(sigChan, syscall.SIGINT)

	var wg sync.WaitGroup
	chClosed := make(chan struct{})

	serverCtx, shutdown := context.WithCancel(context.Background())

	go handleListener(l, serverCtx, &wg, chClosed)

	log.Println("Server started")

	s := <-sigChan
	switch s {
	case syscall.SIGINT:
		log.Println("Server shutdown...")
		// 各クライアントとの接続の終了をトリガー
		// handleConnection: servetCtx, readCtxいずれかのDoneチャネルに通知されたら、接続を終了する
		// handleRead: handleConnectionのconn.Close()で終了される
		shutdown()
		// Listenerを閉じる
		// handleListener: serverCtx.Done()に通知されるまで待つ。つまり、接続やそこで行われる処理が全て終了するまで待つ
		// Close処理がないとAcceptTCPでブロックされ続ける
		l.Close()

		// 各クライアントとの接続が終了するまで待つ
		wg.Wait()
		// Listenerが終了するまで待つ
		<-chClosed
		log.Println("Server shutdown completed")
	default:
		panic("unexpected signal has been received")
	}
}
