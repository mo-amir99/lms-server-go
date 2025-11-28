// Get token from URL
const urlParams = new URLSearchParams(window.location.search);
const token = urlParams.get("token");

if (!token) {
  showMessage("Invalid or missing reset token.", "error");
  document.getElementById("resetForm").style.display = "none";
}

document.getElementById("resetForm").addEventListener("submit", async function (e) {
  e.preventDefault();

  const newPassword = document.getElementById("newPassword").value;
  const confirmPassword = document.getElementById("confirmPassword").value;

  // Validate passwords match
  if (newPassword !== confirmPassword) {
    showMessage("Passwords do not match.", "error");
    return;
  }

  // Show loading
  document.getElementById("submitBtn").disabled = true;
  document.getElementById("loading").style.display = "block";

  try {
    const response = await fetch("/api/auth/resetPassword", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        token: token,
        newPassword: newPassword,
      }),
    });
    
    const data = await response.json();
    
    if (response.ok && data.success) {
      showMessage(
        "Password reset successful! You can now login with your new password.",
        "success"
      );
      document.getElementById("resetForm").style.display = "none";
      setTimeout(() => {
        window.close();
      }, 3000);
    } else {
      showMessage(
        data.message || "Password reset failed. Please try again.",
        "error"
      );
    }
  } catch (error) {
    console.error("Reset error:", error);
    showMessage(
      "Network error. Please check your connection and try again.",
      "error"
    );
  } finally {
    document.getElementById("submitBtn").disabled = false;
    document.getElementById("loading").style.display = "none";
  }
});

document.getElementById("backBtn").addEventListener("click", function(e) {
  e.preventDefault();
  window.close();
});

function showMessage(message, type) {
  const messageDiv = document.getElementById("message");
  messageDiv.innerHTML = `<div class="message ${type}">${message}</div>`;
}
