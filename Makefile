MAELSTROM := ../maelstrom/maelstrom
CHALLENGES := $(patsubst cmd/%.go,%,$(wildcard cmd/*.go))

.PHONY: build clean $(CHALLENGES:%=test/%)

build: $(CHALLENGES:%=bin/%)

bin/%: cmd/%.go
	go build -o $@ ./$<

test/echo: bin/echo
	$(MAELSTROM) test -w echo --bin bin/echo --node-count 1 --time-limit 10

clean:
	rm -f $(CHALLENGES:%=bin/%)
