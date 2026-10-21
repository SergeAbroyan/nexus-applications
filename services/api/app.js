const form = document.querySelector("#create-item");
const input = document.querySelector("#item-name");
const loadButton = document.querySelector("#load-items");
const result = document.querySelector("#result");

async function showResponse(response) {
  const body = await response.text();
  result.textContent = response.ok ? body : `Request failed (${response.status}): ${body}`;
}

loadButton.addEventListener("click", async () => {
  loadButton.disabled = true;
  result.textContent = "Loading...";
  try {
    await showResponse(await fetch("/api/items"));
  } catch (error) {
    result.textContent = `API unavailable: ${error.message}`;
  } finally {
    loadButton.disabled = false;
  }
});

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  result.textContent = "Saving...";
  try {
    const response = await fetch("/api/items", {
      method: "POST",
      headers: {"Content-Type": "application/json"},
      body: JSON.stringify({name: input.value})
    });
    await showResponse(response);
    if (response.ok) input.value = "";
  } catch (error) {
    result.textContent = `API unavailable: ${error.message}`;
  }
});
