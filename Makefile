.PHONY: generate fmt test mysql-start mysql-init mysql-migrate-up mysql-migrate-down mysql-migrate-reset mysql-smoke dev-cluster-mysql

generate:
	python3 tools/gen_opcodes.py
	gofmt -w ./internal/protocol

fmt:
	gofmt -w ./cmd ./internal

test: generate fmt
	go test ./...

mysql-start:
	bash scripts/mysql-local.sh start

mysql-init:
	bash scripts/mysql-local.sh init

mysql-migrate-up:
	bash scripts/migrate-mysql.sh up

mysql-migrate-down:
	bash scripts/migrate-mysql.sh down

mysql-migrate-reset:
	bash scripts/migrate-mysql.sh reset

mysql-smoke:
	bash scripts/smoke-mysql.sh

dev-cluster-mysql:
	bash scripts/dev-cluster-mysql.sh
