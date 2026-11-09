const result = document.querySelector("#result");

async function request(button, url, options) {
  button.disabled = true;
  result.textContent = "Working...";
  try {
    const response = await fetch(url, options);
    const body = await response.text();
    result.textContent = response.ok ? body : `Request failed (${response.status}): ${body}`;
    return response.ok;
  } catch (error) {
    result.textContent = `API unavailable: ${error.message}`;
    return false;
  } finally {
    button.disabled = false;
  }
}

function postJSON(body) {
  return {
    method: "POST",
    headers: {"Content-Type": "application/json"},
    body: JSON.stringify(body)
  };
}

function submitForm(form, url) {
  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    const field = form.elements[0];
    const sent = await request(form.querySelector("button"), url, postJSON({[field.name]: field.value}));
    if (sent) form.reset();
  });
}

submitForm(document.querySelector("#create-item"), "/api/items");
submitForm(document.querySelector("#send-notification"), "/api/notify");

const loadButton = document.querySelector("#load-items");
loadButton.addEventListener("click", () => request(loadButton, "/api/items"));

async function showVersion() {
  const label = document.querySelector("#version");
  try {
    const response = await fetch("/api/version");
    if (response.status === 404) {
      label.textContent = "Served by an earlier version";
      return;
    }
    const body = await response.json();
    label.textContent = `Served by version ${body.version.slice(0, 7)}`;
  } catch {
    label.textContent = "";
  }
}

showVersion();
