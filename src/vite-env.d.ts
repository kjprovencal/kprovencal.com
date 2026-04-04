/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_TURNSTILE_SITE_KEY?: string;
  readonly VITE_PUBLIC_API_URL?: string;
}

declare module "*.md?raw" {
  const content: string;
  export default content;
}

type TurnstileRenderOptions = Record<string, unknown> & {
  sitekey: string;
  theme?: string;
  callback?: (token: string) => void;
  "error-callback"?: () => void;
  "expired-callback"?: () => void;
};

interface TurnstileApi {
  render(
    container: string | HTMLElement,
    options: TurnstileRenderOptions
  ): string;
  getResponse(widgetId?: string): string | undefined;
  reset(widgetId?: string): void;
  remove(widgetId?: string): void;
}

declare global {
  interface Window {
    turnstile?: TurnstileApi;
  }
}

export {};
