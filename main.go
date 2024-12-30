package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type MessagesRequest struct {
	Model     string    `json:"model"`
	System    string    `json:"system"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens"`
}

type ContentBlock struct {
	Text string `json:"text"`
}

type MessagesResponse struct {
	Content []ContentBlock `json:"content"`
}

func getDiff(all bool) (string, error) {
	args := []string{"diff"}
	if !all {
		args = append(args, "--cached")
	}
	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error getting diff: %v", err)
	}
	return string(output), nil
}

func askClaude(prompt string, apiKey string) (string, error) {
	ctx := context.Background()
	claudeModel := "claude-3-5-sonnet-20240620"
	systemPrompt := `You are a Git commit message assistant. If you need more context, ask exactly one clear question. 
If you have enough context, provide ONLY the commit message without any explanations or questions. 
The commit message should follow best practices and be wrapped in triple backticks.`

	reqBody := MessagesRequest{
		Model:  claudeModel,
		System: systemPrompt,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
		MaxTokens: 4096,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var result MessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("error decoding response: %v", err)
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty response from API")
	}
	return result.Content[0].Text, nil
}

func getUserInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func extractCommitMessage(response string) string {
	start := strings.Index(response, "```")
	if start == -1 {
		return ""
	}
	end := strings.Index(response[start+3:], "```")
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(response[start+3 : start+3+end])
}

func editInVim(message string) (string, error) {
	tempFile, err := os.CreateTemp("", "commit-msg-*.txt")
	if err != nil {
		return "", fmt.Errorf("error creating temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.WriteString(message); err != nil {
		return "", fmt.Errorf("error writing to temp file: %v", err)
	}
	tempFile.Close()

	cmd := exec.Command("vim", tempFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error running vim: %v", err)
	}

	editedContent, err := os.ReadFile(tempFile.Name())
	if err != nil {
		return "", fmt.Errorf("error reading edited file: %v", err)
	}

	editedStr := string(editedContent)
	if editedStr == message {
		confirm := getUserInput("No changes made. Use original message? (y/n): ")
		if confirm != "y" {
			return "", fmt.Errorf("edit cancelled")
		}
	}

	return editedStr, nil
}

const helpText = `Usage: gitcommit [options]

Options:
  -a        Commit all changes (including unstaged)
  -help     Display this help message

When run, the program will:
1. Ask for an initial commit message
2. Get feedback from Claude
3. Present options to:
   - Accept the suggested message (y)
   - Reject it (n)
   - Edit it in vim (e)

Environment:
  CLAUDE_API_KEY    Required API key for Claude`

func main() {
	help := flag.Bool("help", false, "display help message")
	allChanges := flag.Bool("a", false, "commit all changes")
	flag.Usage = func() {
		fmt.Println(helpText)
	}
	flag.Parse()

	if *help || len(flag.Args()) > 0 {
		flag.Usage()
		return
	}

	apiKey := os.Getenv("CLAUDE_API_KEY")
	if apiKey == "" {
		fmt.Println("Please set CLAUDE_API_KEY environment variable")
		return
	}

	originalMessage := getUserInput("Enter commit message: ")

	diff, err := getDiff(*allChanges)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	if diff == "" {
		fmt.Println("No staged changes found. Stage your changes first.")
		return
	}

	prompt := fmt.Sprintf(`Help me write a better git commit message. Here's my original message:
"%s"

Here are the changes:
%s`, originalMessage, diff)

	for {
		response, err := askClaude(prompt, apiKey)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		commitMsg := extractCommitMessage(response)
		if commitMsg != "" {
			fmt.Printf("\nSuggested commit message:\n%s\n", commitMsg)
			answer := getUserInput("\nUse this message? (y/n/e to edit): ")

			var finalMessage string
			switch answer {
			case "y":
				finalMessage = commitMsg
			case "e":
				edited, err := editInVim(commitMsg)
				if err != nil {
					fmt.Printf("Error editing message: %v\n", err)
					return
				}
				finalMessage = strings.TrimSpace(edited)
			case "n":
				continue
			default:
				fmt.Println("Invalid option. Please enter y, n, or e.")
				continue
			}

			args := []string{"commit"}
			if *allChanges {
				args = append(args, "-a")
			}
			args = append(args, "-m", finalMessage)
			cmd := exec.Command("git", args...)
			if err := cmd.Run(); err != nil {
				fmt.Printf("Error making commit: %v\n", err)
				return
			}
			fmt.Println("Commit successful!")
			return
		}

		// If no commit message was found, treat the response as a question
		moreInfo := getUserInput(fmt.Sprintf("\nClaude asks: %s\nYour response: ", response))
		prompt += fmt.Sprintf("\n\nAdditional context: %s", moreInfo)
	}
}
