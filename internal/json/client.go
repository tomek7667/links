package json

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"sync"

	"github.com/tomek7667/links/internal/domain"
)

const DefaultPath = "./links.db.json"

type Client struct {
	Path string
	mu   sync.Mutex
	db   Db
}

type Db struct {
	Links    []domain.Link   `json:"links"`
	Settings domain.Settings `json:"settings"`
}

func New() (*Client, error) {
	c := &Client{Path: DefaultPath}

	_, err := os.Stat(c.Path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		c.db = Db{Links: []domain.Link{}}
		if err := c.saveLocked(); err != nil {
			return nil, fmt.Errorf("failed to create default db: %w", err)
		}
		return c, nil
	case err != nil:
		return nil, fmt.Errorf("failed to stat the db at '%s': %w", c.Path, err)
	}

	if err := c.load(); err != nil {
		return nil, fmt.Errorf("failed to load the database: %w", err)
	}
	return c, nil
}

// load reads the database and gives IDs to links saved by older versions.
func (c *Client) load() error {
	b, err := os.ReadFile(c.Path)
	if err != nil {
		return fmt.Errorf("failed to read the db at '%s': %w", c.Path, err)
	}
	var db Db
	if err := json.Unmarshal(b, &db); err != nil {
		return fmt.Errorf("failed to json decode db: %w", err)
	}
	if db.Links == nil {
		db.Links = []domain.Link{}
	}

	migrated := false
	seen := make(map[string]bool, len(db.Links))
	for i := range db.Links {
		if db.Links[i].ID == "" || seen[db.Links[i].ID] {
			db.Links[i].ID = newID()
			migrated = true
		}
		seen[db.Links[i].ID] = true
	}

	c.db = db
	if migrated {
		return c.saveLocked()
	}
	return nil
}

// saveLocked writes the database atomically (temp file + rename). The caller
// must hold c.mu, or be the only user of c.
func (c *Client) saveLocked() error {
	b, err := json.MarshalIndent(c.db, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal the database: %w", err)
	}
	tmp := c.Path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("failed to write the database: %w", err)
	}
	if err := os.Rename(tmp, c.Path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to replace the database: %w", err)
	}
	return nil
}

// mutate applies fn to the database and persists it. If fn fails or the save
// fails, the in-memory state is rolled back so it never drifts from disk.
func (c *Client) mutate(fn func(db *Db) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	prev := Db{Links: slices.Clone(c.db.Links), Settings: c.db.Settings}
	if err := fn(&c.db); err != nil {
		c.db = prev
		return err
	}
	if err := c.saveLocked(); err != nil {
		c.db = prev
		return err
	}
	return nil
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.saveLocked(); err != nil {
		fmt.Printf("failed to save the database on close: %v\n", err)
	}
}

func newID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Errorf("failed to generate id: %w", err))
	}
	return hex.EncodeToString(b)
}
