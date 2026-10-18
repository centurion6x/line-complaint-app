import liff from "@line/liff";

const LIFF_ID = import.meta.env.VITE_LIFF_ID;

let initPromise = null;

/**
 * Initializes the LIFF SDK exactly once per page load. Safe to call
 * from multiple components — they all await the same promise.
 *
 * Requires the "openid" scope (the default when a LIFF app is added to
 * a channel) so that liff.getIDToken() returns a usable token.
 */
export function initLiff() {
  if (!initPromise) {
    if (!LIFF_ID) {
      initPromise = Promise.reject(
        new Error(
          "VITE_LIFF_ID is not set — this build was not configured with a LIFF ID."
        )
      );
    } else {
      initPromise = liff.init({ liffId: LIFF_ID }).then(() => {
        // Opened outside LINE (e.g. a normal mobile/desktop browser):
        // liff.login() redirects to LINE's login page and back.
        // Opened inside LINE: the user is already logged in, this is a
        // no-op.
        if (!liff.isLoggedIn() && !liff.isInClient()) {
          liff.login();
        }
      });
    }
  }
  return initPromise;
}

export function isRunningInLineClient() {
  return liff.isInClient();
}

/**
 * Returns a fresh ID token for the current session. The backend uses
 * this to recover the caller's LINE user ID server-side — see
 * VerifyIDToken in the Go backend.
 */
export function getIdToken() {
  const token = liff.getIDToken();
  if (!token) {
    throw new Error("No LINE session is available — please reopen this page from LINE.");
  }
  return token;
}

export function closeLiffWindow() {
  if (liff.isInClient()) {
    liff.closeWindow();
  }
}
