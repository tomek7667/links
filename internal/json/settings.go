package json

import "github.com/tomek7667/links/internal/domain"

func (c *Client) Settings() domain.Settings {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.db.Settings
}

func (c *Client) SaveSettings(s domain.Settings) (domain.Settings, error) {
	s, err := domain.NormalizeSettings(s)
	if err != nil {
		return s, err
	}
	err = c.mutate(func(db *Db) error {
		db.Settings = s
		return nil
	})
	return s, err
}
