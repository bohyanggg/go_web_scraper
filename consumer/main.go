package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
	"github.com/segmentio/kafka-go"
)

// ImageData represents the image data structure
type ImageData struct {
	URL string `json:"url"`
	Alt string `json:"alt"`
}

func main() {
	// Load environment variables
	kafkaBroker := "kafka:9092"
	topic := "image-data"
	postgresDSN := "postgresql://consumer_user:consumer_password@postgres_consumer:5432/consumer_db?sslmode=disable"

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", postgresDSN)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	log.Println("Successfully connected to PostgreSQL")
	defer db.Close()

	// Ensure the table exists
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS images (
		id SERIAL PRIMARY KEY,
		url TEXT NOT NULL,
		alt TEXT
	);
	`
	_, err = db.Exec(createTableQuery)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	// Configure Kafka reader
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{kafkaBroker},
		Topic:   topic,
		GroupID: "image-consumer-group",
	})
	defer reader.Close()

	log.Println("Starting Kafka consumer...")

	for {
		// Read message from Kafka
		msg, err := reader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("Error reading message: %v", err)
			continue
		}

		log.Printf("Received message: %s", string(msg.Value))

		// Deserialize the message
		var imageData ImageData
		err = json.Unmarshal(msg.Value, &imageData)
		if err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			continue
		}

		// Download the image
		resp, err := http.Get(imageData.URL)
		if err != nil {
			log.Printf("Failed to download image: %v", err)
			continue
		}
		defer resp.Body.Close()

		image, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Failed to read image data: %v", err)
			continue
		}

		//
		// Insert into PostgreSQL
		log.Printf("Start inserting image data into PostgreSQL")
		insertQuery := `INSERT INTO images (url, alt, image) VALUES ($1, $2, $3)`
		_, err = db.Exec(insertQuery, imageData.URL, imageData.Alt, image)
		if err != nil {
			log.Printf("Failed to insert data into PostgreSQL: %v", err)
			continue
		} else {
			log.Printf("Inserted image data into PostgreSQL: %+v", imageData)
		}

		// Check to make sure binary data is correct
		var picture []byte
		var url, alt string
		err = db.QueryRow(`SELECT image, url, alt FROM images WHERE id = $1`, 305).Scan(&picture, &url, &alt)
		if err != nil {
			log.Fatalf("Failed to query image data: %v", err)
		}

		// Save the binary data to a file
		err = os.WriteFile("/tmp/output_image.jpg", picture, 0644)
		if err != nil {
			log.Fatalf("Failed to save image: %v", err)
		}

		log.Printf("Image saved as output_image.jpg from URL: %s (Alt: %s)", url, alt)
	}
}
