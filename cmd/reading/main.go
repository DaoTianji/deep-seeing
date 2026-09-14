// reading starts the isolated competition prototype without loading ordinary
// memory, databases, other roles, or the production Room application.
package main

import (
	"context"
	"deep-seeing/internal/room"
	"deep-seeing/internal/story"
	"flag"
	"github.com/joho/godotenv"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:3320", "local listening address")
	root := flag.String("data", "data/reading-showcase", "isolated story data directory")
	env := flag.String("env", "", "optional model environment file")
	publicDemo := flag.Bool("public-demo", false, "HTTPS-only competition service with request and model budget guards")
	dailyCalls := flag.Int("daily-model-calls", 300, "maximum actual model attempts per UTC day in public demo")
	flag.Parse()
	if *env != "" {
		if err := godotenv.Load(*env); err != nil {
			log.Fatal("cannot load model environment file")
		}
	}
	host, _, err := net.SplitHostPort(*addr)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		log.Fatal("prototype must listen on a loopback IP")
	}
	var chat story.Completer
	provider := os.Getenv("STORY_PROVIDER")
	if provider == "" {
		provider = "gateway"
	}
	model := os.Getenv("OPENAI_MODEL")
	if provider == "codex" {
		model = os.Getenv("STORY_CODEX_MODEL")
		chat, err = story.NewCodexClient(os.Getenv("STORY_CODEX_BIN"), model)
		if err != nil {
			log.Fatal(err)
		}
		if model == "" {
			model = "codex-default"
		}
	} else if provider != "gateway" {
		log.Fatal("STORY_PROVIDER must be gateway or codex")
	} else if os.Getenv("OPENAI_API_KEY") != "" && os.Getenv("OPENAI_MODEL") != "" {
		chat = story.NewReadingGatewayClient(os.Getenv("OPENAI_API_KEY"), os.Getenv("OPENAI_BASE_URL"), model)
	}
	if *publicDemo && *dailyCalls < 1 {
		log.Fatal("daily-model-calls must be positive")
	}
	if *publicDemo && chat != nil {
		chat = &story.BudgetedCompleter{Next: chat, Root: *root, DailyLimit: *dailyCalls}
	}
	engine, err := story.New(*root, chat, model, os.Getenv("STORY_MODE"))
	if err != nil {
		log.Fatal(err)
	}
	engine.Provider = provider
	engine.Research = story.NewCompanionResearchFactory(*root)
	engine.ResearchProvider = story.CompanionResearchProviderName()
	if *publicDemo {
		engine.Research = nil
		engine.ResearchProvider = ""
	}
	h, err := room.ReadingHandler(engine)
	if err != nil {
		log.Fatal(err)
	}
	if *publicDemo {
		h = story.PublicDemoGuard(h)
	}
	server := &http.Server{Addr: *addr, Handler: h, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	go func() {
		log.Printf("Reading showcase http://%s/reading (mode=%s, model_connected=%t)", *addr, engine.Mode, chat != nil)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}
