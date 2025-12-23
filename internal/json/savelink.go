package json

import (
	"slices"

	"github.com/tomek7667/links/internal/domain"
)

func (c *Client) SaveLink(link domain.Link) {
	c.m.Lock()
	idx := slices.IndexFunc(c.db.Links, func(l domain.Link) bool {
		return l.Url == link.Url
	})
	if idx == -1 {
		c.db.Links = append(c.db.Links, link)
	} else {
		c.db.Links[idx] = link
	}
	go c.autosave()
	c.m.Unlock()
}
