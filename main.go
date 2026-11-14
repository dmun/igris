package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
)

const defaultModel = "mistral-small-latest"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type ChatCompletionResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

type ModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func newClient() (*Client, error) {
	apiKey := os.Getenv("MISTRAL_API_KEY")
	if apiKey == "" {
		return nil, errors.New("MISTRAL_API_KEY is not set")
	}
	return &Client{
		BaseURL: "https://api.mistral.ai/v1",
		APIKey:  apiKey,
	}, nil
}

func encodeJSON(in any) (io.Reader, error) {
	if in == nil {
		return nil, nil
	}
	payload, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(payload), nil
}

func (c *Client) sendJSON(ctx context.Context, method, path string, in, out any) error {
	body, err := encodeJSON(in)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fragment, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, strings.TrimSpace(string(fragment)))
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) ChatCompletion(ctx context.Context, in ChatCompletionRequest) (ChatCompletionResponse, error) {
	var out ChatCompletionResponse
	err := c.sendJSON(ctx, http.MethodPost, "/chat/completions", in, &out)
	return out, err
}

func (c *Client) ListModels(ctx context.Context) (ModelsResponse, error) {
	var out ModelsResponse
	err := c.sendJSON(ctx, http.MethodGet, "/models", nil, &out)
	return out, err
}

func main() {
	log.SetFlags(0)
	if err := newRootCmd().ParseAndRun(context.Background(), os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Fatal(err)
	}
}

func newRootCmd() *ffcli.Command {
	return &ffcli.Command{
		Name:       "igris",
		ShortUsage: "igris <command> [flags]",
		ShortHelp:  "Mistral client.",
		Subcommands: []*ffcli.Command{
			newModelCmd(),
			newChatCmd(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func newModelCmd() *ffcli.Command {
	return &ffcli.Command{
		Name:       "model",
		ShortUsage: "igris model <command>",
		ShortHelp:  "Inspect models.",
		Subcommands: []*ffcli.Command{
			newModelListCmd(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func newModelListCmd() *ffcli.Command {
	fs := flag.NewFlagSet("model list", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print JSON instead of a plain list")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "igris model list [-json]",
		ShortHelp:  "List available models.",
		FlagSet:    fs,
		Exec: func(ctx context.Context, args []string) error {
			return listModels(ctx, *asJSON)
		},
	}
}

func newChatCmd() *ffcli.Command {
	fs := flag.NewFlagSet("chat", flag.ContinueOnError)
	model := fs.String("model", defaultModel, "model to use")

	return &ffcli.Command{
		Name:       "chat",
		ShortUsage: "igris chat [-model name] <prompt>",
		ShortHelp:  "Send a prompt to a model.",
		FlagSet:    fs,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return errors.New("chat requires a prompt")
			}
			return chat(ctx, *model, strings.Join(args, " "))
		},
	}
}

func listModels(ctx context.Context, asJSON bool) error {
	client, err := newClient()
	if err != nil {
		return err
	}

	models, err := client.ListModels(ctx)
	if err != nil {
		return err
	}

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(models)
	}

	for _, model := range models.Data {
		fmt.Println(model.ID)
	}
	return nil
}

func chat(ctx context.Context, model, prompt string) error {
	client, err := newClient()
	if err != nil {
		return err
	}

	result, err := client.ChatCompletion(ctx, ChatCompletionRequest{
		Model: model,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return err
	}
	if len(result.Choices) == 0 {
		return errors.New("empty choices")
	}

	fmt.Println(result.Choices[0].Message.Content)
	return nil
}
