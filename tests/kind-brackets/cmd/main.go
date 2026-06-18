/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright freiheit.com*/

// Manual testing helpers for the kind-brackets test suite.
//
// Usage (kind):
//
//	go run ./tests/kind-brackets/cmd helm-upgrade [--brackets=true] [--development=false] [--staging=true] [--channel-size=50]
//	go run ./tests/kind-brackets/cmd release-train [--env=staging]
//	go run ./tests/kind-brackets/cmd create-release --app=APP --bracket=BRACKET [--team=TEAM] [--version=1] [--envs=development,staging]
//	go run ./tests/kind-brackets/cmd reset-db
//
// Usage (GKE):
//
//	KUBERPULT_TEST_MODE=gke KUBERPULT_TEST_VERSION=v13.52.9 go run ./tests/kind-brackets/cmd <subcommand> [flags]
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	kb "github.com/freiheit-com/kuberpult/tests/kind-brackets"
)

// cliTB implements kb.TB for use outside of a test context.
type cliTB struct{}

func (c *cliTB) Helper() {}
func (c *cliTB) Fatalf(format string, args ...any) { log.Fatalf(format, args...) }
func (c *cliTB) Logf(format string, args ...any)   { log.Printf(format, args...) }

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  main helm-upgrade [--brackets=true] [--development=false] [--staging=true] [--channel-size=50]")
		fmt.Fprintln(os.Stderr, "  main release-train [--env=staging]")
		fmt.Fprintln(os.Stderr, "  main create-release --app=APP --bracket=BRACKET [--team=test-team] [--version=1] [--envs=development,staging]")
		fmt.Fprintln(os.Stderr, "  main reset-db")
		os.Exit(1)
	}

	tb := &cliTB{}
	cfg := kb.MustLoadConfig()

	subcommand := os.Args[1]
	args := os.Args[2:]

	switch subcommand {
	case "helm-upgrade":
		fs := flag.NewFlagSet("helm-upgrade", flag.ExitOnError)
		brackets := fs.Bool("brackets", true, "enable experimental brackets")
		development := fs.Bool("development", false, "enable bracket on development cluster")
		staging := fs.Bool("staging", true, "enable bracket on staging cluster")
		channelSize := fs.Int("channel-size", 50, "kuberpult events channel size")
		_ = fs.Parse(args)
		kb.HelmUpgrade(tb, cfg, kb.HelmUpgradeParams{
			BracketsEnabled:    *brackets,
			DevelopmentEnabled: *development,
			StagingEnabled:     *staging,
			ChannelSize:        *channelSize,
		})

	case "release-train":
		kb.EnsurePortForwards(tb, cfg)
		fs := flag.NewFlagSet("release-train", flag.ExitOnError)
		env := fs.String("env", "staging", "target environment")
		_ = fs.Parse(args)
		kb.ReleaseTrain(tb, *env)

	case "create-release":
		kb.EnsurePortForwards(tb, cfg)
		fs := flag.NewFlagSet("create-release", flag.ExitOnError)
		app := fs.String("app", "", "application name (required)")
		team := fs.String("team", "test-team", "team name")
		bracket := fs.String("bracket", "", "bracket name (required)")
		version := fs.String("version", "1", "release version number")
		envsStr := fs.String("envs", "development,staging", "comma-separated target environments")
		_ = fs.Parse(args)
		if *app == "" || *bracket == "" {
			log.Fatal("--app and --bracket are required")
		}
		manifests := map[string]string{}
		for _, env := range strings.Split(*envsStr, ",") {
			manifests[env] = kb.StableManifest(*app, env, *version)
		}
		kb.CreateRelease(tb, *app, *team, *bracket, *version, manifests)

	case "reset-db":
		db := kb.MustLoadDBCreds(cfg)
		kb.ResetDB(tb, cfg, db)

	default:
		log.Fatalf("unknown subcommand %q; valid: helm-upgrade, release-train, create-release, reset-db", subcommand)
	}
}
