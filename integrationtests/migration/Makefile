test_migration:
	export $ENV_FILENAME="thread_ids"
	export $ENV_NUM_RECORDS="10"
	export $ENV_NUM_THREADS="10"
	cd generate && ENV_REPO_PATH="../generated" go run main.go
	ENV_REPO_PATH="generated" go test ./...
	rm -rf generated
