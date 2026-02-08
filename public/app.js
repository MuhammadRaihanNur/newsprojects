const form = document.getElementById("uploadForm");
const imageInput = document.getElementById("imageInput");
const captionInput = document.getElementById("captionInput");
const previewImg = document.getElementById("previewImg");
const previewBox = document.getElementById("previewBox");
const statusEl = document.getElementById("status");
const postsEl = document.getElementById("posts");
const refreshBtn = document.getElementById("refreshBtn");

const API = "/api/posts";

function formatTime(isoOrDate) {
  const d = new Date(isoOrDate);
  return d.toLocaleString("id-ID", { dateStyle: "medium", timeStyle: "short" });
}

imageInput.addEventListener("change", () => {
  const f = imageInput.files?.[0];
  if (!f) {
    previewImg.style.display = "none";
    previewBox.querySelector(".muted").style.display = "block";
    return;
  }
  const url = URL.createObjectURL(f);
  previewImg.src = url;
  previewImg.style.display = "block";
  previewBox.querySelector(".muted").style.display = "none";
});

async function loadPosts() {
  postsEl.innerHTML = `<div class="muted">Loading...</div>`;
  try {
    const res = await fetch(API);
    const data = await res.json();

    if (!Array.isArray(data) || data.length === 0) {
      postsEl.innerHTML = `<div class="muted">Belum ada berita.</div>`;
      return;
    }

    postsEl.innerHTML = data.map(p => `
  <a class="post" href="/detail.html?id=${p.id}" style="text-decoration:none; color:inherit; display:block;">
    <img src="${p.imageUrl}" alt="news image"/>
    <div class="content">
      <p class="caption">${escapeHtml(p.caption)}</p>
      <div class="meta">${formatTime(p.createdAt)}</div>
    </div>
  </a>
`).join("");

  } catch (e) {
    postsEl.innerHTML = `<div class="muted">Gagal load: ${e.message}</div>`;
  }
}

form.addEventListener("submit", async (e) => {
  e.preventDefault();
  statusEl.textContent = "";

  const file = imageInput.files?.[0];
  const caption = captionInput.value.trim();

  if (!file || !caption) {
    statusEl.textContent = "Gambar dan keterangan wajib diisi.";
    return;
  }

  const fd = new FormData();
  fd.append("image", file);
  fd.append("caption", caption);

  try {
    statusEl.textContent = "Uploading...";
    const res = await fetch(API, { method: "POST", body: fd });

    if (!res.ok) {
      const msg = await res.text();
      throw new Error(msg || "Upload failed");
    }

    // reset
    captionInput.value = "";
    imageInput.value = "";
    previewImg.style.display = "none";
    previewBox.querySelector(".muted").style.display = "block";

    statusEl.textContent = "Upload berhasil ✅";
    await loadPosts();
  } catch (e2) {
    statusEl.textContent = "Upload gagal: " + e2.message;
  }
});

refreshBtn.addEventListener("click", loadPosts);

function escapeHtml(str) {
  return str.replace(/[&<>"']/g, (m) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#039;"
  }[m]));
}

// initial load
loadPosts();
