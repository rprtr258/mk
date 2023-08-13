package main

import (
	"fmt"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/rprtr258/fun"
	"github.com/rprtr258/log"
	"github.com/urfave/cli/v2"

	"github.com/rprtr258/mk"
	"github.com/rprtr258/mk/agent"
	"github.com/rprtr258/mk/contrib/docker"
	"github.com/rprtr258/mk/ssh"
)

func main() {
	if err := (&cli.App{ //nolint:exhaustruct // daaaaaa
		// TODO: by default should show help
		Name:  "mk",
		Usage: "project commands runner",
		// TODO: hide flag, show command
		HideHelp:        true,
		HideHelpCommand: true,
		/* TODO: custom help format
			@awk 'BEGIN {FS = ":.*?# "} { \
				if (/^[a-zA-Z_-]+:.*?#.*$$/) \
					{ printf "  ${YELLOW}%-20s${RESET}${WHTIE}%s${RESET}\n", $$1, $$2} \
				else if (/^## .*$$/) \
					{ printf "${CYAN}%s:${RESET}\n", substr($$1,4)} \
			}' $(MAKEFILE_LIST)

		   # regex for makefile targets
		   	@printf "────────────────────────`tput bold``tput setaf 2` Make Targets `tput sgr0`────────────────────────────────\n"
		   	@sed -ne "/@sed/!s/\(^[^#?=]*:[^=]\).*##\(.*\)/`tput setaf 2``tput bold`\1`tput sgr0`\2/p" $(MAKEFILE_LIST)
		   # regex for makefile variables
		   	@printf "────────────────────────`tput bold``tput setaf 4` Make Variables `tput sgr0`───────────────────────────────\n"
		   	@sed -ne "/@sed/!s/\(.*\)[?:]=\(.*\)##\(.*\)/`tput setaf 4``tput bold`\1: `tput setaf 5`\2`tput sgr0`\3/p" $(MAKEFILE_LIST)
		*/
		Commands: []*cli.Command{
			{
				Name:  "test",
				Usage: "run tests, get coverage",
				Action: func(ctx *cli.Context) error {
					if _, _, err := mk.ExecContext(ctx.Context,
						"go", "test", "./...",
						"-count=1",
						"-race",
					); err != nil {
						return fmt.Errorf("go test: %w", err)
					}

					// TODO: command to show coverage percent
					// if _, _, err := mk.ExecContext(ctx.Context,
					// 	"go", "tool", "cover",
					// 	"-func=cover.out",
					// ); err != nil {
					// 	return fmt.Errorf("show coverage: %w", err)
					// }

					return nil
				},
				Subcommands: []*cli.Command{
					{
						Name:  "coverage",
						Usage: "run tests and show coverage",
						Action: func(ctx *cli.Context) error {
							// TODO: collect coverage
							if _, _, err := mk.ExecContext(ctx.Context,
								"go", "test", "./...",
								"-count=1",
								"-cover",
								"-race",
								"-coverprofile=cover.out",
							); err != nil {
								return fmt.Errorf("go test: %w", err)
							}

							if _, _, err := mk.ExecContext(ctx.Context,
								"go", "tool", "cover",
								"-html=cover.out",
							); err != nil {
								return fmt.Errorf("show coverage: %w", err)
							}

							return nil
						},
					},
				},
			},
			{
				Name:  "lint",
				Usage: "run linters",
				Action: func(ctx *cli.Context) error {
					// TODO: lint go

					// TODO: lint proto

					return nil
				},
				Subcommands: []*cli.Command{
					{
						Name:  "go",
						Usage: "linter .go sources",
						Action: func(ctx *cli.Context) error {
							// TODO: ensure instead
							// TODO: pin version
							if _, _, err := mk.ExecContext(ctx.Context,
								"go", "install", "github.com/golangci/golangci-lint/cmd/golangci-lint@v1.50",
							); err != nil {
								return fmt.Errorf("install golangci-lint: %w", err)
							}

							if _, _, err := mk.ExecContext(ctx.Context,
								"golangci-lint", "run", "-v",
							); err != nil {
								return fmt.Errorf("golangci-lint: %w", err)
							}

							return nil
						},
					},
					{
						Name:  "proto:",
						Usage: "lint .proto files",
						Action: func(ctx *cli.Context) error {
							// TODO: ensure instead
							// TODO: pin version
							if _, _, err := mk.ExecContext(ctx.Context,
								"go", "install", "github.com/yoheimuta/protolint/cmd/protolint@latest",
							); err != nil {
								return fmt.Errorf("install protolint: %w", err)
							}

							if _, _, err := mk.ExecContext(ctx.Context,
								"protolint", "lint", "-reporter", "unix", "./api",
							); err != nil {
								return fmt.Errorf("protolint: %w", err)
							}

							return nil
						},
					},
				},
			},
			{
				Name:  "precommit",
				Usage: "run precommit checks",
				Action: func(ctx *cli.Context) error {
					if _, _, err := mk.ExecContext(ctx.Context,
						"gofmt", "-w", "-s", "-d", ".",
					); err != nil {
						return fmt.Errorf("run gofmt: %w", err)
					}

					// TODO: replace
					// $(MAKE) lint
					// $(MAKE) test

					return nil
				},
			},
			{
				Name:  "install-hook",
				Usage: "install git precommit hook",
				Action: func(ctx *cli.Context) error {
					if err := os.WriteFile(
						".git/hooks/pre-commit",
						[]byte("#!/bin/sh\nmake precommit"),
						0o544,
					); err != nil {
						return fmt.Errorf("write precommit hook: %w", err)
					}

					return nil
				},
			},
			{
				Name:  "todo",
				Usage: "list TODO comments",
				Action: func(ctx *cli.Context) error {
					if _, _, err := mk.ExecContext(ctx.Context,
						"grep", "// TODO",
						"-r",
						"--exclude-dir", "third_party",
						"--exclude", "Makefile",
						".",
					); err != nil {
						return fmt.Errorf("list todos: %w", err)
					}

					// TODO if none found
					// fmt.Println("All done!")

					return nil
				},
			},
			{
				Name: "deps",
				Subcommands: []*cli.Command{
					{
						Name:  "bump",
						Usage: "update dependencies",
						Action: func(ctx *cli.Context) error {
							if _, _, err := mk.ExecContext(ctx.Context,
								"go", "get", "-u", "./...",
							); err != nil {
								return fmt.Errorf("update deps: %w", err)
							}

							// TODO: extract "go" to command
							if _, _, err := mk.ExecContext(ctx.Context,
								"go", "mod", "tidy",
							); err != nil {
								return fmt.Errorf("go mod tidy: %w", err)
							}

							return nil
						},
					},
					{
						Name:  "audit",
						Usage: "audit dependencies",
						Action: func(ctx *cli.Context) error {
							// TODO: simplify?
							// TODO: move version out
							if _, _, err := mk.ExecContext(ctx.Context,
								"bash", "-c", `go list -m -json all |
									docker run --rm -i sonatypecommunity/nancy:latest sleuth`,
							); err != nil {
								return fmt.Errorf("nancy: %w", err)
							}

							// TODO: ensure govulncheck instead
							// TODO: move version out
							if _, _, err := mk.ExecContext(ctx.Context,
								"go", "install", "golang.org/x/vuln/cmd/govulncheck@latest",
							); err != nil {
								return fmt.Errorf("install govulncheck: %w", err)
							}

							if _, _, err := mk.ExecContext(ctx.Context,
								"govulncheck", "./...",
							); err != nil {
								return fmt.Errorf("govulncheck: %w", err)
							}

							return nil
						},
					},
					{
						Name:  "outdated",
						Usage: "list outdated dependencies",
						Action: func(ctx *cli.Context) error {
							// TODO: simplify?
							// TODO: move version out
							if _, _, err := mk.ExecContext(ctx.Context,
								"bash", "-c", `go list -u -m -json all |
									docker run --rm -i psampaz/go-mod-outdated:latest -update -direct`,
							); err != nil {
								return fmt.Errorf("go-mod-outdated: %w", err)
							}

							return nil
						},
					},
				},
			},
			{
				Name:  "gen",
				Usage: "run all source generators",
				Action: func(ctx *cli.Context) error {
					// TODO: gen mock
					// TODO: gen grpc
					return nil
				},
				Subcommands: []*cli.Command{
					{
						Name:  "mock",
						Usage: "generate mocks go sources",
						Action: func(ctx *cli.Context) error {
							// TODO: simplify?
							// TODO: move version out
							if _, _, err := mk.ExecContext(ctx.Context,
								"bash", "-c", `go install github.com/golang/mock/mockgen@v1.6.0
									mockgen \
										-source ./internal/$(APP_NAME)/logic/data_provider.go \
										-destination ./internal/$(APP_NAME)/logic/mock_logic/data_provider_mocks.go
									mockgen \
										-source ./internal/$(APP_NAME)/logic/logic.go \
										-destination ./internal/$(APP_NAME)/logic/mock_logic/logic_mocks.go`,
							); err != nil {
								return fmt.Errorf("generate mocks: %w", err)
							}

							return nil
						},
					},
					{
						Name:  "grpc",
						Usage: "generate grpc go sources",
						Action: func(ctx *cli.Context) error {
							// TODO: simplify?
							// TODO: move version out
							if _, _, err := mk.ExecContext(ctx.Context,
								"bash", "-c", `
# install
wget -qO ${GRPC_INSTALL_FILENAME} ${GRPC_INSTALL_SOURCE}
unzip -qod third_party/protoc ${GRPC_INSTALL_FILENAME}
rm -f ${GRPC_INSTALL_FILENAME}
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2

#GRPC_PKG_DIR:=./pkg/api/grpc
#GRPC_API_PROTO_PATH:=./api/grpc

# generate
	mkdir -p $(GRPC_PKG_DIR)
	./third_party/protoc/bin/protoc \
		-I ./third_party/protoc/include/google/protobuf \
		-I ./third_party/protoc/include/google/protobuf/compiler \
		-I $(GRPC_API_PROTO_PATH) \
		--go_out=$(GRPC_PKG_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(GRPC_PKG_DIR) \
		--go-grpc_opt=paths=source_relative \
		$(GRPC_API_PROTO_PATH)/*.proto`,
							); err != nil {
								return fmt.Errorf("generate grpc: %w", err)
							}

							return nil
						},
					},
				},
			},
			{
				Name:  "swagger-ui", // or some <img>
				Usage: "start, stop or build <some docker image>",
				// TODO: implement
				Subcommands: []*cli.Command{
					{
						Name:  "stop",
						Usage: "stop <some docker image>",
						// TODO: implement
					},
					{
						Name:  "build",
						Usage: "build <some docker image> image",
						// TODO: implement
					},
				},
			},
			{
				Name:  "migrate",
				Usage: "manage database migrations",
				Subcommands: []*cli.Command{
					{
						Name:  "create",
						Usage: "create migration",
						Action: func(ctx *cli.Context) error {
							// TODO: simplify
							if _, _, err := mk.ExecContext(ctx.Context,
								"bash", "-c", `
go install github.com/charmbracelet/gum@latest
$(eval name=$(shell gum input --placeholder "migration name"))
@echo $(name)
`); err != nil {
								return fmt.Errorf("create migration: %w", err)
							}

							return nil
						},
					},
					{
						Name:  "up",
						Usage: "run all or several migrations up",
						// TODO: implement
					},
					{
						Name:  "down",
						Usage: "run one or several migrations down",
						// TODO: implement
					},
					{
						Name:  "ls",
						Usage: "list applied migrations",
						// TODO: implement
					},
					{
						Name:  "reset",
						Usage: "reset all applied migrations",
						// TODO: implement
					},
				},
			},
			{
				Name:  "run",
				Usage: "run app",
				// TODO: implement
				Subcommands: []*cli.Command{
					{
						Name:  "subapp",
						Usage: "run subapp",
						// TODO: implement
					},
				},
			},
			{
				Name:  "db",
				Usage: "commands to interact with databases",
				Subcommands: []*cli.Command{
					{
						Name:  "seed",
						Usage: "seed database",
						// TODO: implement
					},
					{
						Name:  "mongo-sh",
						Usage: "open mongosh",
						// TODO: implement
					},
					{
						Name:  "mongo-express",
						Usage: "open mongo-express",
						// TODO: implement
					},
					{
						Name:  "minio-web",
						Usage: "open minio frontend",
						// TODO: implement
					},
					{
						Name:  "redis-cli",
						Usage: "open redis cli",
						// TODO: implement
					},
					{
						Name:  "dump-<table>",
						Usage: "dump table into terminal",
						// TODO: implement
					},
				},
			},
			{
				Name:  "build",
				Usage: "Compile/recompile mk binary for running tasks",
				Action: func(ctx *cli.Context) error {
					if _, _, err := mk.ExecContext(ctx.Context,
						"go", "build",
						"-o", "mk",
						"cmd/mk/main.go",
					); err != nil {
						return fmt.Errorf("build main.go: %w", err)
					}

					return nil
				},
				Subcommands: []*cli.Command{
					{
						Name:  "agent",
						Usage: "Compile mk-agent binary",
						// TODO: watch bool flag
						Action: func(ctx *cli.Context) error {
							if err := agent.BuildLocally(ctx.Context); err != nil {
								return fmt.Errorf("build agent: %w", err)
							}

							return nil
						},
					},
				},
			},
		},
	}).Run(os.Args); err != nil {
		log.Fatal(err.Error())
	}
}
