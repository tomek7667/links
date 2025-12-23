package json

import "github.com/tomek7667/links/internal/domain"

func (c *Client) GetLinks() []domain.Link {
	return c.db.Links
}
