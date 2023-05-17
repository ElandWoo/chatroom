package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type RequestBody struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type ResponseBody struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Choices []struct {
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason"`
		Message      Message
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func createRequestBody() RequestBody {
	return RequestBody{"gpt-3.5-turbo", []Message{{"user", "Hello!"}}}
}

func createRequest(payload []byte) *http.Request {
	req, err := http.NewRequest("POST", OpenAIURL, bytes.NewBuffer(payload))
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+OpenAIAPIKey)
	return req
}

func createClient() *http.Client {
	proxyURL, err := url.Parse(ProxyURL)
	if err != nil {
		log.Fatal(err)
	}
	return &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
}

func printResponseBody(respBody ResponseBody) {
	fmt.Printf("ID: %s\nObject: %s\nCreated: %d\n", respBody.ID, respBody.Object, respBody.Created)
	for i, choice := range respBody.Choices {
		fmt.Printf("Choice %d:\n  Index: %d\n  Finish Reason: %s\n  Message Role: %s\n  Message Content: %s\n", i, choice.Index, choice.FinishReason, choice.Message.Role, choice.Message.Content)
	}
	fmt.Printf("Usage:\n  Prompt Tokens: %d\n  Completion Tokens: %d\n  Total Tokens: %d\n", respBody.Usage.PromptTokens, respBody.Usage.CompletionTokens, respBody.Usage.TotalTokens)
}

func main() {
	reqBody := createRequestBody()
	payload, err := json.Marshal(reqBody)
	if err != nil {
		log.Fatal(err)
	}

	client := createClient()
	resp, err := client.Do(createRequest(payload))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var respBody ResponseBody
	err = json.Unmarshal(body, &respBody)
	if err != nil {
		log.Fatal(err)
	}

	printResponseBody(respBody)
}
