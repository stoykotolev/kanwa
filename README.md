# kanwa

Solutions to the
[Fly.io Distributed Systems Challenges](https://fly.io/dist-sys/)
implemented in Go, tested with [Maelstrom](https://github.com/jepsen-io/maelstrom).

Maelstrom is a workbench built on top of [Jepsen](https://jepsen.io/)
that simulates distributed nodes communicating over a network.
It injects faults (partitions, crashes, message loss) and verifies correctness properties.

## Prerequisites

- Go 1.25+
- [Maelstrom](https://github.com/jepsen-io/maelstrom) —
  clone it at `../maelstrom` relative to this repo (the Makefile expects `../maelstrom/maelstrom`)
- Java 11+ (required by Maelstrom)

## Project structure

```bash
cmd/
  echo/         # Challenge 1 — Echo
  unique-ids/   # Challenge 2 — Unique ID generation
  broadcast/    # Challenge 3a & 3b — Broadcast
```

## Building

```sh
make build        # builds all binaries into bin/
make bin/echo     # build a specific challenge
make clean        # remove built binaries
```

## Challenges

### Challenge 1 — Echo

A node that echoes back any message it receives.
Baseline sanity-check for the Maelstrom setup.

```sh
make test/echo
```

**How it works:** registers a handler for `echo` messages,
swaps the type to `echo_ok`, and replies with the original body unchanged.

---

### Challenge 2 — Unique IDs

Generate globally unique IDs across a cluster under network partitions,
with total availability.

```sh
make test/unique-ids   # 3 nodes, 1000 req/s, with partition nemesis
```

**How it works:** each node produces IDs in the form `<nodeID>-<counter>`.
Because node IDs are unique within the cluster and the counter is
monotonically increasing per node, the combination is globally unique
even without coordination.

---

### Challenge 3a/3b — Broadcast

A gossip broadcast system where every node eventually receives every message.

```sh
make test/broadcast        # 3a — single node
make test/multi-broadcast  # 3b — 5-node cluster
```

**How it works:**

- On `topology`, each node records its neighbours from the provided graph.
- On `broadcast`, if the message hasn't been seen before,
  the node stores it and forwards it to all neighbours (gossip fan-out).
- On `read`, the node returns all messages it has collected.
- A `sync.RWMutex` guards the shared message set so concurrent handlers don't race.

Deduplication (the `has` check before forwarding) prevents
infinite loops in the gossip propagation.

---

## Viewing results

After any test run, start the Maelstrom web UI to inspect timelines,
latency graphs, and checker results:

```sh
make serve
```

Then open the URL printed in the terminal (default `http://localhost:8080`).
