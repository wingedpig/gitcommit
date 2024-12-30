# gitcommit

A Git commit message assistant powered by Claude AI.

## Overview

gitcommit is a command-line tool that helps you write better Git commit messages. It uses Claude AI to analyze your changes and suggest clear, descriptive commit messages that follow best practices.

## Features

- Analyzes staged changes to understand context
- Suggests well-formatted commit messages
- Interactive workflow with options to:
  - Accept suggested message
  - Edit message in vim
  - Request a new suggestion
- Supports committing all changes with -a flag
- Validates edited messages

## Installation

```bash
go install github.com/wingedpig/gitcommit@latest
```

## Setup

Get an API key from Anthropic (https://console.anthropic.com/)
Set your API key:

```bash
export CLAUDE_API_KEY=your_api_key_here
```

## Usage

# Show help
gitcommit -help

# Commit staged changes
gitcommit

# Commit all changes (including unstaged)
gitcommit -a
When run, gitcommit will:

Ask for your initial commit message
Analyze the changes and suggest an improved message
Let you accept, edit, or reject the suggestion

## License

MIT License - see LICENSE file for details.

## Contributing

Contributions are welcome! Feel free to open issues or submit pull requests.
