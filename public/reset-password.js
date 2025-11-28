const urlParams = new URLSearchParams(window.location.search);
const token = urlParams.get("token");
const resetForm = document.getElementById("resetForm");
const submitBtn = document.getElementById("submitBtn");
const loadingIndicator = document.getElementById("loading");
const backBtn = document.getElementById("backBtn");

if (!token) {
  showMessage("Invalid or missing reset token.", "error");
  if (resetForm) {
    resetForm.style.display = "none";
  }
}

if (resetForm) {
  resetForm.addEventListener("submit", async function (e) {
    e.preventDefault();

    const newPassword = document.getElementById("newPassword").value;
    const confirmPassword = document.getElementById("confirmPassword").value;

    if (newPassword !== confirmPassword) {
      showMessage("Passwords do not match.", "error");
      return;
    }

    toggleLoading(true);

    try {
      const response = await fetch("/api/auth/resetPassword", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          token,
          newPassword,
        }),
      });

      let payload = null;
      try {
        payload = await response.json();
      } catch (_err) {
        // Ignore JSON parse errors; payload stays null
      }

      const succeeded = response.ok && payload?.success === true;

      if (succeeded) {
        const successMessage =
          payload?.message || "Password reset successful. You can now sign in with your new password.";
        showMessage(successMessage, "success");
        resetForm.style.display = "none";
        return;
      }

      const errorMessage =
        payload?.message ||
        payload?.error ||
        `Password reset failed (HTTP ${response.status}). Please try again.`;
      showMessage(errorMessage, "error");
    } catch (error) {
      console.error("Reset error:", error);
      showMessage(
        "Network error. Please check your connection and try again.",
        "error"
      );
    } finally {
      toggleLoading(false);
    }
  });
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

function toggleLoading(isLoading) {
  if (!submitBtn || !loadingIndicator) {
    return;
  }
  submitBtn.disabled = isLoading;
  loadingIndicator.style.display = isLoading ? "block" : "none";
}

function showMessage(message, type) {
  const messageDiv = document.getElementById("message");
  if (!messageDiv) return;
  messageDiv.innerHTML = `<div class="message ${type}">${message}</div>`;
}
