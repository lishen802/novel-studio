import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./app/App";
import type { StudioClient } from "./lib/studio-client";

const root = document.getElementById("root");

if (!root) {
  throw new Error("root element is missing");
}
const rootElement = root;

async function renderApp() {
  let previewClient: StudioClient | undefined;
  if (import.meta.env.DEV && !window.go) {
    const { FakeStudioClient } = await import("./lib/fake-studio-client");
    const fake = new FakeStudioClient();
    fake.projectParent = "/本地作品";
    fake.projectToOpen = fake.openProjectResult.path;
    previewClient = fake;
  }

  createRoot(rootElement).render(
    <StrictMode>
      <App client={previewClient} />
    </StrictMode>,
  );
}

void renderApp();
