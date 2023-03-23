package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

const (
	chatGPTAPIURL = "https://api.openai.com/v1/chat/completions"
)

func main() {
	token := os.Getenv("DISCORD_BOT_TOKEN")

	discord, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("Error creating Discord session:", err)
		return
	}
	//discord.Identify.Intents = discordgo.IntentsGuildMessages
	discord.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages)
	discord.AddHandler(messageCreate)

	err = discord.Open()
	if err != nil {
		fmt.Println("Error opening connection:", err)
		return
	}

	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	discord.Close()
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	fmt.Printf("\nReceived message: %s from %s in channel %s\n", m.Content, m.Author.ID, m.ChannelID)

	if m.Author.ID == s.State.User.ID {
		fmt.Println("Ignoring message from self")
		return
	}

	if strings.HasPrefix(m.Content, "/chatgpt") {
		prompt := strings.TrimPrefix(m.Content, "/chatgpt")

		response, err := getChatGPTResponse(prompt)
		if err != nil {
			_, err := s.ChannelMessageSend(m.ChannelID, "Error: "+err.Error())
			if err != nil {
				fmt.Println("Error sending error message:", err)
			}
			return
		}

		// Verify that the answer is not empty before sending
		response = strings.TrimSpace(response)
		if response == "" {
			_, err = s.ChannelMessageSend(m.ChannelID, "Error: The response is empty.")
			if err != nil {
				fmt.Println("Error sending empty response message:", err)
			}
			return
		}

		_, err = s.ChannelMessageSend(m.ChannelID, response)
		if err != nil {
			fmt.Println("Error sending response message:", err)
		}
	} else {
		fmt.Println("Message does not have /chatgpt prefix")
	}
}

func getChatGPTResponse(prompt string) (string, error) {
	chatGPTAPIToken := os.Getenv("CHAT_GPT_API_TOKEN")

	fmt.Printf("getChatGPTResponse called with prompt: %s\n", prompt)
	client := &http.Client{}

	type RequestPayload struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		MaxTokens int    `json:"max_tokens"`
		Model     string `json:"model"`
	}

	data := RequestPayload{
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			{
				Role:    "system",
				Content: "You are a helpful assistant.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens: 50,
		Model:     "gpt-3.5-turbo",
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	request, err := http.NewRequest("POST", chatGPTAPIURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return "", err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+chatGPTAPIToken)

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	fmt.Printf("Status code: %d\n", response.StatusCode)
	body, err := ioutil.ReadAll(response.Body)
	fmt.Printf("Response body: %s\n", string(body))
	if err != nil {
		return "", err
	}

	type ResponsePayload struct {
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	var payload ResponsePayload
	err = json.Unmarshal(body, &payload)
	if err != nil {
		return "", err
	}

	if len(payload.Choices) > 0 {
		// Remove any leading whitespace from the response text.
		return strings.TrimSpace(payload.Choices[0].Message.Content), nil
	}

	fmt.Printf("Status code: %d\nResponse body: %s\n", response.StatusCode, string(body))
	return "", fmt.Errorf("no response from ChatGPT")

}
