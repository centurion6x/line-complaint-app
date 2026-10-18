import { useRef, useState } from "react";
import { getIdToken, closeLiffWindow } from "./liff";

const MAX_PHOTOS = 4;
const MAX_PHOTO_MB = 10;

export default function ComplaintForm() {
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [photos, setPhotos] = useState([]); // { file, previewUrl }
  const [status, setStatus] = useState("idle"); // idle | submitting | done | error
  const [errorMessage, setErrorMessage] = useState("");
  const [ticketId, setTicketId] = useState("");
  const fileInputRef = useRef(null);

  function handleFiles(fileList) {
    const incoming = Array.from(fileList);
    const accepted = [];
    let rejection = "";

    for (const file of incoming) {
      if (photos.length + accepted.length >= MAX_PHOTOS) {
        rejection = `You can attach up to ${MAX_PHOTOS} photos.`;
        break;
      }
      if (!file.type.startsWith("image/")) {
        rejection = "Only image files are accepted.";
        continue;
      }
      if (file.size > MAX_PHOTO_MB * 1024 * 1024) {
        rejection = `Each photo must be under ${MAX_PHOTO_MB}MB.`;
        continue;
      }
      accepted.push({ file, previewUrl: URL.createObjectURL(file) });
    }

    if (accepted.length > 0) {
      setPhotos((prev) => [...prev, ...accepted]);
    }
    if (rejection) {
      setErrorMessage(rejection);
    }
  }

  function removePhoto(index) {
    setPhotos((prev) => {
      const next = [...prev];
      const [removed] = next.splice(index, 1);
      if (removed) URL.revokeObjectURL(removed.previewUrl);
      return next;
    });
  }

  async function handleSubmit(event) {
    event.preventDefault();
    setErrorMessage("");

    if (!title.trim()) {
      setErrorMessage("Please enter a title.");
      return;
    }

    setStatus("submitting");

    try {
      const idToken = getIdToken();

      const form = new FormData();
      form.append("idToken", idToken);
      form.append("title", title.trim());
      form.append("description", description.trim());
      for (const { file } of photos) {
        form.append("photos", file);
      }

      const response = await fetch("/api/complaints", {
        method: "POST",
        body: form,
      });

      const body = await response.json();

      if (!response.ok || !body.ok) {
        throw new Error(body.error || "Something went wrong submitting your complaint.");
      }

      setTicketId(body.ticketId);
      setStatus("done");
    } catch (err) {
      setErrorMessage(err.message || "Something went wrong submitting your complaint.");
      setStatus("error");
    }
  }

  if (status === "done") {
    return (
      <div className="screen">
        <div className="success-card">
          <div className="success-icon" aria-hidden="true">
            <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="#06C755" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
              <path d="M20 6 9 17l-5-5" />
            </svg>
          </div>
          <h1>Complaint submitted</h1>
          <p>
            Ticket <strong>{ticketId}</strong> — check your chat with us for a
            confirmation message.
          </p>
          <button type="button" className="btn btn-primary" onClick={closeLiffWindow}>
            Close
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="screen">
      <form className="form" onSubmit={handleSubmit}>
        <h1>File a Complaint</h1>
        <p className="subtitle">We'll confirm receipt right here in chat.</p>

        <label className="field">
          <span>Title</span>
          <input
            type="text"
            value={title}
            maxLength={200}
            placeholder="Short summary"
            onChange={(e) => setTitle(e.target.value)}
            disabled={status === "submitting"}
          />
        </label>

        <label className="field">
          <span>Description</span>
          <textarea
            value={description}
            maxLength={2000}
            rows={5}
            placeholder="What happened?"
            onChange={(e) => setDescription(e.target.value)}
            disabled={status === "submitting"}
          />
        </label>

        <div className="field">
          <span>Photos ({photos.length}/{MAX_PHOTOS})</span>

          <div className="photo-grid">
            {photos.map((p, i) => (
              <div className="photo-thumb" key={p.previewUrl}>
                <img src={p.previewUrl} alt="" />
                <button
                  type="button"
                  className="photo-remove"
                  aria-label="Remove photo"
                  onClick={() => removePhoto(i)}
                  disabled={status === "submitting"}
                >
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="#FFFFFF" strokeWidth="3" strokeLinecap="round">
                    <path d="M6 6l12 12M18 6L6 18" />
                  </svg>
                </button>
              </div>
            ))}

            {photos.length < MAX_PHOTOS && (
              <button
                type="button"
                className="photo-add"
                onClick={() => fileInputRef.current?.click()}
                disabled={status === "submitting"}
              >
                <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="#06C755" strokeWidth="1.8" strokeLinecap="round">
                  <path d="M4 7h3l2-2h6l2 2h3v12H4z" />
                  <circle cx="12" cy="13" r="3.5" />
                </svg>
                <span>Add</span>
              </button>
            )}
          </div>

          {/* capture="environment" offers the camera directly on mobile;
              users can still pick an existing photo from their library. */}
          <input
            ref={fileInputRef}
            type="file"
            accept="image/*"
            capture="environment"
            multiple
            hidden
            onChange={(e) => {
              handleFiles(e.target.files);
              e.target.value = "";
            }}
          />
        </div>

        {errorMessage && <div className="error-banner">{errorMessage}</div>}

        <button type="submit" className="btn btn-primary" disabled={status === "submitting"}>
          {status === "submitting" ? "Submitting…" : "Submit Complaint"}
        </button>
      </form>
    </div>
  );
}
