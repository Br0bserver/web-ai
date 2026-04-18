.PHONY: build rebuild frontend backend docker clean

BINARY := web-ai

build: frontend backend

rebuild: clean build

frontend:
	@if [ ! -d frontend/node_modules ]; then npm --prefix frontend install; fi
	npm --prefix frontend run build

backend:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY) ./cmd/server/
	@rm -f server
	@ls -lh $(BINARY)

docker:
	docker build -t web-ai .

clean:
	rm -rf static/dist/* $(BINARY) server frontend/node_modules
