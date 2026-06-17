package main

import (
	"encoding/json"
	"log"
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

var (
	mu       sync.RWMutex
	messages = make(map[int]struct{})
)

func addMessages(msg int) {
	mu.Lock()
	defer mu.Unlock()
	messages[msg] = struct{}{}
}

func hasMessage(msg int) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := messages[msg]
	return ok
}

var (
	pending_mu sync.RWMutex
	pending    = make(map[string][]int)
)

func addPending(neighbor string, msg int) {
	pending_mu.Lock()
	defer pending_mu.Unlock()
	pending[neighbor] = append(pending[neighbor], msg)
}

func deletePending(neighbor string, msg int) {
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

var neighbours struct {
	mu   sync.RWMutex
	data []string
}

func main() {
	n := maelstrom.NewNode()

	ticker := time.NewTicker(1 * time.Second)
	shutdown := make(chan bool)

	go func() {

		for {
			select {
			case <-shutdown:
				return
			case <-ticker.C:
				copyMap := make(map[string][]int)
				pending_mu.RLock()
				for k, v := range pending {
					copyMap[k] = make([]int, len(v))
					copy(copyMap[k], v)
				}
				pending_mu.RUnlock()

				for nh, msgs := range copyMap {
					for _, msg := range msgs {
						if err := n.RPC(nh, BroadcastMessage{
							Type:    "broadcast",
							Message: msg,
						}, func(m maelstrom.Message) error {
							deletePending(nh, msg)
							return nil
						}); err != nil {
							log.Println("Failed sending message for neighbor", nh)
						}
					}
				}
			}
		}

	}()

	n.Handle("topology", func(msg maelstrom.Message) error {

		var body TopologyMessage
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		neighbours.mu.Lock()
		defer neighbours.mu.Unlock()
		neighbours.data = body.Topology[n.ID()]

		var responseBody = TopologyResponse{Type: "topology_ok"}

		return n.Reply(msg, responseBody)
	})

	n.Handle("broadcast", func(msg maelstrom.Message) error {

		var body BroadcastMessage
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		seen := hasMessage(body.Message)

		if !seen {
			addMessages(body.Message)

			// defer avoided intentionally; broadcast handler is on the hot path
			neighbours.mu.RLock()
			for _, nh := range neighbours.data {
				addPending(nh, body.Message)
				if err := n.RPC(nh, BroadcastMessage{
					Type:    "broadcast",
					Message: body.Message,
				}, func(msg maelstrom.Message) error {
					deletePending(nh, body.Message)
					return nil
				}); err != nil {
					log.Println("Failed something. ", err.Error())
				}
			}
			neighbours.mu.RUnlock()
		}

		if body.MessageId != nil {
			return n.Reply(msg, BroadcastResponse{
				Type: "broadcast_ok",
			})
		}

		return nil
	})

	n.Handle("read", func(msg maelstrom.Message) error {
		// defer avoided intentionally; read handler is on the hot path
		mu.RLock()
		values := make([]int, 0, len(messages))
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
		ticker.Stop()
		shutdown <- true
		log.Fatal(err)
	}
}
