const statusEl = document.getElementById("detailStatus");
const wrapEl = document.getElementById("detailWrap");
const imgEl = document.getElementById("detailImg");
const captionEl = document.getElementById("detailCaption");
const metaEl = document.getElementById("detailMeta");

function formatTime(isoOrDate) {
    const d = new Date(isoOrDate);
    return d.toLocaleString("id-ID", { dateStyle: "full", timeStyle: "short" });
}

function getId() {
    const url = new URL(window.location.href);
    return url.searchParams.get("id");
}

async function loadDetail() {
    const id = getId();
    if (!id) {
        statusEl.textContent = "ID berita tidak ditemukan di URL. Contoh: detail.html?id=1";
        return;
    }

    try {
        const res = await fetch(`/api/posts/${encodeURIComponent(id)}`);
        if (!res.ok) {
            const msg = await res.text();
            throw new Error(msg || `HTTP ${res.status}`);
        }

        const p = await res.json();

        imgEl.src = p.imageUrl;
        captionEl.textContent = p.caption;
        metaEl.textContent = formatTime(p.createdAt);

        statusEl.style.display = "none";
        wrapEl.style.display = "block";
    } catch (e) {
        statusEl.textContent = "Gagal load detail: " + e.message;
    }
}

loadDetail();
