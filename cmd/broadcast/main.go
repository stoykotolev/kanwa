package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type BroadcastMessage struct {
	Type      string `json:"type"`
	MessageId *int   `json:"msg_id"`
	Message   int    `json:"message"`
}

type BroadcastResponse struct {
	Type string `json:"type"`
}

type TopologyMessage struct {
	Type     string              `json:"type"`
	Topology map[string][]string `json:"topology"`
}

type TopologyResponse struct {
	Type string `json:"type"`
}

type ReadResponse struct {
	Type     string `json:"type"`
	Messages []int  `json:"messages"`
}

type SyncMessage struct {
	Type    string `json:"type"`
	Pending []int  `json:"pending"`
}
type SyncResponse struct {
	Type string `json:"type"`
}

type Node struct {
	parent   string
	children []string
}

var seen = struct {
	mu       sync.RWMutex
	messages map[int]struct{}
}{
	messages: make(map[int]struct{}),
}

func addMessages(msg int) {
	seen.mu.Lock()
	defer seen.mu.Unlock()
	seen.messages[msg] = struct{}{}
}

func hasMessage(msg int) bool {
	seen.mu.RLock()
	defer seen.mu.RUnlock()
	_, ok := seen.messages[msg]
	return ok
}

var pending = struct {
	mu       sync.RWMutex
	messages map[string][]int
}{
	messages: make(map[string][]int),
}

func addPending(neighbor string, msg int) {
	pending.mu.Lock()
	defer pending.mu.Unlock()
	pending.messages[neighbor] = append(pending.messages[neighbor], msg)
}

func deletePending(neighbor string, msg int) {
	pending.mu.Lock()
	defer pending.mu.Unlock()
	msgs := pending.messages[neighbor]
	for i, m := range msgs {
		if m == msg {
			pending.messages[neighbor] = append(msgs[:i], msgs[i+1:]...)
			return
		}
	}
}

var neighbours struct {
	mu   sync.RWMutex
	data []string
}

func main() {
	n := maelstrom.NewNode()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				copyMap := make(map[string][]int)
				pending.mu.RLock()
				for k, v := range pending.messages {
					copyMap[k] = make([]int, len(v))
					copy(copyMap[k], v)
				}
				pending.mu.RUnlock()

				for nh, values := range copyMap {
					if len(values) == 0 {
						continue
					}
					if err := n.RPC(nh, SyncMessage{
						Type:    "sync",
						Pending: values,
					}, func(msg maelstrom.Message) error {
						for _, msg := range values {
							deletePending(nh, msg)
						}
						return nil
					}); err != nil {
						log.Println("Failed sending message for neighbor", nh)
					}
				}
			}
		}
	})

	n.Handle("sync", func(msg maelstrom.Message) error {
		var body SyncMessage
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}
		// body.pending
		for _, val := range body.Pending {
			forwardMsg(val, msg.Src, n)
		}

		return n.Reply(msg, SyncResponse{
			Type: "sync_ok",
		})
	})

	n.Handle("topology", func(msg maelstrom.Message) error {
		var body TopologyMessage
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		topo := make(map[string]*Node)

		for n := range body.Topology {
			node := &Node{}
			id, err := strconv.Atoi(n[1:])
			if err != nil {
				log.Println("failed conversion", err)
				continue
			}

			// id is 0, this is root
			if id == 0 {
				children := make([]string, 0, 4)
				for i := 1; i <= 4 && i < len(body.Topology); i++ {
					children = append(children, fmt.Sprintf("n%d", i))
				}
				node.children = children
				topo["n0"] = node
				continue
			}

			// calculate index for the rest;
			idx := id / 5
			node.parent = "n" + strconv.Itoa(idx)

			if id*5 < len(body.Topology) {
				children := make([]string, 0, 5)
				for i := range 5 {
					children = append(children, fmt.Sprintf("n%d", 5*id+i))
				}
				node.children = children
			}

			topo[n] = node
		}

		neighbours.mu.Lock()
		defer neighbours.mu.Unlock()
		neighbours.data = append([]string{}, topo[n.ID()].children...)
		p := topo[n.ID()].parent
		if p != "" {
			neighbours.data = append(neighbours.data, p)
		}

		responseBody := TopologyResponse{Type: "topology_ok"}

		return n.Reply(msg, responseBody)
	})

	n.Handle("broadcast", func(msg maelstrom.Message) error {
		var body BroadcastMessage
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		forwardMsg(body.Message, msg.Src, n)

		if body.MessageId != nil {
			return n.Reply(msg, BroadcastResponse{
				Type: "broadcast_ok",
			})
		}

		return nil
	})

	n.Handle("read", func(msg maelstrom.Message) error {
		// defer avoided intentionally; read handler is on the hot path
		seen.mu.RLock()
		values := make([]int, 0, len(seen.messages))
		for k := range seen.messages {
			values = append(values, k)
		}
		seen.mu.RUnlock()

		body := ReadResponse{
			Type:     "read_ok",
			Messages: values,
		}

		return n.Reply(msg, body)
	})

	if err := n.Run(); err != nil {
		cancel()
		wg.Wait()
		log.Fatal(err)
	}
}

func forwardMsg(msg int, sender string, n *maelstrom.Node) {
	msgSeen := hasMessage(msg)

	if !msgSeen {
		addMessages(msg)

		// defer avoided intentionally; broadcast handler is on the hot path
		neighbours.mu.RLock()
		for _, nh := range neighbours.data {
			if sender == nh {
				continue
			}
			addPending(nh, msg)
			if err := n.Send(nh, BroadcastMessage{
				Type:    "broadcast",
				Message: msg,
			}); err != nil {
				log.Println("Failed something. ", err.Error())
			}
		}
		neighbours.mu.RUnlock()
	}
}
