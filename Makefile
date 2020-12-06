threaddb-up:
	docker-compose -f docker-compose-dev.yml up --build

threaddb-stop:
	docker-compose -f docker-compose-dev.yml stop

threaddb-clean:
	docker-compose -f docker-compose-dev.yml down -v --remove-orphans

test:
	go test -race -timeout 45m ./...
.PHONY: test