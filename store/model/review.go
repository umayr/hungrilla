package model

import "time"

type Review struct {
	Author   string
	PostedAt time.Time
	Rating   int
	Text     string
}
