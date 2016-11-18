package crawler

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/umayr/inhuman"

	"github.com/umayr/hungrilla/conf"
	"github.com/umayr/hungrilla/store/model"

	log "github.com/Sirupsen/logrus"
	"golang.org/x/net/html"
	"strings"
	"time"
)

type ChanItem chan model.Restaurant
type ChanDone chan bool
type ChanErr chan error

type Crawler struct {
	maxPages int
	baseUrl  string
	city     string
	uri      string

	chRest ChanItem
	chErr  ChanErr

	wgOuter sync.WaitGroup
	wgInner sync.WaitGroup

	restaurants []model.Restaurant
	errors      []error

	async bool

	outItem ChanItem
	outErr  ChanErr
	outDone ChanDone
}

type Config struct {
	MaxPages int
	BaseUrl  string
	City     string

	Async bool

	OutItem ChanItem
	OutErr  ChanErr
	OutDone ChanDone
}

func New() *Crawler {
	log.Debug("[crawler] Creating a new instance")
	c := conf.Get()
	return &Crawler{
		maxPages: c.MaxPages,
		baseUrl:  c.BaseURL,
		city:     c.City,
		uri:      fmt.Sprintf("%s/%s/delivery", c.BaseURL, c.City),
		chRest:   make(ChanItem),
		chErr:    make(ChanErr),
	}
}

func NewWithConfig(c *Config) *Crawler {
	log.Debug("[crawler] Creating a new instance with config")
	cnf := conf.Get()

	crw := &Crawler{}
	if c.Async {
		if c.OutItem == nil || c.OutDone == nil || c.OutErr == nil {
			panic("channels should be provided when running in async mode")
		}

		crw.async = c.Async
		crw.outItem, crw.outDone, crw.outErr = c.OutItem, c.OutDone, c.OutErr
	} else {
		crw.async = false
	}

	if c.MaxPages != 0 {
		crw.maxPages = c.MaxPages
	} else {
		crw.maxPages = cnf.MaxPages
	}

	if c.BaseUrl != "" {
		crw.baseUrl = c.BaseUrl
	} else {
		crw.baseUrl = cnf.BaseURL
	}

	if c.City != "" {
		crw.city = c.City
	} else {
		crw.city = cnf.City
	}

	crw.uri = fmt.Sprintf("%s/%s/delivery", crw.baseUrl, crw.city)
	crw.chRest = make(ChanItem)
	crw.chErr = make(ChanErr)

	return crw
}

func (t *Crawler) Begin() {
	t0 := time.Now()
	log.WithField("max-pages", t.maxPages).Debug("[crawler] Starting crawler")
	t.wgOuter.Add(t.maxPages)

	for i := 0; i < t.maxPages; i++ {
		go t.pull(i)
	}

	t.wgOuter.Wait()
	t1 := time.Now()
	log.Debugf("[crawler] Fetched %d restaurant with %d errors in %fsecs", len(t.restaurants), len(t.errors), t1.Sub(t0).Seconds())
}

func (t *Crawler) pull(pno int) {
	url := fmt.Sprintf("%s?&Search_PageNo=%d", t.uri, pno)
	log.WithField("url", url).Debugf("[crawler] Pulling page #%d", pno)

	doc, err := goquery.NewDocument(url)
	if err != nil {
		t.chErr <- err
		log.WithError(err).Error("[crawler] Error while fetching document")
		return
	}

	nodes := doc.Find("section#listing-container > article")
	log.Debugf("[crawler] Found %d nodes on page %d", nodes.Length(), pno)
	t.wgInner.Add(nodes.Length())

	nodes.Each(func(_ int, node *goquery.Selection) {
		go t.outer(node)
	})

	defer t.wgOuter.Done()

	go func() {
		for {
			select {
			case r := <-t.chRest:
				t.restaurants = append(t.restaurants, r)
			case err := <-t.chErr:
				t.errors = append(t.errors, err)
			}
		}
	}()

	t.wgInner.Wait()
	return
}

func (t *Crawler) outer(n *goquery.Selection) {
	log.Debug("[crawler] Parsing outer information for a restaurant")
	defer t.wgInner.Done()

	r := model.Restaurant{}
	imgUrl, exists := n.Find(".item-pic > img").First().Attr("src")
	if exists {
		r.ImgURL = imgUrl
	}

	url, exists := n.Find(".item-pic > a").First().Attr("href")
	if exists {
		r.URL = url
	}

	r.Title = n.Find(".item-title > a").Text()

	rating, exists := n.Find(".item-title > span.item-star-rating").Attr("data-rating")
	if exists {
		i64, err := strconv.ParseInt(rating, 10, 64)
		if err != nil {
			t.chErr <- err
			log.WithError(err).WithField("rating", rating).Error("[crawler] Error parsing rating")
			return
		}

		r.Rating = int(i64)
	}

	r.Type = n.Find(".item-meta > .item-address").Text()

	deliveryTime, err := inhuman.Parse(n.Find(".item-meta .row-fluid .span4").First().Children().Last().Text())
	if err != nil {
		t.chErr <- err
		log.WithError(err).WithField("delivery-time", n.Find(".item-meta .row-fluid .span4").First().Children().Last().Text()).Error("[crawler] Error parsing delivery time")
		return
	}
	r.DeliveryTime = deliveryTime

	log.WithFields(log.Fields{
		"img-url":       r.ImgURL,
		"url":           r.URL,
		"title":         r.Title,
		"rating":        r.Rating,
		"type":          r.Type,
		"delivery-time": r.DeliveryTime,
	}).Debug("[crawler] Parsed restaurant")
	t.chRest <- t.inner(r)
}

func (t *Crawler) inner(r model.Restaurant) model.Restaurant {
	url := fmt.Sprintf("%s%s", t.baseUrl, r.URL)
	log.Debugf("[crawler] Requesting url %s", url)

	doc, err := goquery.NewDocument(url)
	if err != nil {
		t.chErr <- err
		log.WithError(err).Error("[crawler] Error while fetching document")
		return r
	}

	doc.Find(".tab-pane.mspan7-menu").Each(func(_ int, n *goquery.Selection) {
		meal := model.Meal{}

		cat := n.Find("h4").Text()
		meal.Type = cat

		n.Find(".menu-item").Each(func(_ int, i *goquery.Selection) {
			log.Debug("[crawler] Parsing menu items")

			title := strings.TrimSpace(
				i.
					Find(".menu-item-name").
					Contents().
					FilterFunction(func(_ int, s *goquery.Selection) bool {
						return s.Nodes[0].Type == html.TextNode
					}).
					Text(),
			)
			desc := i.Find(".menu-item-name > small").Text()

			meal.Name = title
			meal.Description = desc

			i.Find(".menu-subitems > .menu-subitem").Each(func(_ int, s *goquery.Selection) {
				kind := strings.TrimSpace(s.Find(".subitem-name").Text())

				p, exists := s.Find("input[type=hidden]#ItemPrice").Attr("value")
				if !exists {
					parts := strings.Split(strings.TrimSpace(s.Find(".subitem-price > span").Text()), " ")
					if len(parts) > 1 {
						p = parts[1]
					}
				}

				price, err := strconv.Atoi(p)
				if err != nil {
					t.chErr <- err
					log.WithError(err).Error("[crawler] Error converting string price to integer")
					return
				}

				meal.Servings = append(meal.Servings, model.Serving{Type: kind, Price: price})

				log.WithFields(log.Fields{
					"title":   title,
					"desc":    desc,
					"type":    cat,
					"url":     url,
					"serving": kind,
					"price":   price,
				}).Debug("[crawler] Parsed restaurant meal details")
			})

			r.Menu = append(r.Menu, meal)
		})
	})

	return r
}
