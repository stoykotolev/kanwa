package main

import (
	"encoding/json"
	"log"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type BroadcastMessage struct {
	Type    string  `json:"type"`
	Message float64 `json:"message"`
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
	Type     string    `json:"type"`
	Messages []float64 `json:"messages"`
}

func main() {
	n := maelstrom.NewNode()

	// topology := map[string][]string{}
	n.Handle("topology", func(msg maelstrom.Message) error {

		var body TopologyMessage
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		var responseBody = TopologyResponse{Type: "topology_ok"}

		return n.Reply(msg, responseBody)
	})

	values := []float64{}
	n.Handle("broadcast", func(msg maelstrom.Message) error {

		var body BroadcastMessage
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		values = append(values, body.Message)

		var response = BroadcastResponse{
			Type: "broadcast_ok",
		}

		return n.Reply(msg, response)
	})

	n.Handle("read", func(msg maelstrom.Message) error {

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
