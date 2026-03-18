package vk

import "fmt"

type Client struct {
	accessToken string
}

func NewClient(accessToken string) *Client {
	return &Client{
		accessToken: accessToken,
	}
}

func (c *Client) GetWall(wallID string) string {
	// mock
	return fmt.Sprintf("https://vk.com/%s :: %s", wallID, c.accessToken)
}
