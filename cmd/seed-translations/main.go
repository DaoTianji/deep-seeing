// seed-translations prepares distributable default editions from the embedded
// public-domain samples only. It never reads personal reader data.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"deep-seeing/internal/story"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func main() {
	env := flag.String("env", "", "model environment file")
	out := flag.String("output", "internal/story/translations", "default edition output directory")
	cache := flag.String("cache", "data/translation-seeding", "isolated resumable generation cache")
	selected := flag.String("books", "necklace,magi,leaf,paw,taohuayuan,quanxue,mulan", "public sample IDs")
	run := flag.Bool("run", false, "explicitly allow public sample text to be sent to configured model")
	batchSize := flag.Int("batch-size", 16, "sentences per batch, 1-16")
	flag.Parse()
	if !*run {
		fmt.Fprintln(os.Stderr, "Pass -run to authorize model generation from public samples.")
		os.Exit(1)
	}
	if *env != "" {
		if err := godotenv.Load(*env); err != nil {
			fmt.Fprintln(os.Stderr, "cannot load model environment")
			os.Exit(1)
		}
	}
	if os.Getenv("OPENAI_API_KEY") == "" || os.Getenv("OPENAI_MODEL") == "" {
		fmt.Fprintln(os.Stderr, "model configuration missing")
		os.Exit(1)
	}
	catalog := map[string]story.Book{}
	for _, b := range story.ReadingCatalog() {
		catalog[b.ID] = b
	}
	ids := strings.Split(*selected, ",")
	for _, id := range ids {
		if _, ok := catalog[id]; !ok {
			fmt.Fprintln(os.Stderr, "unknown sample:", id)
			os.Exit(1)
		}
	}
	client := story.NewReadingGatewayClient(os.Getenv("OPENAI_API_KEY"), os.Getenv("OPENAI_BASE_URL"), os.Getenv("OPENAI_MODEL"))
	client.MaxTokens = 8192
	var wg sync.WaitGroup
	slots := make(chan struct{}, 2)
	errs := make(chan error, len(ids))
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			e, err := story.New(filepath.Join(*cache, id), client, os.Getenv("OPENAI_MODEL"), "agent")
			if err != nil {
				errs <- fmt.Errorf("%s: initialization failed", id)
				return
			}
			e.Book = catalog[id]
			owner := uuid.NewString()
			completed := 0
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
			defer cancel()
			for attempts := 0; attempts < 100; attempts++ {
				s, err := e.PrepareDefaultBatch(ctx, owner, "fluent", completed, *batchSize)
				if err != nil {
					errs <- fmt.Errorf("%s: generation stopped, cache retained: %w", id, err)
					return
				}
				fmt.Printf("%s %d/%d sentences\n", id, s.Completed, s.Total)
				if s.Completed == s.Total {
					if err = e.ExportDefaultTranslation(owner, "fluent", filepath.Join(*out, id+".json")); err != nil {
						errs <- err
					}
					return
				}
				completed = s.Completed
			}
			errs <- fmt.Errorf("%s: batch limit reached", id)
		}(id)
	}
	wg.Wait()
	close(errs)
	failed := false
	for err := range errs {
		failed = true
		fmt.Fprintln(os.Stderr, err)
	}
	if failed {
		os.Exit(1)
	}
}
