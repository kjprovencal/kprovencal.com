import { marked } from "marked";

type EmojiToken = {
  type: "emoji";
  raw: string;
  shortcode: string;
  emoji: string;
};

type GenericToken = {
  type: string;
  raw: string;
};

const EMOJI_MAP: Record<string, string> = {
  smile: "😄",
  grin: "😁",
  joy: "😂",
  heart: "❤️",
  star: "⭐",
  sparkles: "✨",
  tada: "🎉",
  fire: "🔥",
  rocket: "🚀",
  check: "✅",
  wave: "👋",
  wedding: "💍",
  couple: "👰🤵",
  party: "🥳",
  camera: "📷",
  link: "🔗",
  letter: "📧",
  menu: "🍽️",
};

let registered = false;

export function registerEmojiExtension(): void {
  if (registered) return;
  registered = true;

  marked.use({
    extensions: [
      {
        name: "emoji",
        level: "inline",
        start(src: string): number | undefined {
          const idx = src.indexOf(":");
          return idx >= 0 ? idx : undefined;
        },
        tokenizer(src: string): EmojiToken | undefined {
          const match = src.match(/^:([a-z0-9_+-]+):/i);
          if (!match) return undefined;
          const shortcode = match[1].toLowerCase();
          const emoji = EMOJI_MAP[shortcode];
          if (!emoji) return undefined;
          return {
            type: "emoji",
            raw: match[0],
            shortcode,
            emoji,
          };
        },
        renderer(token: GenericToken): string {
          if (token.type !== "emoji") return token.raw;
          const t = token as EmojiToken;
          return `<span class="md-emoji" role="img" aria-label="${t.shortcode}">${t.emoji}</span>`;
        },
      },
    ],
  });
}
