.EXPORT_ALL_VARIABLES:

RUN_MIGRATION_TEST="true"
ENV_FILENAME="thread_ids"
ENV_NUM_RECORDS="10"
ENV_NUM_THREADS="10"

test-migration:
	cd generate && ENV_REPO_PATH="../generated" go run main.go
	ENV_REPO_PATH="generated" go test ./... || rm -rf generated
	rm -rf generated
