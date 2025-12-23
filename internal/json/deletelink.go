package json

import (
	"slices"

	"github.com/tomek7667/links/internal/domain"
)

func (c *Client) DeleteLink(url string) {
	c.m.Lock()
	idx := slices.IndexFunc(c.db.Links, func(l domain.Link) bool {
		return l.Url == url
	})
	if idx != -1 {
		c.db.Links = append(c.db.Links[:idx], c.db.Links[idx+1:]...)
	}
	go c.autosave()
	c.m.Unlock()
}
