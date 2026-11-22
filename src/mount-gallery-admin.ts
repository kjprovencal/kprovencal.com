import { publicApiUrl } from "./api-base";
import {
  decideResize,
  fitWithin,
  MAX_WEB_BYTES,
  uploadContentType,
  WEB_JPEG_QUALITY,
} from "./gallery-image";

type AlbumSummary = {
  id: string;
  title: string;
  share_path: string;
  created_at: string;
  photo_count: number;
};

type PhotoRow = {
  id: string;
  filename: string;
  status: string;
  width: number;
  height: number;
  web_url?: string;
};

type AlbumDetail = AlbumSummary & { photos?: PhotoRow[] };

type UploadSlot = {
  photo_id: string;
  original_url: string;
  original_content_type: string;
  web_url: string;
  web_content_type: string;
};

const UPLOAD_BATCH = 20;

function adminFetch(path: string, init?: RequestInit): Promise<Response> {
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

async function errorMessage(res: Response): Promise<string> {
  const data = (await res.json().catch(() => null)) as { error?: string } | null;
  return data?.error ?? `Request failed (HTTP ${res.status}).`;
}

function shareUrl(sharePath: string): string {
  return `${window.location.origin}${sharePath}`;
}

async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return false;
  }
}

function putWithProgress(
  url: string,
  body: Blob,
  contentType: string,
  onProgress: (ratio: number) => void,
  signal: AbortSignal
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    const fail = (err: Error) => {
      signal.removeEventListener("abort", onAbort);
      reject(err);
    };
    const onAbort = () => {
      xhr.abort();
      fail(new Error("cancelled"));
    };
    signal.addEventListener("abort", onAbort, { once: true });
    xhr.open("PUT", url);
    xhr.setRequestHeader("Content-Type", contentType);
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable && event.total > 0) {
        onProgress(event.loaded / event.total);
      }
    };
    xhr.onload = () => {
      signal.removeEventListener("abort", onAbort);
      if (xhr.status >= 200 && xhr.status < 300) resolve();
      else fail(new Error(`Upload failed (${xhr.status}).`));
    };
    xhr.onerror = () => {
      fail(
        new Error(
          "Upload was blocked. The Space needs a CORS rule that allows PUT from this site."
        )
      );
    };
    xhr.send(body);
  });
}

async function renderWebJpeg(
  file: File,
  maxEdge: number
): Promise<{ blob: Blob; width: number; height: number }> {
  let bitmap: ImageBitmap;
  try {
    bitmap = await createImageBitmap(file, { imageOrientation: "from-image" });
  } catch {
    throw new Error("Could not read this photo. Export a JPEG and try again.");
  }
  try {
    const fit = fitWithin(bitmap.width, bitmap.height, maxEdge);
    if (fit.width < 1 || fit.height < 1) {
      throw new Error("Could not read this photo. Export a JPEG and try again.");
    }
    const canvas = document.createElement("canvas");
    canvas.width = fit.width;
    canvas.height = fit.height;
    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error("Could not prepare a web-sized copy.");
    ctx.drawImage(bitmap, 0, 0, fit.width, fit.height);
    const blob = await new Promise<Blob | null>((resolve) => {
      canvas.toBlob((result) => resolve(result), "image/jpeg", WEB_JPEG_QUALITY);
    });
    if (!blob) throw new Error("Could not prepare a web-sized copy.");
    return { blob, width: bitmap.width, height: bitmap.height };
  } finally {
    bitmap.close();
  }
}

export function mountGalleryAdmin(
  panel: HTMLElement,
  signal: AbortSignal,
  onUnauthorized: () => void
): { reload: () => void } {
  panel.classList.add("gallery-admin");
  const status = el("p", "gallery-admin__status");
  status.setAttribute("aria-live", "polite");
  const view = el("div", "gallery-admin__view");
  panel.replaceChildren(status, view);

  let busy = false;
  let mode: "list" | "detail" = "list";
  let detailId = "";

  function setStatus(text: string): void {
    status.textContent = text;
  }

  async function guard(res: Response): Promise<boolean> {
    if (res.status === 401) {
      onUnauthorized();
      return false;
    }
    if (!res.ok) {
      setStatus(await errorMessage(res));
      return false;
    }
    return true;
  }

  function field(labelText: string, input: HTMLInputElement): HTMLLabelElement {
    const label = el("label", "gallery-field");
    const span = el("span", "gallery-field__label");
    span.textContent = labelText;
    label.append(span, input);
    return label;
  }

  function textInput(type: "text" | "password", autocomplete: string): HTMLInputElement {
    const input = el("input", "gallery-field__input");
    input.type = type;
    input.required = true;
    input.setAttribute("autocomplete", autocomplete);
    return input;
  }

  async function showList(): Promise<void> {
    mode = "list";
    detailId = "";
    view.replaceChildren();
    const heading = el("h2", "gallery-admin__title");
    heading.textContent = "Albums";
    const hint = el("p", "gallery-admin__hint");
    hint.textContent =
      "Create an album, then share its link. Guests need the PIN to view and download photos.";

    const form = el("form", "gallery-form");
    const titleInput = textInput("text", "off");
    const pinInput = textInput("password", "new-password");
    const confirmInput = textInput("password", "new-password");
    pinInput.minLength = 4;
    confirmInput.minLength = 4;
    const submit = el("button", "gallery-button gallery-button--primary");
    submit.type = "submit";
    submit.textContent = "Create album";
    form.append(
      field("Title", titleInput),
      field("PIN", pinInput),
      field("Confirm PIN", confirmInput),
      submit
    );
    form.addEventListener(
      "submit",
      (event) => {
        event.preventDefault();
        void createAlbum(titleInput.value, pinInput.value, confirmInput.value);
      },
      { signal }
    );

    const list = el("div", "gallery-album-list");
    list.textContent = "Loading albums…";
    view.append(heading, hint, form, list);

    const res = await adminFetch("/admin/albums", { signal });
    if (signal.aborted || mode !== "list") return;
    if (!(await guard(res))) {
      list.textContent = "";
      return;
    }
    const albums = (await res.json()) as AlbumSummary[];
    list.textContent = "";
    if (!Array.isArray(albums) || albums.length === 0) {
      const empty = el("p", "gallery-admin__empty");
      empty.textContent = "No albums yet.";
      list.append(empty);
      return;
    }
    for (const album of albums) {
      list.append(albumRow(album));
    }
  }

  function albumRow(album: AlbumSummary): HTMLElement {
    const row = el("article", "gallery-album");
    const title = el("h3", "gallery-album__title");
    title.textContent = album.title;
    const meta = el("p", "gallery-album__meta");
    const count = album.photo_count;
    meta.textContent = `${count} photo${count === 1 ? "" : "s"}`;
    const actions = el("div", "gallery-actions");
    const open = el("button", "gallery-button gallery-button--primary");
    open.type = "button";
    open.textContent = "Open";
    open.addEventListener("click", () => void openAlbum(album.id), { signal });
    const copy = el("button", "gallery-button");
    copy.type = "button";
    copy.textContent = "Copy link";
    copy.addEventListener(
      "click",
      () => void copyLink(shareUrl(album.share_path)),
      { signal }
    );
    const remove = el("button", "gallery-button gallery-button--danger");
    remove.type = "button";
    remove.textContent = "Delete";
    remove.addEventListener("click", () => void deleteAlbum(album), { signal });
    actions.append(open, copy, remove);
    row.append(title, meta, actions);
    return row;
  }

  async function copyLink(url: string): Promise<void> {
    const copied = await copyText(url);
    setStatus(copied ? "Link copied." : url);
  }

  async function createAlbum(title: string, pin: string, confirm: string): Promise<void> {
    if (pin !== confirm) {
      setStatus("PIN entries do not match.");
      return;
    }
    setStatus("Creating album…");
    const res = await adminFetch("/admin/albums", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ title: title.trim(), pin }),
      signal,
    });
    if (signal.aborted) return;
    if (!(await guard(res))) return;
    const created = (await res.json()) as AlbumSummary;
    setStatus("Album created.");
    await openAlbum(created.id);
  }

  async function deleteAlbum(album: AlbumSummary): Promise<void> {
    if (!window.confirm(`Delete “${album.title}” and its photos?`)) return;
    setStatus("Deleting album…");
    const res = await adminFetch(`/admin/albums/${encodeURIComponent(album.id)}`, {
      method: "DELETE",
      signal,
    });
    if (signal.aborted) return;
    if (!(await guard(res))) return;
    setStatus("Album deleted.");
    await showList();
  }

  async function openAlbum(id: string): Promise<void> {
    mode = "detail";
    detailId = id;
    view.replaceChildren();
    const loading = el("p", "gallery-admin__hint");
    loading.textContent = "Loading album…";
    view.append(loading);

    const res = await adminFetch(`/admin/albums/${encodeURIComponent(id)}`, { signal });
    if (signal.aborted || detailId !== id) return;
    if (!(await guard(res))) return;
    const album = (await res.json()) as AlbumDetail;
    renderDetail(album);
  }

  function renderDetail(album: AlbumDetail): void {
    view.replaceChildren();
    const back = el("button", "gallery-button");
    back.type = "button";
    back.textContent = "All albums";
    back.addEventListener("click", () => void showList(), { signal });

    const heading = el("h2", "gallery-admin__title");
    heading.textContent = album.title;

    const link = el("p", "gallery-share");
    const url = shareUrl(album.share_path);
    const linkText = el("span", "gallery-share__url");
    linkText.textContent = url;
    const copy = el("button", "gallery-button");
    copy.type = "button";
    copy.textContent = "Copy link";
    copy.addEventListener("click", () => void copyLink(url), { signal });
    link.append(linkText, copy);

    const titleForm = el("form", "gallery-form gallery-form--inline");
    const titleInput = textInput("text", "off");
    titleInput.required = true;
    titleInput.value = album.title;
    const saveTitle = el("button", "gallery-button");
    saveTitle.type = "submit";
    saveTitle.textContent = "Rename";
    titleForm.append(field("Title", titleInput), saveTitle);
    titleForm.addEventListener(
      "submit",
      (event) => {
        event.preventDefault();
        void patchAlbum(album.id, { title: titleInput.value.trim() });
      },
      { signal }
    );

    const pinForm = el("form", "gallery-form gallery-form--inline");
    const pinInput = textInput("password", "new-password");
    const confirmInput = textInput("password", "new-password");
    pinInput.minLength = 4;
    confirmInput.minLength = 4;
    const savePin = el("button", "gallery-button");
    savePin.type = "submit";
    savePin.textContent = "Change PIN";
    pinForm.append(field("New PIN", pinInput), field("Confirm PIN", confirmInput), savePin);
    pinForm.addEventListener(
      "submit",
      (event) => {
        event.preventDefault();
        if (pinInput.value !== confirmInput.value) {
          setStatus("PIN entries do not match.");
          return;
        }
        void patchAlbum(album.id, { pin: pinInput.value });
      },
      { signal }
    );

    const fileInput = el("input", "gallery-file-input");
    fileInput.type = "file";
    fileInput.accept = "image/*";
    fileInput.multiple = true;
    const fileLabel = el("label", "gallery-file-label");
    const fileLabelText = el("span");
    fileLabelText.textContent = "Add photos";
    fileLabel.append(fileLabelText, fileInput);
    fileInput.addEventListener(
      "change",
      () => {
        const files = [...(fileInput.files ?? [])];
        fileInput.value = "";
        if (files.length > 0) void uploadFiles(album.id, files);
      },
      { signal }
    );

    const log = el("ul", "gallery-upload-log");
    const grid = el("ul", "gallery-grid");
    fillGrid(grid, album.id, album.photos ?? []);
    view.append(back, heading, link, titleForm, pinForm, fileLabel, log, grid);
  }

  function fillGrid(grid: HTMLElement, albumId: string, photos: PhotoRow[]): void {
    grid.replaceChildren();
    if (photos.length === 0) {
      const empty = el("li", "gallery-admin__empty");
      empty.textContent = "No photos yet.";
      grid.append(empty);
      return;
    }
    for (const photo of photos) {
      grid.append(photoCard(albumId, photo));
    }
  }

  function photoCard(albumId: string, photo: PhotoRow): HTMLLIElement {
    const card = el("li", "gallery-card");
    if (photo.web_url && photo.status === "ready") {
      const img = el("img", "gallery-card__img");
      img.src = photo.web_url;
      img.alt = photo.filename;
      card.append(img);
    } else {
      const pending = el("p", "gallery-card__pending");
      pending.textContent = photo.status === "pending" ? "Upload unfinished" : "No preview";
      card.append(pending);
    }
    const name = el("p", "gallery-card__name");
    name.textContent = photo.filename;
    const remove = el("button", "gallery-button gallery-button--danger");
    remove.type = "button";
    remove.textContent = "Remove";
    remove.addEventListener(
      "click",
      () => void deletePhoto(albumId, photo),
      { signal }
    );
    card.append(name, remove);
    return card;
  }

  async function patchAlbum(
    id: string,
    body: { title?: string; pin?: string }
  ): Promise<void> {
    setStatus("Saving…");
    const res = await adminFetch(`/admin/albums/${encodeURIComponent(id)}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
      signal,
    });
    if (signal.aborted) return;
    if (!(await guard(res))) return;
    setStatus(body.pin ? "PIN updated." : "Album renamed.");
    await openAlbum(id);
  }

  async function deletePhoto(albumId: string, photo: PhotoRow): Promise<void> {
    if (!window.confirm(`Remove “${photo.filename}”?`)) return;
    setStatus("Removing photo…");
    const res = await adminFetch(
      `/admin/albums/${encodeURIComponent(albumId)}/photos/${encodeURIComponent(photo.id)}`,
      { method: "DELETE", signal }
    );
    if (signal.aborted) return;
    if (!(await guard(res))) return;
    setStatus("Photo removed.");
    await openAlbum(albumId);
  }

  function uploadRow(name: string): { item: HTMLLIElement; bar: HTMLElement; note: HTMLElement } {
    const item = el("li", "gallery-upload");
    const label = el("p", "gallery-upload__name");
    label.textContent = name;
    const track = el("div", "gallery-progress");
    const bar = el("span");
    track.append(bar);
    const note = el("p", "gallery-upload__note");
    note.textContent = "Waiting…";
    item.append(label, track, note);
    return { item, bar, note };
  }

  async function uploadFiles(albumId: string, files: File[]): Promise<void> {
    busy = true;
    const log = view.querySelector(".gallery-upload-log");
    if (!(log instanceof HTMLElement)) {
      busy = false;
      return;
    }
    log.replaceChildren();
    setStatus(`Uploading ${files.length} photo${files.length === 1 ? "" : "s"}…`);
    try {
      for (let offset = 0; offset < files.length; offset += UPLOAD_BATCH) {
        const chunk = files.slice(offset, offset + UPLOAD_BATCH);
        const ready: {
          file: File;
          row: ReturnType<typeof uploadRow>;
          web: { blob: Blob; width: number; height: number };
        }[] = [];
        for (const file of chunk) {
          const row = uploadRow(file.name);
          log.append(row.item);
          const decision = decideResize(file);
          switch (decision.kind) {
            case "skip":
              row.note.textContent = decision.reason;
              row.item.classList.add("gallery-upload--error");
              break;
            case "resize":
              row.note.textContent = "Preparing web size…";
              try {
                const web = await renderWebJpeg(file, decision.maxEdge);
                if (web.blob.size > MAX_WEB_BYTES) {
                  row.note.textContent = "The web-sized copy is larger than 12 MB.";
                  row.item.classList.add("gallery-upload--error");
                  break;
                }
                ready.push({ file, row, web });
              } catch (err) {
                row.item.classList.add("gallery-upload--error");
                row.note.textContent =
                  err instanceof Error ? err.message : "Could not prepare this photo.";
              }
              break;
            default: {
              const _never: never = decision;
              void _never;
            }
          }
        }
        if (ready.length === 0) continue;
        const res = await adminFetch(`/admin/albums/${encodeURIComponent(albumId)}/uploads`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            files: ready.map(({ file, web }) => ({
              name: file.name,
              type: uploadContentType(file.type, file.name),
              size: file.size,
              web_size: web.blob.size,
            })),
          }),
          signal,
        });
        if (signal.aborted) return;
        if (!(await guard(res))) return;
        const body = (await res.json()) as { uploads?: UploadSlot[] };
        const slots = body.uploads ?? [];
        for (let i = 0; i < ready.length; i++) {
          const slot = slots[i];
          const item = ready[i];
          if (!slot || !item) continue;
          await uploadOne(albumId, item.file, item.row, item.web, slot);
        }
      }
      const failed = log.querySelector(".gallery-upload--error");
      setStatus(failed ? "Some photos could not be uploaded." : "Upload finished.");
    } catch (err) {
      if (!signal.aborted) {
        setStatus(err instanceof Error ? err.message : "Upload failed.");
      }
    } finally {
      busy = false;
      if (!signal.aborted && detailId === albumId) await refreshGrid(albumId);
    }
  }

  async function refreshGrid(albumId: string): Promise<void> {
    const res = await adminFetch(`/admin/albums/${encodeURIComponent(albumId)}`, { signal });
    if (signal.aborted || !res.ok) return;
    const album = (await res.json()) as AlbumDetail;
    const grid = view.querySelector(".gallery-grid");
    if (!(grid instanceof HTMLElement)) return;
    fillGrid(grid, albumId, album.photos ?? []);
  }

  async function uploadOne(
    albumId: string,
    file: File,
    row: ReturnType<typeof uploadRow>,
    web: { blob: Blob; width: number; height: number },
    slot: UploadSlot
  ): Promise<void> {
    const setBar = (ratio: number) => {
      row.bar.style.width = `${Math.round(Math.min(1, Math.max(0, ratio)) * 100)}%`;
    };
    try {
      row.note.textContent = "Uploading original…";
      await putWithProgress(slot.original_url, file, slot.original_content_type, (ratio) => {
        setBar(ratio * 0.8);
      }, signal);
      row.note.textContent = "Uploading web size…";
      await putWithProgress(slot.web_url, web.blob, slot.web_content_type, (ratio) => {
        setBar(0.8 + ratio * 0.15);
      }, signal);
      const res = await adminFetch(
        `/admin/albums/${encodeURIComponent(albumId)}/photos/${encodeURIComponent(slot.photo_id)}/complete`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ width: web.width, height: web.height }),
          signal,
        }
      );
      if (!res.ok) throw new Error(await errorMessage(res));
      setBar(1);
      row.note.textContent = "Done";
    } catch (err) {
      if (signal.aborted) return;
      row.item.classList.add("gallery-upload--error");
      row.note.textContent = err instanceof Error ? err.message : "Upload failed.";
      await adminFetch(
        `/admin/albums/${encodeURIComponent(albumId)}/photos/${encodeURIComponent(slot.photo_id)}`,
        { method: "DELETE", signal }
      ).catch(() => undefined);
    }
  }

  return {
    reload() {
      if (busy || signal.aborted) return;
      if (mode === "detail" && detailId) void openAlbum(detailId);
      else void showList();
    },
  };
}
