const GALLERY_PATH = /^\/gallery\/([a-fA-F0-9]{32})$/;

/** Share-link token from a pathname, or null when the path is not an album. */
export function galleryTokenFromPath(pathname: string): string | null {
  const path =
    pathname.length > 1 ? pathname.replace(/\/+$/, "") : pathname || "/";
  const match = GALLERY_PATH.exec(path);
  return match?.[1]?.toLowerCase() ?? null;
}
