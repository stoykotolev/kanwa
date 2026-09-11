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
  echo/         # Challenge 1  — Echo
  unique-ids/   # Challenge 2  — Unique ID generation
  broadcast/    # Challenge 3a–3e — Broadcast (single node → efficient, fault-tolerant gossip)
  g-counter/    # Challenge 4  — Grow-only counter
store/          # Maelstrom run history (results.edn, logs, timelines) — one directory per run
```

## Building

```sh
make build        # builds all binaries into bin/
make bin/echo     # build a specific challenge
make clean        # remove built binaries
```

## Results at a glance

All numbers come from `results.edn` of the run named in the last column
(under `store/<workload>/<run>/`). Messages/op is `:net :servers :msgs-per-op`,
the figure the Fly.io challenge targets refer to; latencies are `:stable-latencies` in ms.

| Challenge         | Cluster                                        | Load             | Ops    | Msgs/op (servers)       | Median / max stable latency               | Result                                                               | Run                          |
| ----------------- | ---------------------------------------------- | ---------------- | ------ | ----------------------- | ----------------------------------------- | -------------------------------------------------------------------- | ---------------------------- |
| 1 Echo            | 1 node                                         | 5 req/s, 10 s    | 49     | 0                       | —                                         | ✅ valid                                                             | `echo/20260522T230323`       |
| 2 Unique IDs      | 3 nodes, partition nemesis, total availability | 1000 req/s, 30 s | 25 001 | 0                       | —                                         | ✅ valid, 0 failed                                                   | `unique-ids/20260522T234200` |
| 3a Broadcast      | 1 node                                         | 10 req/s, 20 s   | 192    | 0                       | 0 / 0                                     | ✅ valid                                                             | `broadcast/20260730T091233`  |
| 3b Multi-node     | 5 nodes                                        | 10 req/s, 20 s   | 198    | 3.04                    | 0 / 0                                     | ✅ valid                                                             | `broadcast/20260730T085341`  |
| 3c Fault-tolerant | 5 nodes, partition nemesis                     | 10 req/s, 20 s   | 175    | 3.14                    | 4 850 / 15 605                            | ✅ valid — 0 lost, 0 duplicated, 69 stale reads during the partition | `broadcast/20260730T085429`  |
| 3d Efficient I    | 25 nodes, 100 ms link latency                  | 100 req/s, 20 s  | 1 794  | **13.17** (target < 30) | **345 / 411** (targets < 400 / < 600)     | ✅ valid                                                             | `broadcast/20260730T085516`  |
| 3e Efficient II   | same run as 3d                                 |                  |        | **13.17** (target < 20) | **345 / 411** (targets < 1 000 / < 2 000) | ✅ valid                                                             | `broadcast/20260730T085516`  |
| 4 G-counter       | 3 nodes, partition nemesis                     | 100 req/s, 20 s  | 1 692  | 4.83                    | —                                         | ✅ valid, final reads agree (1214)                                   | `g-counter/20260911T085317`  |

The 3c latencies are dominated by the partition itself: a value written on one side
can only cross once the network heals and the next sync tick fires, so the
median is seconds, not milliseconds. That is expected and the checker accepts it.

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
even without coordination. 25 001 IDs generated in 30 s with zero
failures while the network was being partitioned.

---

### Challenge 3a/3b/3c — Broadcast

A gossip broadcast system where every node eventually receives every message,
including across network partitions.

```sh
make test/broadcast        # 3a — single node
make test/multi-broadcast  # 3b — 5-node cluster
make test/ft-broadcast     # 3c — 5-node cluster with partition nemesis
```

**How it works:**

- On `topology`, each node computes its own neighbours (see 3d below —
  the topology Maelstrom suggests is ignored in favour of a fixed tree).
- On `broadcast`, if the message hasn't been seen before,
  the node stores it, sends it to every neighbour except the sender
  (fire-and-forget), and records it as _pending_ for each of those neighbours.
- A ticker goroutine wakes once per second and sends each neighbour one
  `sync` RPC carrying all its pending values. Pending entries are only
  cleared when the neighbour acknowledges the `sync`, so a partitioned
  neighbour keeps receiving the batch until it can answer — this is what
  makes 3c pass with `lost-count 0`.
- On `read`, the node returns all messages it has collected.
- `sync.RWMutex` guards the seen-set, the pending map and the neighbour list.

Deduplication (the `has` check before forwarding) prevents
infinite loops in the gossip propagation.

---

### Challenge 3d/3e — Efficient Broadcast

Same workload, 25 nodes, 100 ms injected latency on every link, 100 req/s.
Targets: 3d wants < 30 msgs/op, median < 400 ms, max < 600 ms;
3e tightens to < 20 msgs/op with a looser < 1 s / < 2 s latency budget.

```sh
make test/ebo   # 25 nodes, --latency 100, --rate 100
```

One implementation satisfies both. How the numbers came down, run by run
(all 25-node runs, `:servers :msgs-per-op` and stable latencies from `results.edn`):

| Stage                                                      | Msgs/op   | Median     | Max        | Run               |
| ---------------------------------------------------------- | --------- | ---------- | ---------- | ----------------- |
| Maelstrom's default grid topology, RPC for everything      | 94.26     | 468 ms     | 796 ms     | `20260708T194224` |
| + don't send a message back to the node it came from       | 65.91     | 460 ms     | 813 ms     | `20260715T201556` |
| Depth-2 tree topology, per-value acks                      | 28.01     | 356 ms     | 448 ms     | `20260727T235635` |
| Tree + fire-and-forget broadcast + batched `sync` per tick | **13.17** | **345 ms** | **411 ms** | `20260730T085516` |

**Topology.** The grid Maelstrom proposes has a diameter that makes the
latency target impossible at 100 ms per hop. The nodes instead build a
fixed 5-ary tree: `n0` is the root with children `n1`–`n4`, and every other
node `nK` has parent `n(K/5)` and children `n(5K)`–`n(5K+4)` when those exist.
For 25 nodes that is depth 2 and the longest path (leaf → root → leaf) is
4 hops, which fits inside the 400 ms median. A star (everything through
`n0`) would have a shorter diameter but concentrates all the fan-out on one node.

**Message count.** The first big cut is not echoing a message back to its
sender. The second is the tree itself (each message crosses each edge once
instead of flooding a grid). The last, and largest, is switching from one
RPC per value to one `sync` per neighbour per tick that carries every
pending value: one round-trip amortised over the whole batch.

**Trade-off.** Batching on a 1 s ticker buys the message budget at the
cost of up to a second of extra propagation delay for anything the
fire-and-forget send didn't deliver. The fire-and-forget path keeps the
happy-path latency low; the batched `sync` is the safety net.

---

### Challenge 4 — Grow-only counter

A counter that any node can increment and that every node eventually
reads the same value from, on top of Maelstrom's sequentially consistent
`seq-kv` store, under partitions.

```sh
make test/gc   # 3 nodes, 100 req/s, partition nemesis
```

**How it works:**

- `add` reads the current value from `seq-kv` and issues a compare-and-swap
  `old → old + delta`. If the CAS fails with `PreconditionFailed`, another
  node got there first: re-read and retry until it lands. Nothing is
  acknowledged until the write is actually in the store.
- `read` reads the value and then performs a no-op CAS `v → v` against it.
  `seq-kv` is only sequentially consistent, so a plain read may be stale;
  a successful CAS on the same value proves the read was current. If the
  CAS fails, re-read and try again.

Result: 594 adds and 1 098 reads, all successful, across a partition;
every node's final read agreed on 1214. About 4.8 KV messages per client op
is the price of the CAS retry loops.

**Alternative.** A CRDT G-counter (one slot per node, merged by summing)
would need no shared store and no retries; the `seq-kv` + CAS approach was
chosen to exercise the compare-and-swap path instead.

---

## Viewing results

After any test run, start the Maelstrom web UI to inspect timelines,
latency graphs, and checker results:

```sh
make serve
```

Then open the URL printed in the terminal (default `http://localhost:8080`).
Every run's raw `results.edn`, `jepsen.log` and history live under `store/`.
