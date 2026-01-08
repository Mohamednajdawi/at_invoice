package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found")
	}

	apiKey := os.Getenv("SENDGRID_API_KEY")
	fromEmail := os.Getenv("FROM_EMAIL")

	if apiKey == "" {
		log.Fatal("SENDGRID_API_KEY not set in .env")
	}
	if fromEmail == "" {
		log.Fatal("FROM_EMAIL not set in .env")
	}

	// Prompt for test email
	var testEmail string
	fmt.Print("Enter your email to test: ")
	fmt.Scanln(&testEmail)

	// Create test email
	from := mail.NewEmail("AT-Invoice Test", fromEmail)
	to := mail.NewEmail("", testEmail)
	subject := "SendGrid Test - AT-Invoice"

	htmlContent := `
		<!DOCTYPE html>
		<html>
		<head>
			<meta charset="UTF-8">
			<style>
				body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
				.container { max-width: 600px; margin: 0 auto; padding: 20px; }
				.header { background-color: #dc2626; color: white; padding: 20px; text-align: center; }
				.content { padding: 20px; background-color: #f9fafb; }
			</style>
		</head>
		<body>
			<div class="container">
				<div class="header">
					<h1>✅ SendGrid Test Successful!</h1>
				</div>
				<div class="content">
					<p>Great news! Your SendGrid integration is working correctly.</p>
					<p>You can now send API key emails to your customers.</p>
				</div>
			</div>
		</body>
		</html>
	`

	plainText := "SendGrid test successful! Your integration is working."

	message := mail.NewSingleEmail(from, subject, to, plainText, htmlContent)
	client := sendgrid.NewSendClient(apiKey)

	fmt.Println("\nSending test email...")
	response, err := client.Send(message)
	if err != nil {
		log.Fatalf("Failed to send email: %v", err)
	}

	fmt.Printf("✅ Email sent successfully!\n")
	fmt.Printf("Status Code: %d\n", response.StatusCode)
	fmt.Printf("Check your inbox at: %s\n", testEmail)

	if response.StatusCode >= 400 {
		fmt.Printf("Response Body: %s\n", response.Body)
		fmt.Printf("Response Headers: %v\n", response.Headers)
	}
}
