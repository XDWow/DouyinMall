package usecase

import "strings"

type AlipayWebConfig struct {
	AppID     string
	PID       string
	NotifyURL string
	ReturnURL string
}

func (c AlipayWebConfig) Normalize() AlipayWebConfig {
	c.AppID = strings.TrimSpace(c.AppID)
	c.PID = strings.TrimSpace(c.PID)
	c.NotifyURL = strings.TrimSpace(c.NotifyURL)
	c.ReturnURL = strings.TrimSpace(c.ReturnURL)
	return c
}
