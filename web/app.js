const reviewsEl = document.getElementById("reviews");
const inputEl = document.getElementById("input");
const generateBtn = document.getElementById("generate");
const newChatBtn = document.getElementById("new-chat");
const hideSidebarBtn = document.getElementById("hide-sidebar");
const showSidebarBtn = document.getElementById("show-sidebar");
const sidebar = document.getElementById("sidebar");
const modelSelect = document.getElementById("model-select");

let reviews = [];
let userTabs = [];

function renderReviews() {
  reviewsEl.innerHTML = "";
  for (const review of reviews) {
    const div = document.createElement("div");
    div.className = "bg-gray-700 p-2 rounded shadow";
    div.textContent = review;
    reviewsEl.appendChild(div);
  }
}

function autoResize(textarea) {
  textarea.style.height = "auto";
  textarea.style.height = textarea.scrollHeight + "px";
}

function renderSidebarTabs() {
  const sidebarTabs = document.createElement("div");
  sidebarTabs.className = "flex flex-col gap-2 p-2";
  for (const tab of userTabs) {
    const button = document.createElement("button");
    button.className = "bg-gray-600 p-2 rounded text-left hover:bg-gray-500";
    button.textContent = `Review ${tab.id} (${new Date(tab.date).toLocaleString()})`;
    button.addEventListener("click", () => {
      reviewsEl.innerHTML = "";
      const div = document.createElement("div");
      div.className = "bg-gray-700 p-2 rounded shadow";
      div.textContent = `Request: ${tab.request}\nAnswer: ${tab.answer}\nModel: ${tab.model}\nThink: ${tab.think || "N/A"}`;
      reviewsEl.appendChild(div);
    });
    sidebarTabs.appendChild(button);
  }
  sidebar.innerHTML = "";
  sidebar.appendChild(sidebarTabs);
}

async function fetchReviews() {
  try {
    const response = await fetch("http://localhost:8080/user/review/get", {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });
    if (!response.ok) throw new Error("Failed to fetch reviews");
    userTabs = await response.json();
    renderSidebarTabs();
  } catch (error) {
    console.error("Error fetching reviews:", error);
    reviewsEl.innerHTML = "<div class='bg-red-700 p-2 rounded shadow'>Error loading reviews</div>";
  }
}

generateBtn.addEventListener("click", async () => {
  const text = inputEl.value.trim();
  if (!text) return;

  const selectedModel = modelSelect.value || "minstral-8b-2410";
  let review;

  try {
    const response = await fetch("http://localhost:8080/user/review/add", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        request: text,
        model: selectedModel,
      }),
    });
    if (!response.ok) throw new Error("Failed to add review");
    const newTab = await response.json();

    if (selectedModel === "magistral-medium-2506") {
      review = `Отзыв (с размышлением, модель ${selectedModel}): ${text}`;
    } else {
      review = `Отзыв (без размышления, модель ${selectedModel}): ${text}`;
    }

    reviews.push(review);
    userTabs.push(newTab);
    inputEl.value = "";
    autoResize(inputEl);
    renderReviews();
    renderSidebarTabs();
  } catch (error) {
    console.error("Error adding review:", error);
    reviewsEl.innerHTML = "<div class='bg-red-700 p-2 rounded shadow'>Error adding review</div>";
  }
});

newChatBtn.addEventListener("click", () => {
  reviews = [];
  renderReviews();
});

hideSidebarBtn.addEventListener("click", () => {
  sidebar.classList.add("hidden");
  showSidebarBtn.classList.remove("hidden");
});

showSidebarBtn.addEventListener("click", () => {
  sidebar.classList.remove("hidden");
  showSidebarBtn.classList.add("hidden");
});

// Fetch reviews on page load
document.addEventListener("DOMContentLoaded", fetchReviews);