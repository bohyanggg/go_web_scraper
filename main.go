package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gocolly/colly"
	"github.com/segmentio/kafka-go"
)

type ImageData struct {
	URL string `json:"url"`
	Alt string `json:"alt"`
}

func scrapeImages(targetURL string) []ImageData {
	images := []ImageData{}

	// Create a new collector
	c := colly.NewCollector()

	// Set custom User-Agent and Referer
	c.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"
	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Referer", "https://www.vesselfinder.com/")
		r.Headers.Set("Cookie", "__eoi=ID=e32746e65147a091:T=1736328196:RT=1736389423:S=AA-AfjZna_0nE4JkyozdsA_jgf0q; __gads=ID=d6a8d64bec2f1b35:T=1736328196:RT=1736389423:S=ALNI_Ma_zPpU9VvW9sCWK5ar9niNprvnbg; __gpi=UID=00000fd99498027d:T=1736328196:RT=1736389423:S=ALNI_MbdrBtTnjsRM2flYr8_wxdEKfQ0RA; cto_bundle=OU0oXV91b0RLOXljNkNPcjUzcDN0aGhQNjBkZUs0U0JpNFliJTJGNFlWU3ZJMVJlU1pCdGRCV3lTNlRDUXBybW11elgycTN2WDFVY0tzT2ZYcCUyQndFWXpwTFU1dFZ4dnNmN0QzUmtuRWdCYUlubzYwS1F1cHc3S2F6Rjl2OWxoNzg2d3lKcFE; cto_bidid=HBZuWV9Ya3h2UVBEZXA5ViUyRkpBaGc5MUpmcVFTUVg5ajlPNDV4Qk84SmFFT0ZweTJzSW1IZVpDdEJHbGlOeGQ0ZlhvV2NjdmU0WnpaRHpydnZaZVhqb3VneXB3JTNEJTNE; _sharedID=855cac8b-a380-45d0-9ab2-a7a1f0bada25; _sharedID_cst=kSylLAssaw%3D%3D; _ga=GA1.1.89097713.1736316807; _ga_0MB1EVE8B7=GS1.1.1736388637.5.1.1736388937.0.0.0; usprivacy=1N--; _cc_id=ac51159f94776b80f8d44ea234f66a8d; panoramaId=a91448fcc442e15645f154c8eb1f185ca02c4352aca53ae798fd68adb052fa58; panoramaIdType=panoDevice; panoramaId_expiry=1736921607970; ROUTEID=.2")
	})

	// Limit requests to reduce the risk of being blocked
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*vesselfinder.com*",
		Delay:       3 * time.Second,
		RandomDelay: 1 * time.Second,
	})

	// Target the specific <img> element with id="main-photo"
	c.OnHTML("img#main-photo", func(e *colly.HTMLElement) {
		imageURL := e.Attr("src")
		altText := e.Attr("alt")

		if imageURL != "" {
			images = append(images, ImageData{
				URL: imageURL,
				Alt: altText,
			})
		}
	})

	c.OnResponse(func(r *colly.Response) {
		log.Printf("Response received: %d\n", r.StatusCode)
	})

	// Handle errors
	c.OnError(func(r *colly.Response, err error) {
		log.Printf("Request failed: %v\n", err)
	})

	// Visit the target URL
	err := c.Visit(targetURL)
	if err != nil {
		log.Fatalf("Failed to visit target URL: %v", err)
	}

	return images
}

func sendToKafka(topic string, broker string, data []ImageData) {
	writer := kafka.Writer{
		Addr:     kafka.TCP(broker),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}

	defer writer.Close()

	for _, img := range data {
		message, err := json.Marshal(img)
		if err != nil {
			log.Printf("Failed to serialize data: %v\n", err)
			continue
		}

		log.Printf("Attempting to send message to Kafka: %s", string(message))

		err = writer.WriteMessages(
			context.Background(),
			kafka.Message{
				Value: message,
			},
		)

		if err != nil {
			log.Printf("Failed to send message to Kafka: %v\n", err)
		} else {
			log.Printf("Message sent successfully to Kafka topic: %s", topic)
		}
	}
}

func main() {
	log.Println("Starting the scraper...")
	targetURL := os.Getenv("TARGET_URL")
	if targetURL == "" {
		log.Fatal("TARGET_URL environment variable is required")
	}

	broker := os.Getenv("KAFKA_BROKER")
	if broker == "" {
		log.Fatal("KAFKA_BROKER environment variable is required")
	}

	topic := os.Getenv("TOPIC")
	if topic == "" {
		log.Fatal("TOPIC environment variable is required")
	}

	images := scrapeImages(targetURL)

	for _, img := range images {
		fmt.Printf("Image URL: %s, Alt Text: %s\n", img.URL, img.Alt)
	}

	sendToKafka(topic, broker, images)
	log.Println("Delaying container exit for debugging...")
	time.Sleep(10 * time.Minute)
	// fmt.Printf("%s", images[0])
}
