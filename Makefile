MAELSTROM := ../maelstrom/maelstrom
CHALLENGES := $(patsubst cmd/%/,%,$(wildcard cmd/*/))

.PHONY: build clean $(CHALLENGES:%=test/%)

build: $(CHALLENGES:%=bin/%)

bin/%:
	go build -o $@ ./cmd/$*

test/echo: bin/echo
	$(MAELSTROM) test -w echo --bin bin/echo --node-count 1 --time-limit 10

test/unique-ids: bin/echo
	$(MAELSTROM) test -w unique-ids --bin bin/unique-ids --time-limit 30 --rate 1000 --node-count 3 --availability total --nemesis partition

clean:
	rm -f $(CHALLENGES:%=bin/%)

serve:
	$(MAELSTROM) serve
