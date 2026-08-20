package main

import (
	"bufio"
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
	"deep-seeing/internal/backup"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/origin"
	"deep-seeing/internal/room"
	"deep-seeing/internal/soul"
)

func main() {
	cliMode := false
	for _, arg := range os.Args[1:] {
		if arg == "--cli" || arg == "-cli" {
			cliMode = true
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sessionID := "room"
	if cliMode {
		sessionID = "cli"
	}
	application, err := app.New(ctx, app.Options{SessionID: sessionID})
	if err != nil {
		log.Fatal(err)
	}
	defer application.Close(context.Background())

	if cliMode {
		runCLI(ctx, application)
		return
	}
	runRoom(ctx, application)
}

func runRoom(ctx context.Context, application *app.App) {
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
	if application.FirstBoot {
		fmt.Println("first_boot: Origin Introduction 将只呈现这一次。")
	}
	fmt.Println("浏览器打开上面的地址即可对话。终端模式请用: go run ./cmd/see --cli")
	fmt.Println("按 Ctrl+C 停止。")

	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func runCLI(ctx context.Context, application *app.App) {
	fmt.Println("Deep-Seeing")
	fmt.Printf("STM: %s | LTM: %s | Review/Dream: opportunity\n", application.STMBackend, application.GraphLabel)
	originLabel := application.OriginLetter.PersonKey
	if originLabel == "" {
		originLabel = "(none)"
	}
	bootLabel := "already met"
	if application.FirstBoot {
		bootLabel = "first_boot (Origin presented once)"
	}
	fmt.Printf("Soul: %s | Origin: %s (%s) | %s\n", soul.DefaultSoulPath, originLabel, origin.RoleAtOrigin, bootLabel)
	fmt.Println("命令: /review  /dream  /backup  |  exit 退出（先回看机会）")
	fmt.Println()

	reader := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("你> ")
		if !reader.Scan() {
			runSessionReview(ctx, application)
			break
		}
		text := strings.TrimSpace(reader.Text())
		if text == "" {
			continue
		}
		if text == "exit" || text == "quit" {
			runSessionReview(ctx, application)
			break
		}
		if text == "/review" {
			runSessionReview(ctx, application)
			continue
		}
		if text == "/dream" {
			runDream(ctx, application)
			continue
		}
		if text == "/backup" {
			dest, err := backup.Snapshot(".", "", nil)
			if err != nil {
				fmt.Printf("backup 失败: %v\n", err)
			} else {
				fmt.Printf("backup ok: %s\n", dest)
			}
			continue
		}

		fmt.Print("助手> ")
		err := application.Queue.RunCognitive(ctx, "chat", func(turnCtx context.Context) error {
			_, err := application.Service.StreamTurn(turnCtx, text,
				func(delta string) { fmt.Print(delta) },
				func(name string) { fmt.Printf("\n  · tool start: %s\n", name) },
			)
			return err
		})
		if err != nil {
			fmt.Printf("\nagent 错误: %v\n", err)
			continue
		}
		fmt.Println()
		fmt.Println()
	}
}

func runDream(ctx context.Context, application *app.App) {
	fmt.Println("… Dream opportunity …")
	var res memory.DreamResult
	err := application.Queue.RunCognitive(ctx, "dream", func(turnCtx context.Context) error {
		var runErr error
		res, runErr = application.Dreamer.Run(turnCtx, application.Scope, true)
		return runErr
	})
	if err != nil {
		fmt.Printf("dream 失败: %v\n", err)
		return
	}
	if res.Skipped {
		fmt.Printf("No change / skipped: %s\n", res.Reason)
		if res.Notes != "" {
			fmt.Printf("notes: %s\n", res.Notes)
		}
		return
	}
	fmt.Printf("dream_id=%s\n", res.DreamID)
	if len(res.Accepted) > 0 {
		fmt.Printf("accepted: %s\n", strings.Join(res.Accepted, ", "))
	}
	if len(res.Rejected) > 0 {
		fmt.Printf("rejected: %s\n", strings.Join(res.Rejected, ", "))
	}
	if len(res.Deferred) > 0 {
		fmt.Printf("deferred: %s\n", strings.Join(res.Deferred, ", "))
	}
	if len(res.MutationIDs) > 0 {
		fmt.Printf("mutations: %s\n", strings.Join(res.MutationIDs, ", "))
	}
	if res.Notes != "" {
		fmt.Printf("notes: %s\n", res.Notes)
	}
}

func runSessionReview(ctx context.Context, application *app.App) {
	history, err := application.STM.Get(application.SessionID)
	if err != nil {
		log.Printf("session review stm: %v", err)
		return
	}
	fmt.Println("… Session Review opportunity …")
	var res memory.ReviewResult
	err = application.Queue.RunCognitive(ctx, "review", func(turnCtx context.Context) error {
		var runErr error
		res, runErr = application.Reviewer.Run(turnCtx, application.Scope, application.SessionID, history)
		return runErr
	})
	if err != nil {
		fmt.Printf("回看失败: %v\n", err)
		return
	}
	if res.Skipped {
		fmt.Printf("No change / skipped: %s\n", res.Reason)
		if res.Notes != "" {
			fmt.Printf("notes: %s\n", res.Notes)
		}
		return
	}
	fmt.Printf("hypothesis=%s\n", res.Hypothesis)
	if res.StateObservationID != "" {
		fmt.Printf("state_observation: %s\n", res.StateObservationID)
	}
	if len(res.ProposalIDs) > 0 {
		fmt.Printf("proposals: %s\n", strings.Join(res.ProposalIDs, ", "))
	}
	if res.Notes != "" {
		fmt.Printf("notes: %s\n", res.Notes)
	}
}
