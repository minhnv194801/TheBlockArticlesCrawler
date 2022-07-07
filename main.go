package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/gocolly/colly"
)

var (
	EndOfArticles bool   = false
	PublicKey     string = "0x02fb51a24a56b1cd7776850b74ce9d7c8caca7c650b2ea6b54a9593b6ecf745e1a"
	Database      *redis.Client
)

type Article struct {
	Title    string `json:"title"`
	ImageURL string `json:"image"`
	Content  string `json:"content"`
	Date     string `json:"date"`
}

// Adding article into the database using url as the key
//
// This function will process the article data into json string and store the url and that
// json string as a pair of key-value and then record the url into the key "theblock"
func AddTheBlockArticleToDB(url string, newArticle Article) error {
	time, err := time.Parse("January 2, 2006, 3:4PM", newArticle.Date)
	if err != nil {
		return err
	}
	score := -time.Unix()
	jsonBytes, err := json.Marshal(newArticle)
	if err != nil {
		return err
	}
	jsonString := string(jsonBytes)
	Database.Set(url, jsonString, 0)
	member := redis.Z{
		Score:  float64(score),
		Member: url,
	}
	err = Database.ZAdd("theblock", member).Err()
	if err != nil {
		return err
	}
	return nil
}

// Create a new crawler for www.theblock.co
func NewTheBlockCrawler() *colly.Collector {
	c := colly.NewCollector(
		colly.Async(true),
		colly.AllowedDomains("www.theblock.co"),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "www.theblock.co",
		Parallelism: 100,
		RandomDelay: 5 * time.Second,
	})

	c.WithTransport(&http.Transport{
		DisableKeepAlives: true,
	})

	// Handle "theblock.co/latest" pages
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		if strings.Contains(e.Request.URL.String(), "https://www.theblock.co/latest") {
			link := e.Attr("href")
			if strings.Contains(link, "/post") {
				c.Visit(e.Request.AbsoluteURL(link))
			}
		}
	})

	// Handle "theblock.co/post" pages
	c.OnHTML(".articleBody", func(e *colly.HTMLElement) {
		if strings.Contains(e.Request.URL.String(), "https://www.theblock.co/post") {
			var newArticle Article

			// Handle article title
			title := e.ChildText("h1")
			newArticle.Title = title

			// Handle article image
			e.ForEachWithBreak("img[src]", func(index int, e *colly.HTMLElement) bool {
				image := e.Attr("src")
				newArticle.ImageURL = image
				return false
			})

			// Handle article
			content := ""
			e.ForEach("p", func(index int, e *colly.HTMLElement) {
				content += e.Text
			})
			newArticle.Content = content

			// Handle date
			e.ForEachWithBreak(".articleMeta", func(index int, e *colly.HTMLElement) bool {
				date := strings.Split(e.Text, "\n")[1]
				date = date[9 : len(date)-4]
				date = strings.Trim(date, " ")
				newArticle.Date = date
				return false
			})

			// Adding article to database
			err := AddTheBlockArticleToDB(e.Request.URL.String(), newArticle)
			if err != nil {
				log.Fatal(err)
			}
		}
	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	return c
}

// Creating post with an account using a number of newest crawled articles
func CreatePost(pubKey string, number int) {
	// Do something with the public key and get username
	user := "The Block"
	urls, err := Database.ZRange("theblock", 0, int64(number)-1).Result()
	if err != nil {
		log.Fatal(err)
	}
	for _, url := range urls {
		articleJsonString, err := Database.Get(url).Result()
		if err != nil {
			log.Fatal(err)
		}
		var article Article
		json.Unmarshal([]byte(articleJsonString), &article)
		fmt.Println("User:", user)
		fmt.Println("Title:", article.Title)
		fmt.Println("Content:", article.Content)
		fmt.Println("Image:", article.ImageURL)
	}
}

func main() {
	Database = redis.NewClient(&redis.Options{
		Addr:     "localhost:49153",
		Password: "redispw",
		DB:       0,
	})

	c := NewTheBlockCrawler()

	page := 0
	for page < 100 {
		c.Visit("https://www.theblock.co/latest?start=" + strconv.Itoa(page*10))
		page++
	}
	c.Wait()

	CreatePost(PublicKey, 10)
}
