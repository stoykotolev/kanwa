MAELSTROM := ../maelstrom/maelstrom
CHALLENGES := $(patsubst cmd/%/,%,$(wildcard cmd/*/))

.PHONY: build clean $(CHALLENGES:%=test/%)

build: $(CHALLENGES:%=bin/%)

bin/%:
	go build -o $@ ./cmd/$*

test/echo: bin/echo
	$(MAELSTROM) test -w echo --bin bin/echo --node-count 1 --time-limit 10

test/unique-ids: bin/unique-ids
	$(MAELSTROM) test -w unique-ids --bin bin/unique-ids --time-limit 30 --rate 1000 --node-count 3 --availability total --nemesis partition

test/broadcast: bin/broadcast
	$(MAELSTROM) test -w broadcast --bin bin/broadcast --node-count 1 --time-limit 20 --rate 10

test/multi-broadcast: bin/broadcast
	$(MAELSTROM) test -w broadcast --bin bin/broadcast --node-count 5 --time-limit 20 --rate 10

clean:
	rm -f $(CHALLENGES:%=bin/%)

serve:
	$(MAELSTROM) serve
