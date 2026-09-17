import { useEffect, useState } from "react";
import "./App.css";

function App() {
  const path = window.location.pathname;
  const pollId = path.split("/poll/")[1];

  const [question, setQuestion] = useState("");
  const [option1, setOption1] = useState("");
  const [option2, setOption2] = useState("");

  const [createdPoll, setCreatedPoll] = useState(null);
  const [poll, setPoll] = useState(null);
  const [pollNotFound, setPollNotFound] = useState(false);

  // Load the poll when someone opens a poll link
  useEffect(() => {
    if (!pollId) {
      return;
    }

    fetch(`http://localhost:8080/polls/${pollId}`)
      .then(async (response) => {
        if (!response.ok) {
          setPollNotFound(true);
          return null;
        }

        return response.json();
      })
      .then((data) => {
        if (data) {
          setPoll(data);
        }
      });

    // Connect to the WebSocket for live updates
    const socket = new WebSocket(
      `ws://localhost:8080/ws/${pollId}`
    );

    socket.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);

        if (data.id) {
          setPoll(data);
        }
      } catch (error) {
        console.log("WebSocket:", event.data);
      }
    };

    return () => {
      socket.close();
    };
  }, [pollId]);

  // Create a new poll
  const createPoll = async () => {
    if (!question.trim() || !option1.trim() || !option2.trim()) {
      alert("Please enter a question and both options.");
      return;
    }

    const response = await fetch("http://localhost:8080/polls", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        question: question,
        options: [option1, option2],
      }),
    });

    const data = await response.json();

    setCreatedPoll(data);
    setPoll(data);

    window.history.pushState({}, "", `/poll/${data.id}`);
  };

  // Vote for an option
  const vote = async (option) => {
    const response = await fetch(
      `http://localhost:8080/polls/${poll.id}/vote`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          option: option,
        }),
      }
    );

    const data = await response.json();

    setPoll(data);
  };

  // Copy the current poll link
  const copyLink = async () => {
    try {
      await navigator.clipboard.writeText(window.location.href);
      alert("Poll link copied!");
    } catch (error) {
      alert("Could not copy the link. Please copy it manually.");
    }
  };

  return (
    <div className="app">

      {/* Create Poll Screen */}
      {!poll && !pollNotFound && (
        <div className="card create-card">

          <div className="poll-header">
            <div className="app-title">
              <span className="app-icon">◈</span>

              <div>
                <strong>Live Polling</strong>
                <small>Create · Share · Vote Live</small>
              </div>
            </div>

            <div className="live-status">
              <span className="live-dot"></span>
              LIVE
            </div>
          </div>

          <div className="create-heading">
            <h1>Create a Poll</h1>
            <p>
              Ask a question and let people vote in real time.
            </p>
          </div>

          <input
            type="text"
            placeholder="Enter your question"
            value={question}
            onChange={(event) => setQuestion(event.target.value)}
          />

          <input
            type="text"
            placeholder="Option 1"
            value={option1}
            onChange={(event) => setOption1(event.target.value)}
          />

          <input
            type="text"
            placeholder="Option 2"
            value={option2}
            onChange={(event) => setOption2(event.target.value)}
          />

          <button onClick={createPoll}>
            Create Poll
          </button>

          {createdPoll && (
            <div>
              <h3>Poll Created!</h3>
              <p>Poll ID: {createdPoll.id}</p>
            </div>
          )}
        </div>
      )}

      {/* Poll Not Found Screen */}
      {pollNotFound && (
        <div className="card">
          <h1>Poll Not Found</h1>

          <p className="error-message">
            This poll does not exist or may have been removed.
          </p>
        </div>
      )}

      {/* Poll Results Screen */}
      {poll && (
        <div className="card poll-card">

          <div className="poll-header">
            <div className="app-title">
              <span className="app-icon">◈</span>

              <div>
                <strong>Live Polling</strong>
                <small>Create · Share · Vote Live</small>
              </div>
            </div>

            <div className="live-status">
              <span className="live-dot"></span>
              LIVE
            </div>
          </div>

          <h1>{poll.question}</h1>

          {poll.options.map((option) => {
            const totalVotes = Object.values(poll.votes).reduce(
              (sum, count) => sum + count,
              0
            );

            const percentage =
              totalVotes === 0
                ? 0
                : Math.round(
                    (poll.votes[option] / totalVotes) * 100
                  );

            return (
              <div className="poll-option" key={option}>

                <div className="option-header">

                  <button onClick={() => vote(option)}>
                    {option}
                  </button>

                  <span>
                    {poll.votes[option]} votes · {percentage}%
                  </span>

                </div>

                <div className="bar-background">
                  <div
                    className="bar-fill"
                    style={{ width: `${percentage}%` }}
                  ></div>
                </div>

              </div>
            );
          })}

          {/* Share Poll */}
          <div className="share-box">

            <p>Share this poll:</p>

            <input
              type="text"
              value={window.location.href}
              readOnly
            />

            <button onClick={copyLink}>
              Copy Link
            </button>

          </div>

        </div>
      )}

    </div>
  );
}

export default App;