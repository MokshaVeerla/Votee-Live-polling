import { useEffect, useState } from "react";
import "./App.css";

const API_URL =
  import.meta.env.VITE_API_URL || "http://localhost:8080";

const WS_URL =
  import.meta.env.VITE_WS_URL || "ws://localhost:8080";

function App() {
  const [isLoggedIn, setIsLoggedIn] = useState(
    localStorage.getItem("pollAuth") === "true"
  );

  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [loginError, setLoginError] = useState("");

  const [question, setQuestion] = useState("");
  const [option1, setOption1] = useState("");
  const [option2, setOption2] = useState("");

  const [poll, setPoll] = useState(null);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState(false);

  const path = window.location.pathname;

  const pollId = path.startsWith("/poll/")
    ? path.split("/poll/")[1]
    : null;

  // Get poll and connect to live updates
  useEffect(() => {
    if (!pollId) {
      return;
    }

    fetch(`${API_URL}/polls/${pollId}`)
      .then((response) => {
        if (!response.ok) {
          throw new Error("Poll not found");
        }

        return response.json();
      })
      .then((data) => {
        setPoll(data);
        setError("");
      })
      .catch(() => {
        setError("Poll not found");
      });

    const socket = new WebSocket(
      `${WS_URL}/ws/${pollId}`
    );

    socket.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);

        if (data.id) {
          setPoll(data);
        }
      } catch {
        console.log("Could not read live update");
      }
    };

    socket.onerror = () => {
      console.log("WebSocket connection error");
    };

    return () => {
      socket.close();
    };
  }, [pollId]);

  // Login
  const handleLogin = async (event) => {
    event.preventDefault();

    setLoginError("");

    if (!username.trim() || !password.trim()) {
      setLoginError("Please enter your username and password");
      return;
    }

    try {
      const response = await fetch(
        `${API_URL}/login`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            username,
            password,
          }),
        }
      );

      const data = await response.json();

      if (!response.ok) {
        setLoginError(data.error || "Login failed");
        return;
      }

      localStorage.setItem("pollAuth", "true");
      localStorage.setItem("pollToken", data.token);

      setIsLoggedIn(true);
      setUsername("");
      setPassword("");
    } catch {
      setLoginError("Could not connect to the server");
    }
  };

  // Create a new poll
  const handleCreatePoll = async (event) => {
    event.preventDefault();

    setError("");

    if (!question.trim()) {
      setError("Please enter a question");
      return;
    }

    if (!option1.trim() || !option2.trim()) {
      setError("Please enter both options");
      return;
    }

    try {
      const token = localStorage.getItem("pollToken");

      const response = await fetch(
        `${API_URL}/polls`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({
            question,
            options: [option1, option2],
          }),
        }
      );

      const data = await response.json();

      if (!response.ok) {
        setError(data.error || "Could not create poll");
        return;
      }

      window.location.href = `/poll/${data.id}`;
    } catch {
      setError("Could not connect to the server");
    }
  };

  // Vote for an option
  const handleVote = async (option) => {
    if (!poll) {
      return;
    }

    try {
      const response = await fetch(
        `${API_URL}/polls/${poll.id}/vote`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            option,
          }),
        }
      );

      const data = await response.json();

      if (!response.ok) {
        setError(data.error || "Could not record vote");
        return;
      }

      setPoll(data);
      setError("");
    } catch {
      setError("Could not connect to the server");
    }
  };

  // Copy the poll link
  const copyLink = async () => {
    try {
      await navigator.clipboard.writeText(
        window.location.href
      );

      setCopied(true);

      setTimeout(() => {
        setCopied(false);
      }, 2000);
    } catch {
      setError("Could not copy the link");
    }
  };

  // Logout
  const handleLogout = () => {
    localStorage.removeItem("pollAuth");
    localStorage.removeItem("pollToken");

    setIsLoggedIn(false);
  };

  // Shared poll page
  if (pollId) {
    if (error && !poll) {
      return (
        <div className="app">
          <div className="card">
            <div className="brand">VOTEE</div>

            <div className="not-found">
              <span className="small-label">
                POLL ERROR
              </span>

              <h1>Poll Not Found</h1>

              <p>
                This poll may have been deleted or the
                link may be incorrect.
              </p>
            </div>
          </div>
        </div>
      );
    }

    if (!poll) {
      return (
        <div className="app">
          <div className="card loading">
            <div className="brand">VOTEE</div>
            <p>Loading poll...</p>
          </div>
        </div>
      );
    }

    const totalVotes = Object.values(poll.votes).reduce(
      (total, votes) => total + votes,
      0
    );

    return (
      <div className="app">
        <div className="card poll-card">

          <div className="top-bar">
            <div className="brand">
              <span className="pulse-symbol">✦</span>
              VOTEE
            </div>

            <div className="live-badge">
              <span className="live-dot"></span>
              LIVE
            </div>
          </div>

          <div className="poll-heading">
            <span className="small-label">
              QUICK POLL
            </span>

            <h1>{poll.question}</h1>

            <p className="vote-total">
              {totalVotes}{" "}
              {totalVotes === 1 ? "person has" : "people have"}{" "}
              voted
            </p>
          </div>

          <div className="options">
            {poll.options.map((option, index) => {
              const count = poll.votes[option] || 0;

              const percentage =
                totalVotes === 0
                  ? 0
                  : Math.round((count / totalVotes) * 100);

              return (
                <div className="option-card" key={option}>

                  <div className="option-number">
                    {String(index + 1).padStart(2, "0")}
                  </div>

                  <div className="option-content">

                    <div className="option-header">
                      <span className="option-name">
                        {option}
                      </span>

                      <span className="option-percent">
                        {percentage}%
                      </span>
                    </div>

                    <div className="progress-background">
                      <div
                        className="progress"
                        style={{
                          width: `${percentage}%`,
                        }}
                      ></div>
                    </div>

                    <span className="option-votes">
                      {count}{" "}
                      {count === 1 ? "vote" : "votes"}
                    </span>
                  </div>

                  <button
                    className="vote-button"
                    onClick={() => handleVote(option)}
                  >
                    Vote
                  </button>

                </div>
              );
            })}
          </div>

          {error && (
            <p className="error-message">
              {error}
            </p>
          )}

          <div className="live-status">
            <div>
              <span className="live-dot"></span>
              Results update automatically
            </div>

            <span>
              {totalVotes} total
            </span>
          </div>

          <div className="share-section">

            <div>
              <span className="small-label">
                SHARE THIS POLL
              </span>

              <p>
                Send this link to let others vote.
              </p>
            </div>

            <div className="share-box">
              <input
                value={window.location.href}
                readOnly
              />

              <button onClick={copyLink}>
                {copied ? "Copied!" : "Copy"}
              </button>
            </div>

          </div>

        </div>
      </div>
    );
  }

  // Login page
  if (!isLoggedIn) {
    return (
      <div className="app">
        <div className="card login-card">

          <div className="login-brand">
            <div className="brand large-brand">
              <span className="pulse-symbol">✦</span>
              VOTEE
            </div>
          </div>

          <div className="login-heading">
            <h1>Welcome back</h1>

            <p>
              Create a poll and let everyone see
              the results live.
            </p>
          </div>

          <form onSubmit={handleLogin}>

            <label>Username</label>

            <input
              className="text-input"
              type="text"
              placeholder="Enter username"
              value={username}
              onChange={(event) =>
                setUsername(event.target.value)
              }
            />

            <label>Password</label>

            <input
              className="text-input"
              type="password"
              placeholder="Enter password"
              value={password}
              onChange={(event) =>
                setPassword(event.target.value)
              }
            />

            {loginError && (
              <p className="error-message">
                {loginError}
              </p>
            )}

            <button
              className="primary-button"
              type="submit"
            >
              Enter VOTEE
            </button>

          </form>

        </div>
      </div>
    );
  }

  // Create poll page
  return (
    <div className="app">
      <div className="card create-card">

        <div className="top-bar">
          <div className="brand">
            <span className="pulse-symbol">✦</span>
            VOTEE
          </div>

          <button
            className="logout-button"
            onClick={handleLogout}
          >
            Logout
          </button>
        </div>

        <div className="create-heading">
          <span className="small-label">
            NEW POLL
          </span>

          <h1>Ask the question.</h1>

          <p>
            Create a poll, share the link, and watch
            the responses arrive live.
          </p>
        </div>

        <form onSubmit={handleCreatePoll}>

          <label>Question</label>

          <input
            className="text-input"
            type="text"
            placeholder="What should we learn next?"
            value={question}
            onChange={(event) =>
              setQuestion(event.target.value)
            }
          />

          <div className="options-label">
            <label>Options</label>

            <span>2 choices</span>
          </div>

          <div className="create-option">
            <span>01</span>

            <input
              className="text-input"
              type="text"
              placeholder="First option"
              value={option1}
              onChange={(event) =>
                setOption1(event.target.value)
              }
            />
          </div>

          <div className="create-option">
            <span>02</span>

            <input
              className="text-input"
              type="text"
              placeholder="Second option"
              value={option2}
              onChange={(event) =>
                setOption2(event.target.value)
              }
            />
          </div>

          {error && (
            <p className="error-message">
              {error}
            </p>
          )}

          <button
            className="primary-button create-button"
            type="submit"
          >
            Create Poll
            <span>→</span>
          </button>

        </form>

        <div className="create-footer">
          <span className="live-dot"></span>
          Your results will update in real time
        </div>

      </div>
    </div>
  );
}

export default App;