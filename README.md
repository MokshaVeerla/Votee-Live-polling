# Live Polling Tool
A real-time polling application where users can create a poll, share it with others, vote, and see the results update live without refreshing the page.

## Live Demo
https://votee-live-polling.vercel.app/

## Features
- User login for creating polls
- Create a poll with a question and options
- Share polls using a unique link
- Vote on a poll
- View vote counts and percentages
- Live result updates without refreshing
- Store poll data in MongoDB
- Use Redis for live vote counters and updates
- WebSocket-based real-time communication
- Basic input validation

## How to Run
Make sure MongoDB and Redis are running before starting the backend.
React → Go/Gin → MongoDB + Redis → WebSocket → result
   ## Backend
      cd backend
      go run main.go
    
   ## Frontend
      cd frontend
      npm install
      npm run dev

## Tech Stack
- React
- Go / Gin
- MongoDB
- Redis
- WebSockets
- CSS

## Project Structure
/frontend   > React app
/backend    > Go service
README.md   > Project information
