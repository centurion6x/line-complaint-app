import { useEffect, useState } from "react";
import { initLiff } from "./liff";
import ComplaintForm from "./ComplaintForm";

export default function App() {
  const [status, setStatus] = useState("loading"); // loading | ready | error
  const [error, setError] = useState("");

  useEffect(() => {
    initLiff()
      .then(() => setStatus("ready"))
      .catch((err) => {
        setError(err.message || "Could not start the LINE session.");
        setStatus("error");
      });
  }, []);

  if (status === "loading") {
    return (
      <div className="screen">
        <div className="centered-message">Loading…</div>
      </div>
    );
  }

  if (status === "error") {
    return (
      <div className="screen">
        <div className="centered-message error-text">
          {error}
          <br />
          Please open this page from the LINE chat.
        </div>
      </div>
    );
  }

  return <ComplaintForm />;
}
