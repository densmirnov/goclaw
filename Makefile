VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  = -s -w -X github.com/nextlevelbuilder/goclaw/cmd.Version=$(VERSION)
BINARY   = goclaw

.PHONY: build run clean version up down logs reset bench-baseline bench-pprof bench-compare smoke-control-center

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BINARY) .

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)

version:
	@echo $(VERSION)

COMPOSE = docker compose -f docker-compose.yml -f docker-compose.managed.yml -f docker-compose.selfservice.yml

up:
	$(COMPOSE) up -d --build

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f goclaw

reset:
	$(COMPOSE) down -v
	$(COMPOSE) up -d --build

bench-baseline:
	@./benchmarks/scripts/bench_baseline.sh

bench-pprof:
	@./benchmarks/scripts/bench_pprof.sh

bench-compare:
	@if [ -z "$(BASE)" ] || [ -z "$(NEW)" ]; then \
		echo "Usage: make bench-compare BASE=benchmarks/results/<old_ts> NEW=benchmarks/results/<new_ts>"; \
		exit 1; \
	fi
	@BASE="$(BASE)" NEW="$(NEW)" ./benchmarks/scripts/bench_compare.sh

smoke-control-center:
	@./benchmarks/scripts/smoke_control_center.sh
