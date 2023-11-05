package main

import (
	"context"
	"github/garunitue/go-echo-server/server"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// TODO: 全ての停止パターンを網羅できているか
func main() {
	sigChan := make(chan os.Signal, 1)
	signal.Ignore()
	signal.Notify(sigChan, syscall.SIGINT)

	var wg sync.WaitGroup
	chClosed := make(chan struct{})

	serverCtx, shutdown := context.WithCancel(context.Background())
	server := server.NewServer(
		"localhost:12345",
		serverCtx,
		shutdown,
		&wg,
		chClosed,
	)
	server.Listen()
	log.Println("Server started")

	s := <-sigChan
	switch s {
	case syscall.SIGINT:
		log.Println("Server shutdown...")
		server.Shutdown()

		// 各クライアントとの接続が終了するまで待つ
		wg.Wait()
		// Listenerが終了するまで待つ
		<-chClosed
		log.Println("Server shutdown completed")
	default:
		panic("unexpected signal has been received")
	}
}
