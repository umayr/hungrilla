package model

import "time"

type Restaurant struct {
	ImgURL       string
	URL          string
	Title        string
	DeliveryTime time.Duration
	Type         string
	Rating       int

	Menu         []Meal
	Reviews      []Review
}
