package json

import (
	"encoding/json"
	"fmt"
	"os"
)

func (c *Client) readdb() error {
	file, err := os.Open(c.Path)
	if err != nil {
		return fmt.Errorf("failed to read the db at '%s': %w", c.Path, err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&c.db); err != nil {
		return fmt.Errorf("failed to json decode db: %w", err)
	}
	return nil
}
