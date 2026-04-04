import { marked } from "marked";
import { registerEmojiExtension } from "./marked-emoji";
import { registerFormTokenExtension } from "./marked-form-tokens";
import { registerTaggedTableExtension } from "./marked-tagged-table";

let configured = false;

/**
 * Registers GFM options and site extensions (emoji, form tokens, `@table`).
 * Safe to call once at app startup; subsequent calls are no-ops.
 */
export function configureMarked(): void {
  if (configured) return;
  configured = true;
  marked.setOptions({
    gfm: true,
    breaks: false,
  });
  registerEmojiExtension();
  registerFormTokenExtension();
  registerTaggedTableExtension();
}
