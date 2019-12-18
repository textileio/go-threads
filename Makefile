docker:
	$(eval VERSION := $(shell git --no-pager describe --abbrev=0 --tags --always))
	docker build -t go-threads:$(VERSION:v%=%) .