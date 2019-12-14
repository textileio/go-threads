docker:
	$(eval VERSION := $$(shell ggrep -oP 'const Version = "\K[^"]+' common/version.go))
	docker build -t go-textile-threads:$(VERSION) .