package main

import (
	"log"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type Response struct {
	Type string `json:"type"`
}

func main() {
	n := maelstrom.NewNode()

	n.Handle("add", func(msg maelstrom.Message) error {
		return n.Reply(msg, &Response{
			Type: "add_ok",
		})
	})

	n.Handle("read", func(msg maelstrom.Message) error {
		return n.Reply(msg, &Response{
			Type: "add_ok",
		})
	})

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}
