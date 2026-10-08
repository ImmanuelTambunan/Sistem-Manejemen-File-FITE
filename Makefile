.PHONY: db-up db-down api tidy fmt vet build

db-up:      ## jalankan MySQL lokal
	docker compose up -d mysql
db-down:    ## hentikan MySQL lokal
	docker compose down
tidy:
	cd backend && go mod tidy
fmt:
	cd backend && gofmt -w .
vet:
	cd backend && go vet ./...
build:
	cd backend && go build -o bin/api ./cmd/api
api:        ## jalankan API (membaca backend/.env)
	cd backend && set -a && . ./.env && set +a && go run ./cmd/api
