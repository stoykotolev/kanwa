MAELSTROM := ../maelstrom/maelstrom
CHALLENGES := $(patsubst cmd/%/,%,$(wildcard cmd/*/))

.PHONY: build clean $(CHALLENGES:%=test/%)

build: $(CHALLENGES:%=bin/%)

bin/%:
	go build -o $@ ./cmd/$*

test/echo: bin/echo
	$(MAELSTROM) test -w echo --bin bin/echo --node-count 1 --time-limit 10

clean:
	rm -f $(CHALLENGES:%=bin/%)
