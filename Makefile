docker:
	$(eval VERSION := $(shell git --no-pager describe --abbrev=0 --tags --always))
	docker build -t go-textile-threads:$(VERSION:v%=%) .