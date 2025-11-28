const urlParams = new URLSearchParams(window.location.search);
const token = urlParams.get("token");
const backBtn = document.getElementById("backBtn");

if (!token) {
  showResult("Invalid or missing verification token.", false);
} else {
  verifyEmail();
}

async function verifyEmail() {
  try {
    const response = await fetch("/api/auth/verifyEmail", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        token,
      }),
    });

    let payload = null;
    try {
      payload = await response.json();
    } catch (_err) {
      // ignore JSON parse issues
    }

    const succeeded = response.ok && payload?.success === true;

    if (succeeded) {
      const message =
        payload?.message ||
        "Email verification successful! Your account is now verified.";
      showResult(message, true);
      return;
    }

    const errorMessage =
      payload?.message ||
      payload?.error ||
      `Email verification failed (HTTP ${response.status}). Please try again.`;
    showResult(errorMessage, false);
  } catch (error) {
    console.error("Verification error:", error);
    showResult(
      "Network error. Please check your connection and try again.",
      false
    );
  }
}

if (backBtn) {
  backBtn.addEventListener("click", function (e) {
    e.preventDefault();
    if (document.referrer) {
      window.location.href = document.referrer;
    } else {
      window.location.href = "/";
    }
  });
}

function showResult(message, success) {
  const content = document.getElementById("content");
  if (!content) return;
  const iconClass = success ? "success-icon" : "error-icon";
  const icon = success ? "✓" : "✗";

  content.innerHTML = `
    <div class="status-icon ${iconClass}">${icon}</div>
    <div class="result-message">${
      success ? "Email Verified!" : "Verification Failed"
    }</div>
    <div class="result-description">${message}</div>
  `;
}
