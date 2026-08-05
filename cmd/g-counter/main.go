package main

import (
	"context"
	"encoding/json"
	"log"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type AddBody struct {
	Type  string `json:"type"`
	Delta int    `json:"delta"`
}
type AddResponse struct {
	Type string `json:"type"`
}

type ReadResponse struct {
	Type  string `json:"type"`
	Value int    `json:"value"`
}

func main() {
	n := maelstrom.NewNode()
	kv := maelstrom.NewSeqKV(n)
	ctx, cancel := context.WithCancel(context.Background())

	n.Handle("add", func(msg maelstrom.Message) error {
		var body AddBody
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			log.Printf("Failed unmarshalling add body. %s", err.Error())
		}
		nv := body.Delta
		v, err := kv.ReadInt(ctx, "value")
		if err != nil {
			log.Printf("Failed getting value from kv store. %s", err.Error())
		}
		if err := kv.Write(ctx, "value", nv+v); err != nil {
			log.Printf("Failed setting value in kv store. %s", err.Error())
		}
		return n.Reply(msg, AddResponse{
			Type: "add_ok",
		})
	})

	n.Handle("read", func(msg maelstrom.Message) error {
		v, err := kv.ReadInt(ctx, "value")
		if err != nil {
			log.Printf("Failed getting value from kv store. %s", err.Error())
		}
		return n.Reply(msg, ReadResponse{
			Type:  "read_ok",
			Value: v,
		})
	})

	if err := n.Run(); err != nil {
		cancel()
		log.Fatal(err)
	}
}
