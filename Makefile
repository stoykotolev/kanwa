MAELSTROM := ../maelstrom/maelstrom
CHALLENGES := $(patsubst cmd/%/,%,$(wildcard cmd/*/))

.PHONY: build clean FORCE $(CHALLENGES:%=test/%)

build: $(CHALLENGES:%=bin/%)

bin/%: FORCE
	go build -o $@ ./cmd/$*

FORCE:

test/echo: bin/echo
	$(MAELSTROM) test -w echo --bin bin/echo --node-count 1 --time-limit 10

test/unique-ids: bin/unique-ids
	$(MAELSTROM) test -w unique-ids --bin bin/unique-ids --time-limit 30 --rate 1000 --node-count 3 --availability total --nemesis partition

test/broadcast: bin/broadcast
	$(MAELSTROM) test -w broadcast --bin bin/broadcast --node-count 1 --time-limit 20 --rate 10

test/multi-broadcast: bin/broadcast
	$(MAELSTROM) test -w broadcast --bin bin/broadcast --node-count 5 --time-limit 20 --rate 10

test/ft-broadcast: bin/broadcast
	$(MAELSTROM) test -w broadcast --bin bin/broadcast --node-count 5 --time-limit 20 --rate 10 --nemesis partition

test/ebo: bin/broadcast
	$(MAELSTROM) test -w broadcast --bin bin/broadcast --node-count 25 --time-limit 20 --rate 100 --latency 100

test/gc: bin/g-counter 
	$(MAELSTROM) test -w g-counter --bin bin/g-counter --node-count 3 --rate 100 --time-limit 20 --nemesis partition

clean:
	rm -f $(CHALLENGES:%=bin/%)

serve:
	$(MAELSTROM) serve
