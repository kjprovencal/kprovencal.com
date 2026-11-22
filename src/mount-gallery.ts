import { publicApiUrl } from "./api-base";

type GuestPhoto = {
  id: string;
  filename: string;
  width: number;
  height: number;
  web_url?: string;
};

type GuestAlbum = {
  title: string;
  locked: boolean;
  photos?: GuestPhoto[];
};

type DownloadVariant = "web" | "original";

function galleryFetch(path: string, init?: RequestInit): Promise<Response> {
  return fetch(publicApiUrl(path), {
    ...init,
    credentials: "include",
    cache: "no-store",
  });
}

function el<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  className?: string
): HTMLElementTagNameMap[K] {
  const node = document.createElement(tag);
  if (className) node.className = className;
  return node;
}

function parseVariant(raw: string): DownloadVariant | null {
  switch (raw) {
    case "web":
    case "original":
      return raw;
    default:
      return null;
  }
}

function variantLabel(variant: DownloadVariant): string {
  switch (variant) {
    case "web":
      return "Web (smaller)";
    case "original":
      return "Original";
    default: {
      const _never: never = variant;
      return _never;
    }
  }
}

export function mountGallery(token: string, root: HTMLElement): () => void {
  const ac = new AbortController();
  const { signal } = ac;
  root.classList.add("gallery");

  const heading = el("h1", "gallery__title");
  heading.textContent = "Photo album";
  const status = el("p", "gallery__status");
  status.setAttribute("aria-live", "polite");
  const stage = el("div", "gallery__stage");
  root.replaceChildren(heading, status, stage);

  const viewer = el("dialog", "gallery-viewer");
  const viewerImg = el("img", "gallery-viewer__img");
  const viewerName = el("p", "gallery-viewer__name");
  const viewerForm = el("form", "gallery-viewer__bar");
  const variantSelect = el("select", "gallery-field__input");
  variantSelect.name = "variant";
  for (const variant of ["web", "original"] as const) {
    const option = el("option");
    option.value = variant;
    option.textContent = variantLabel(variant);
    variantSelect.append(option);
  }
  const variantLabelEl = el("label", "gallery-field gallery-field--inline");
  const variantSpan = el("span", "gallery-field__label");
  variantSpan.textContent = "Download";
  variantLabelEl.append(variantSpan, variantSelect);
  const downloadBtn = el("button", "gallery-button gallery-button--primary");
  downloadBtn.type = "submit";
  downloadBtn.textContent = "Download";
  const prevBtn = el("button", "gallery-button");
  prevBtn.type = "button";
  prevBtn.textContent = "Previous";
  const nextBtn = el("button", "gallery-button");
  nextBtn.type = "button";
  nextBtn.textContent = "Next";
  const closeBtn = el("button", "gallery-button");
  closeBtn.type = "button";
  closeBtn.textContent = "Close";
  viewerForm.append(variantLabelEl, downloadBtn);
  const viewerNav = el("div", "gallery-actions");
  viewerNav.append(prevBtn, nextBtn, closeBtn);
  viewer.append(viewerImg, viewerName, viewerForm, viewerNav);
  root.append(viewer);

  let photos: GuestPhoto[] = [];
  let index = 0;
  let unlocked = false;
  let refreshing = false;
  let lastViewRefresh = 0;
  const viewRefreshMs = 10 * 60 * 1000;

  function watchViewImage(img: HTMLImageElement): void {
    img.addEventListener("error", () => {
      void refreshPhotos();
    });
  }

  function syncViewerImage(): void {
    if (!viewer.open) return;
    const current = photos[index];
    if (current?.web_url && viewerImg.src !== current.web_url) {
      viewerImg.src = current.web_url;
    }
  }

  watchViewImage(viewerImg);

  function showPhoto(next: number): void {
    if (photos.length === 0) return;
    index = (next + photos.length) % photos.length;
    const photo = photos[index];
    if (!photo) return;
    viewerImg.src = photo.web_url ?? "";
    viewerImg.alt = photo.filename;
    viewerName.textContent = photo.filename;
    if (!viewer.open) viewer.showModal();
  }

  closeBtn.addEventListener("click", () => viewer.close(), { signal });
  prevBtn.addEventListener("click", () => showPhoto(index - 1), { signal });
  nextBtn.addEventListener("click", () => showPhoto(index + 1), { signal });
  viewer.addEventListener(
    "click",
    (event) => {
      if (event.target === viewer) viewer.close();
    },
    { signal }
  );
  viewer.addEventListener(
    "cancel",
    (event) => {
      event.preventDefault();
      viewer.close();
    },
    { signal }
  );
  root.addEventListener(
    "keydown",
    (event) => {
      if (!viewer.open) return;
      if (
        event.target instanceof HTMLSelectElement ||
        event.target instanceof HTMLInputElement
      ) {
        return;
      }
      if (event.key === "ArrowLeft") showPhoto(index - 1);
      else if (event.key === "ArrowRight") showPhoto(index + 1);
    },
    { signal }
  );

  viewerForm.addEventListener(
    "submit",
    (event) => {
      event.preventDefault();
      const photo = photos[index];
      const variant = parseVariant(variantSelect.value);
      if (!photo || !variant) return;
      void downloadPhoto(photo.id, variant);
    },
    { signal }
  );

  async function downloadPhoto(photoId: string, variant: DownloadVariant): Promise<void> {
    status.textContent = "Preparing download…";
    try {
      const res = await galleryFetch(
        `/api/gallery/${token}/photos/${encodeURIComponent(photoId)}/download?variant=${variant}`,
        { signal }
      );
      if (signal.aborted) return;
      if (res.status === 401) {
        viewer.close();
        status.textContent = "Enter the PIN again to download.";
        renderLocked(heading.textContent || "Photo album");
        return;
      }
      if (!res.ok) {
        const data = (await res.json().catch(() => null)) as { error?: string } | null;
        status.textContent = data?.error ?? "Could not download this photo.";
        return;
      }
      const data = (await res.json()) as { url?: string };
      if (!data.url) {
        status.textContent = "Could not download this photo.";
        return;
      }
      const link = document.createElement("a");
      link.href = data.url;
      link.rel = "noopener";
      document.body.append(link);
      link.click();
      link.remove();
      status.textContent = "";
    } catch {
      if (!signal.aborted) status.textContent = "Could not download this photo.";
    }
  }

  function renderLocked(title: string): void {
    unlocked = false;
    heading.textContent = title;
    document.title = title;
    stage.replaceChildren();
    const form = el("form", "gallery-form gallery-pin");
    const hint = el("p", "gallery__hint");
    hint.textContent = "Enter the PIN to view and download these photos.";
    const label = el("label", "gallery-field");
    const span = el("span", "gallery-field__label");
    span.textContent = "PIN";
    const input = el("input", "gallery-field__input");
    input.type = "password";
    input.name = "pin";
    input.required = true;
    input.minLength = 4;
    input.autocomplete = "current-password";
    input.enterKeyHint = "go";
    label.append(span, input);
    const submit = el("button", "gallery-button gallery-button--primary");
    submit.type = "submit";
    submit.textContent = "View album";
    form.append(hint, label, submit);
    form.addEventListener(
      "submit",
      (event) => {
        event.preventDefault();
        void unlock(input.value);
      },
      { signal }
    );
    stage.append(form);
    input.focus();
  }

  function renderPhotos(title: string, rows: GuestPhoto[]): void {
    unlocked = true;
    heading.textContent = title;
    document.title = title;
    photos = rows;
    stage.replaceChildren();
    if (rows.length === 0) {
      const empty = el("p", "gallery__empty");
      empty.textContent = "No photos in this album yet.";
      stage.append(empty);
      return;
    }
    const grid = el("ul", "gallery-grid");
    rows.forEach((photo, photoIndex) => {
      const item = el("li");
      const open = el("button", "gallery-card gallery-card--button");
      open.type = "button";
      if (photo.web_url) {
        const img = el("img", "gallery-card__img");
        img.src = photo.web_url;
        img.alt = "";
        watchViewImage(img);
        open.append(img);
      }
      const name = el("span", "gallery-card__name");
      name.textContent = photo.filename;
      open.append(name);
      open.addEventListener("click", () => showPhoto(photoIndex), { signal });
      item.append(open);
      grid.append(item);
    });
    stage.append(grid);
  }

  async function refreshPhotos(): Promise<void> {
    if (!unlocked || refreshing || signal.aborted) return;
    const now = Date.now();
    if (now - lastViewRefresh < 5000) return;
    lastViewRefresh = now;
    refreshing = true;
    try {
      const res = await galleryFetch(`/api/gallery/${token}`, { signal });
      if (signal.aborted || !res.ok) return;
      const album = (await res.json()) as GuestAlbum;
      if (album.locked) {
        if (viewer.open) viewer.close();
        renderLocked(album.title);
        return;
      }
      const rows = album.photos ?? [];
      const buttons = stage.querySelectorAll(".gallery-card--button");
      if (buttons.length !== rows.length) {
        const keepIndex = index;
        const keepOpen = viewer.open && rows.length > 0;
        renderPhotos(album.title, rows);
        if (keepOpen) showPhoto(Math.min(keepIndex, rows.length - 1));
        else if (viewer.open) viewer.close();
        return;
      }
      photos = rows;
      rows.forEach((photo, i) => {
        const img = buttons[i]?.querySelector("img");
        if (img instanceof HTMLImageElement && photo.web_url && img.src !== photo.web_url) {
          img.src = photo.web_url;
        }
      });
      syncViewerImage();
    } catch {
      /* Keep the photos already on screen. */
    } finally {
      refreshing = false;
    }
  }

  async function load(): Promise<void> {
    status.textContent = "Loading…";
    try {
      const res = await galleryFetch(`/api/gallery/${token}`, { signal });
      if (signal.aborted) return;
      if (res.status === 404) {
        unlocked = false;
        heading.textContent = "Album not found";
        document.title = "Album not found";
        status.textContent = "This link does not match an album.";
        stage.replaceChildren();
        return;
      }
      if (!res.ok) {
        const data = (await res.json().catch(() => null)) as { error?: string } | null;
        status.textContent = data?.error ?? "Could not load this album.";
        return;
      }
      const album = (await res.json()) as GuestAlbum;
      status.textContent = "";
      if (album.locked) renderLocked(album.title);
      else renderPhotos(album.title, album.photos ?? []);
    } catch {
      if (!signal.aborted) status.textContent = "Could not load this album.";
    }
  }

  async function unlock(pin: string): Promise<void> {
    status.textContent = "Checking PIN…";
    try {
      const res = await galleryFetch(`/api/gallery/${token}/unlock`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ pin }),
        signal,
      });
      if (signal.aborted) return;
      if (res.status === 401) {
        status.textContent = "Incorrect PIN.";
        return;
      }
      if (res.status === 429) {
        status.textContent = "Too many attempts. Wait a few minutes and try again.";
        return;
      }
      if (!res.ok) {
        const data = (await res.json().catch(() => null)) as { error?: string } | null;
        status.textContent = data?.error ?? "Could not unlock this album.";
        return;
      }
      await load();
    } catch {
      if (!signal.aborted) status.textContent = "Could not unlock this album.";
    }
  }

  const viewTimer = window.setInterval(() => {
    if (document.visibilityState === "visible") void refreshPhotos();
  }, viewRefreshMs);
  document.addEventListener(
    "visibilitychange",
    () => {
      if (document.visibilityState === "visible") void refreshPhotos();
    },
    { signal }
  );

  void load();

  return () => {
    window.clearInterval(viewTimer);
    if (viewer.open) viewer.close();
    ac.abort();
  };
}
