package main

import (
	"encoding/json"
	"fmt"
	"log"
	"io/ioutil"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/nlopes/slack"
	"gopkg.in/yaml.v2"
)

type Account struct {
	Name 	 string `yaml:"name"`
	Severity string `yaml:"severity"`
	Webhook  string `yaml:"webhook"`
}

type Config struct {
	Color    map[string]string `yaml:"colors"`
	URL      string `yaml:"url"`
	Webhook  string `yaml:"webhook"`
	Account  map[string]Account `yaml:"accounts"`
}

type GuardDutyFinding struct {
	ID          string  `json:id`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Severity    float32 `json:"severity"`
	Type        string  `json:"type"`
	AccountId   string  `json:"accountId"`
}

func isValidSeverity(s string) (bool){
  return s == "low" || s == "medium" || s == "high"
}

func getConfig() *Config {
	var config Config

	file, err := ioutil.ReadFile("main.yml")
	if err != nil {
		log.Fatal("Cannot open main.yml: %v", err)
	}

	err = yaml.Unmarshal(file, &config)
	if err != nil {
		log.Fatal("Cannot unmarshal main.yml: %v", err)
	}

	return &config
}

func handler(event events.CloudWatchEvent) error {
	var finding GuardDutyFinding
	err := json.Unmarshal(event.Detail, &finding)
	if err != nil {
		log.Fatal("Cannot unmarshal finding: %v", err)
	}

	// Default to low severity level
	config := getConfig()
	severity := config.Account[finding.AccountId].Severity
	if ! isValidSeverity(severity) {
		severity = "low"
	}

	// https://docs.aws.amazon.com/guardduty/latest/ug/guardduty_findings.html#guardduty_findings-severity
	// Set the severity level according to the severity value
	// Exit and avoid sending alerts below the configured severity
	severityLevel := "High"
	if finding.Severity < 4 {
		severityLevel = "Low"

		if severity != "low" {
			return nil
		}
	} else if finding.Severity < 7 {
		severityLevel = "Medium"

		if severity == "high" {
			return nil
		}
	}

	// Include account name
	account := fmt.Sprintf("%s (%s)", finding.AccountId, config.Account[finding.AccountId].Name)

	// URL to GuardDuty finding
	findingURL := fmt.Sprintf(config.URL, event.Region, finding.ID)

	// Provide default webhook for unconfigured accounts
	webhook := config.Account[finding.AccountId].Webhook
	if len(webhook) == 0 {
		// This could be empty as well, but we'll error on the post
		webhook = config.Webhook
	}

	attachment := slack.Attachment{
		Color: config.Color[severity],
		Title: finding.Title,
		Text:  finding.Description,
		Fields: []slack.AttachmentField{
			{
				Title: "Account",
				Value: account,
			},
			{
				Title: "Severity",
				Value: severityLevel,
			},
			{
				Title: "URL:",
				Value: findingURL,
			},
		},
	}

	message := slack.WebhookMessage{
		Attachments: []slack.Attachment{attachment},
	}

	return slack.PostWebhook(webhook, &message)
}

func main() {
	lambda.Start(handler)
}
