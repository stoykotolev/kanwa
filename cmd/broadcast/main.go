package main

import (
	"encoding/json"
	"log"
	"sync"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type BroadcastMessage struct {
	Type      string  `json:"type"`
	MessageId *int    `json:"msg_id"`
	Message   float64 `json:"message"`
}

type BroadcastResponse struct {
	Type string `json:"type"`
}

type BroadcastNeighbour struct {
	Type     string    `json:"type"`
	Messages []float64 `json:"messages"`
}

type TopologyMessage struct {
	Type     string              `json:"type"`
	Topology map[string][]string `json:"topology"`
}

type TopologyResponse struct {
	Type string `json:"type"`
}

type ReadResponse struct {
	Type     string    `json:"type"`
	Messages []float64 `json:"messages"`
}

var (
	mu       sync.RWMutex
	messages = make(map[float64]struct{})
)

// Add
func add_messages(msg float64) {
	mu.Lock()
	defer mu.Unlock()
	messages[msg] = struct{}{}
}

// Check
func has_message(msg float64) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := messages[msg]
	return ok
}

var (
	pending_mu sync.RWMutex
	pending    = make(map[string][]float64)
)

// Add
func add_pending(neighbor string, msg float64) {
	pending_mu.Lock()
	defer pending_mu.Unlock()
	pending[neighbor] = append(pending[neighbor], msg)
}

func delete_pending(neighbor string, msg float64) {
	pending_mu.Lock()
	defer pending_mu.Unlock()
	msgs := pending[neighbor]
	for i, m := range msgs {
		if m == msg {
			pending[neighbor] = append(msgs[:i], msgs[i+1:]...)
			return
		}
	}
}

func main() {
	n := maelstrom.NewNode()

	neighbours := []string{}

	n.Handle("topology", func(msg maelstrom.Message) error {

		var body TopologyMessage
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		neighbours = body.Topology[n.ID()]

		var responseBody = TopologyResponse{Type: "topology_ok"}

		return n.Reply(msg, responseBody)
	})

	n.Handle("broadcast", func(msg maelstrom.Message) error {

		var body BroadcastMessage
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		seen := has_message(body.Message)

		if !seen {
			add_messages(body.Message)
			for _, nh := range neighbours {
				add_pending(nh, body.Message)
				n.RPC(nh, BroadcastMessage{
					Type:    "broadcast",
					Message: body.Message,
				}, func(msg maelstrom.Message) error {
					delete_pending(nh, body.Message)
					return nil
				})
			}
		}

		if body.MessageId != nil {
			return n.Reply(msg, BroadcastResponse{
				Type: "broadcast_ok",
			})
		}

		return nil
	})

	n.Handle("read", func(msg maelstrom.Message) error {
		mu.RLock()
		values := make([]float64, 0, len(messages))
		for k := range messages {
			values = append(values, k)
		}
		mu.RUnlock()

		var body = ReadResponse{
			Type:     "read_ok",
			Messages: values,
		}

		return n.Reply(msg, body)
	})

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}
