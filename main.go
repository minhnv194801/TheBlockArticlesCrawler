package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-redis/redis"
	"github.com/gocolly/colly"
)

type ArticleList struct {
	Articles []Article `json:"articles"`
}

type Article struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Date    string `json:"date"`
}

var (
	Database      *redis.Client
	EndOfArticles bool = false
)

func addArticleToDB(newArticle Article) error {
	articles, err := Database.Get("articles").Result()
	if err != nil {
		fmt.Println(err)
		return err
	}
	var articleList ArticleList
	err = json.Unmarshal([]byte(articles), &articleList)
	if err != nil {
		fmt.Println(err)
		return err
	}
	articleList.Articles = append(articleList.Articles, newArticle)
	jsonString, err := json.Marshal(articleList)
	if err != nil {
		fmt.Println(err)
		return err
	}
	Database.Set("articles", jsonString, 0)
	return err
}

func main() {
	Database = redis.NewClient(&redis.Options{
		Addr:     "redis-17969.c57.us-east-1-4.ec2.cloud.redislabs.com:17969",
		Password: "dYrmsiPgAt5aDuB3Eo9KTld0HS1lSuKX",
		DB:       0,
	})
	var emptyArticleList ArticleList
	jsonString, err := json.Marshal(emptyArticleList)
	if err != nil {
		fmt.Println(err)
	}
	Database.Set("articles", jsonString, 0)

	c := colly.NewCollector(
		colly.AllowedDomains("www.theblock.co"),
		colly.Async(true),
	)

	c.OnHTML("div.collectionLatest", func(e *colly.HTMLElement) {
		// Extract the link from the anchor HTML element
		if strings.Contains(e.Request.URL.String(), "https://www.theblock.co/latest") {
			e.ForEach("a[href]", func(index int, e *colly.HTMLElement) {
				link := e.Attr("href")
				if strings.Contains(link, "/post") {
					c.Visit(e.Request.AbsoluteURL(link))
				}
			})
		}
	})

	c.OnHTML("dev.next inactive", func(e *colly.HTMLElement) {
		EndOfArticles = true
	})

	c.OnHTML("article.articleBody", func(e *colly.HTMLElement) {
		if strings.Contains(e.Request.URL.String(), "https://www.theblock.co/post") {
			var article Article
			// Handle article title
			title := e.ChildText("h1[data-v-6ce9c252]")
			article.Title = title
			// Handle article
			content := ""
			e.ForEach("p", func(index int, e *colly.HTMLElement) {
				content += e.Text
			})
			article.Content = content
			// Handle date
			e.ForEach("div.articleMeta", func(index int, e *colly.HTMLElement) {
				date := strings.Split(e.Text, "\n")[1][9:]
				date = strings.Trim(date, " ")
				article.Date = date
			})
			if len(article.Content) != 0 && len(article.Title) != 0 && len(article.Date) != 0 {
				addArticleToDB(article)
			}
		}
	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	page := 0
	for !EndOfArticles {
		c.Visit("https://www.theblock.co/latest?start=" + strconv.Itoa(page*10))
		page++
		// Need further adjustment
		if page == 100 {
			EndOfArticles = true
		}
	}

	c.Wait()
}
