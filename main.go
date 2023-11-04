package main

import (
	"log"
	"net"
)

func handleConnection(conn *net.TCPConn) {
	defer conn.Close()

	buf := make([]byte, 4*1024)

	for {
		n, err := conn.Read(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok {
				if ne.Temporary() {
					continue
				}
			}
			log.Println("Read", err)
			return
		}
		n, err = conn.Write(buf[:n])
		if err != nil {
			log.Println("Write", err)
			return
		}
	}
}

func handleListener(l *net.TCPListener) error {
	defer l.Close()
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
			return err
		}

		// Accept後、リクエスト内容を処理する
		go handleConnection(conn)
	}
}

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

	// 外部から接続を待ち受ける
	err = handleListener(l)
	if err != nil {
		log.Println("handleListener", err)
	}
}
