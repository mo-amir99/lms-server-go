// Get token from URL
const urlParams = new URLSearchParams(window.location.search);
const token = urlParams.get("token");

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
        token: token,
      }),
    });

    const data = await response.json();

    if (response.ok && data.success) {
      showResult(
        "Email verification successful! Your account is now verified.",
        true
      );

      // Redirect after 3 seconds
      setTimeout(() => {
        window.close();
      }, 3000);
    } else {
      showResult(
        data.message || "Email verification failed. Please try again.",
        false
      );
    }
  } catch (error) {
    console.error("Verification error:", error);
    showResult(
      "Network error. Please check your connection and try again.",
      false
    );
  }
}

document.getElementById("backBtn").addEventListener("click", function(e) {
  e.preventDefault();
  window.close();
});

function showResult(message, success) {
  const content = document.getElementById("content");
  const iconClass = success ? "success-icon" : "error-icon";
  const icon = success ? "✓" : "✗";

  content.innerHTML = `
    <div class="status-icon ${iconClass}">${icon}</div>
    <div class="result-message">${success ? "Email Verified!" : "Verification Failed"}</div>
    <div class="result-description">${message}</div>
  `;
}
