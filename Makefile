.PHONY: start stop dev test test-integration

start:
	./scripts/start.sh

stop:
	./scripts/stop.sh

dev:
	kind create cluster --name podustrial --config kind-config.yaml || true
	kind export kubeconfig --name podustrial
	( cd frontend && npm run dev & )
	go run ./cmd/server

test:
	go test ./...
	( cd frontend && npm test )

test-integration:
	kind create cluster --name podustrial-test --config kind-config.yaml || true
	kind export kubeconfig --name podustrial-test
	go test -tags=integration ./...
	kind delete cluster --name podustrial-test
