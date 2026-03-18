package main

import (
	"log"
	"os"

	"arrai/internal/provider/vk"

	"github.com/joho/godotenv"
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Panicf("Error loading .env file: %v", err.Error())
	}
}

func main() {
	accessToken, exists := os.LookupEnv("VK_ACCESS_TOKEN")

	if !exists {
		log.Println("VK_ACCESS_TOKEN is not set")
		return
	}

	client := vk.NewClient(accessToken)
	log.Printf(client.GetWall("wall-1"))
}
