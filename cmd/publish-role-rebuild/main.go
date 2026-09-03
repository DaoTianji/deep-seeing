// Command publish-role-rebuild approves an isolated whole-book rebuild while
// preserving the existing theater instance that was deliberately removed from
// the rebuild workspace. It never writes to the source store.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	"deep-seeing/internal/theater"
)

func main() {
	var (
		sourceRoot = flag.String("source-root", "", "immutable baseline role-store root")
		workRoot   = flag.String("work-root", "", "isolated rebuilt role-store root")
		roleID     = flag.String("role-id", "", "role definition id")
		runID      = flag.String("run-id", "", "rebuild initialization id")
		reason     = flag.String("warning-reason", "", "reason for accepting non-blocking Critic warnings")
	)
	flag.Parse()
	if strings.TrimSpace(*sourceRoot) == "" || strings.TrimSpace(*workRoot) == "" || strings.TrimSpace(*roleID) == "" || strings.TrimSpace(*runID) == "" {
		log.Fatal("source-root, work-root, role-id and run-id are required")
	}

	ctx := context.Background()
	source, err := theater.NewStore(*sourceRoot)
	if err != nil {
		log.Fatal(err)
	}
	work, err := theater.NewStore(*workRoot)
	if err != nil {
		log.Fatal(err)
	}
	baseline, err := source.GetDefinition(ctx, *roleID)
	if err != nil {
		log.Fatal(err)
	}
	rebuilt, err := work.GetDefinition(ctx, *roleID)
	if err != nil {
		log.Fatal(err)
	}
	if baseline.ID != rebuilt.ID || baseline.Scope != rebuilt.Scope {
		log.Fatal("baseline and rebuilt role identity do not match")
	}
	if baseline.MainInstanceID == "" {
		log.Fatal("baseline role has no theater instance to preserve")
	}
	if rebuilt.MainInstanceID != "" && rebuilt.MainInstanceID != baseline.MainInstanceID {
		log.Fatal("rebuilt role points to a different theater instance")
	}
	if rebuilt.MainInstanceID == "" {
		rebuilt.MainInstanceID = baseline.MainInstanceID
		rebuilt, err = work.SaveDefinition(ctx, rebuilt, rebuilt.Version)
		if err != nil {
			log.Fatal(err)
		}
	}

	architect := &theater.CharacterArchitect{Mode: theater.InitModeAgent, Scope: rebuilt.Scope, Store: work}
	run, published, err := architect.ApproveFinal(ctx, *runID, *reason)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("published role=%s version=%d instance=%s run=%s status=%s\n", published.ID, published.Version, published.MainInstanceID, run.ID, run.Status)
}
