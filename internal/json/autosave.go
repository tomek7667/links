package json

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func (c *Client) autosave() {
	b, err := json.Marshal(c.db)
	if err != nil {
		panic(fmt.Errorf("failed to marshal the database: %w", err))
	}
	if err := os.WriteFile(c.Path, b, 0o644); err != nil {
		fmt.Printf("state not saved to the db:\n---\n%s\n---\n", string(b))
		panic(fmt.Errorf("failed to autosave the database: %w", err))
	}
	fmt.Printf("autosaved %s\n", time.Now().Format(time.RFC3339))
}

func (c *Client) Close() {
	c.autosave()
}
