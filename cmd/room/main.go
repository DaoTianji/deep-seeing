package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"deep-seeing/internal/app"
	"deep-seeing/internal/room"
)

// 兼容入口：与 go run ./cmd/see 相同，默认启动谈话室。
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.New(ctx, app.Options{SessionID: "room"})
	if err != nil {
		log.Fatal(err)
	}
	defer application.Close(context.Background())

	addr := strings.TrimSpace(os.Getenv("ROOM_ADDR"))
	if addr == "" {
		addr = "127.0.0.1:3319"
	}
	server := &room.Server{App: application, Addr: addr}
	handler, err := server.Handler()
	if err != nil {
		log.Fatal(err)
	}
	httpServer := &http.Server{Addr: addr, Handler: handler}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	fmt.Println("Deep-Seeing")
	fmt.Printf("谈话室: http://%s\n", addr)
	fmt.Printf("STM: %s | LTM: %s | model: %s\n", application.STMBackend, application.GraphLabel, application.Model)
	fmt.Println("按 Ctrl+C 停止。")

	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
