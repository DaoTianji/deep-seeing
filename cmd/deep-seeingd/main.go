// Command deep-seeingd runs the conversation room plus agency scheduler.
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

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.New(ctx, app.Options{SessionID: "room"})
	if err != nil {
		log.Fatal(err)
	}
	defer application.Close(context.Background())

	application.StartScheduler(ctx)

	addr := strings.TrimSpace(os.Getenv("ROOM_ADDR"))
	if addr == "" {
		addr = "127.0.0.1:8787"
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

	fmt.Println("Deep-Seeing daemon")
	fmt.Printf("谈话室: http://%s\n", addr)
	fmt.Printf("STM: %s | LTM: %s | agency tick: %s\n",
		application.STMBackend, application.GraphLabel, envOr("AGENCY_TICK", "1m"))
	fmt.Println("Intent/wake: data/runtime/runtime.db | outbound contact: off by default")
	fmt.Println("按 Ctrl+C 停止。")

	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
