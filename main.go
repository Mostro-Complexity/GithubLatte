package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
	url "github.com/shimohq/go-url-join"
)

const (
	// GithubURL is URL to visit.
	GithubURL = "https://github.com"
	// GithubDomain is allowed domain.
	GithubDomain = "github.com"
)

func newRepoCollector() (*colly.Collector, error) {
	repoCollector := colly.NewCollector( // initialize colly
		colly.AllowedDomains(GithubDomain), // domain
		colly.AllowURLRevisit(),
		colly.UserAgent("Mozilla/5.0 (Windows NT x.y; rv:10.0) Gecko/20100101 Firefox/10.0"),
	)
	repoCollector.WithTransport(&http.Transport{
		DisableKeepAlives: true,
	})
	err := repoCollector.Limit(&colly.LimitRule{
		// filter domains affected by this rule
		DomainGlob: GithubDomain,
		// set a delay between requests to these domains
		Delay: 2 * time.Second,
		// add an additional random delay
		RandomDelay: 1 * time.Second,
		// set parallelism
		Parallelism: 5,
	})
	if err != nil {
		return nil, err
	}
	extensions.RandomUserAgent(repoCollector)

	repoCollector.OnHTML("h3[class=\"f3 color-fg-muted text-normal lh-condensed\"]",
		func(h *colly.HTMLElement) {
			link := h.ChildAttr("a[class]", "href")
			organization := strings.TrimSpace(h.ChildTexts("a[data-hydro-click]")[0])
			repo := strings.TrimSpace(h.ChildTexts("a[data-hydro-click]")[1])
			log.Printf("-> Repo found: %q -> %s\n", url.Join(organization, repo), url.Join(GithubURL, link))
		},
	)

	return repoCollector, nil
}

func newTopicCollector(repoCollector *colly.Collector) *colly.Collector {
	topicCollector := repoCollector.Clone()
	topicCollector.OnHTML("a[href][class=\"no-underline flex-1 d-flex flex-column\"]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		topic := e.ChildText("p[class=\"f3 lh-condensed mb-0 mt-1 Link--primary\"]")
		log.Printf("Topic found: %q -> %s\n", topic, url.Join(GithubURL, link))
		e.Response.Ctx.Put("Topic", topic)
		err := repoCollector.Request("GET", url.Join(GithubURL, link), nil, e.Response.Ctx, nil)
		if err != nil {
			log.Println(err)
		}
	})
	// before request
	topicCollector.OnRequest(func(r *colly.Request) {
		log.Println("Visiting", r.URL.String())
	})
	// after response
	topicCollector.OnResponse(func(r *colly.Response) {
		log.Println("Get response", r.Request.URL.String())
	})
	// after a scraping task
	topicCollector.OnScraped(func(r *colly.Response) {
		log.Println("Finished", r.Request.URL.String())
	})

	return topicCollector
}

func main() {
	repoCollector, err := newRepoCollector()
	if err != nil {
		log.Println(err)
	}
	topicCollector := newTopicCollector(repoCollector)
	if err != nil {
		log.Println(err)
	}
	// start visit
	for t := 0; t < 10; t++ {
		err := topicCollector.Request("GET", fmt.Sprintf(url.Join(GithubURL, "topics?page=%v"), t+1), nil, nil, nil)
		if err != nil {
			continue
		}
	}
	repoCollector.Wait()
	topicCollector.Wait()
}
