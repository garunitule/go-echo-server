package main

import (
	"context"
	"github/garunitue/go-echo-server/server"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// TODO: 全ての停止パターンを網羅できているか
func main() {
	// ソケットにアドレスとポートをバインド
	tcpAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:12345")
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
	server := server.NewServer(
		"localhost", // TODO: 修正
		l,
		serverCtx,
		shutdown,
		&wg,
		chClosed,
	)

	// go handleListener(l, serverCtx, &wg, chClosed)
	go server.HandleListener()

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
