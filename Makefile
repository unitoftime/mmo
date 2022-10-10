.PHONY: build deploy

build:
	echo "BUILDING"
	$(MAKE) -C ./cmd/ all

deploy:
	echo "DEPLOYING"
