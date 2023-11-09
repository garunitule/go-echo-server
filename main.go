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
	acceptCtx, errAccept := context.WithCancel(context.Background())
	ctxGracefulShutdown, gshutdown := context.WithCancel(context.Background())
	srv := server.NewServer(
		"localhost:12345",
		serverCtx,
		shutdown,
		&wg,
		chClosed,
		acceptCtx,
		errAccept,
		ctxGracefulShutdown,
		gshutdown,
	)
	srv.Listen()
	log.Println("Server started")

	select {
	case sig := <-sigChan:
		switch sig {
		case syscall.SIGINT:
			log.Println("Server shutdown...")
			srv.Shutdown()

			// 各クライアントとの接続が終了するまで待つ
			wg.Wait()
			// Listenerが終了するまで待つ
			<-srv.ChClosed
			log.Println("Server shutdown completed")
		case syscall.SIGQUIT:
			log.Println("Server shutdown...")
			srv.GracefulShutdown()

			srv.Wg.Wait()
			<-srv.ChClosed
			log.Println("Server shutdown completed")
		default:
			panic("unexpected signal has been received")
		}
	case <-srv.AcceptCtx.Done():
		log.Println("Server Error Occurred")
		srv.Wg.Wait()
		<-srv.ChClosed
		log.Println("Server dhutdown completed")
	}
}
