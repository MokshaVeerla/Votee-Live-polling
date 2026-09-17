# Live Polling Tool

A real-time polling application where users can create a poll, share it with others, and see voting results update instantly without refreshing the page.

## Features

- Create a poll with a question and two options
- Share the poll using a unique link
- Vote for an option
- View vote counts and percentages
- See live result updates without refreshing
- Store poll data using Redis
- Real-time communication using WebSockets
- Basic validation for invalid polls and votes

## Tech Stack

- React
- Go
- Redis
- WebSockets
- CSS

## Project Structure

```text
live-polling-tool/
│
├── backend/
│   ├── go.mod
│   ├── go.sum
│   └── main.go
│
├── frontend/
│   ├── src/
│   │   ├── App.jsx
│   │   ├── App.css
│   │   └── index.css
│   ├── package.json
│   └── vite.config.js
│
└── README.md