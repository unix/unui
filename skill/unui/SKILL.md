---
name: unui
description: Use the unUI CLI to retrieve evidence-backed design guidance and apply it to local interface work. Use when the user asks to design, build, restyle, or improve a web page, application screen, or UI component with unUI evidence.
---

# unUI design evidence

Use unUI design evidence for the user's current interface task.

## Workflow

1. Read the workspace instructions and inspect the relevant local implementation before requesting evidence.
2. Understand the requested page or component, product context, visual tone, density, interaction requirements, responsive behavior, and important framework or component constraints.
3. Make one focused request:

   ```sh
   unui ask "<the UI task and its constraints>" --json
   ```

   Each request consumes account usage. Do not make exploratory or duplicate requests.

4. Parse stdout as one JSON document and check `ok` before reading `data`.
5. If the error code is `NOT_LOGGED_IN`, `AUTH_REQUIRED`, or `REGISTRY_AUTH_REQUIRED`:

   - Run `unui auth login`.
   - Let the user approve the browser authorization. Do not click approval controls for them.
   - Retry the same `unui ask ... --json` request once.

6. For any other error, report its code, message, and recovery hint. Do not bypass access limits or retry indefinitely.
7. Apply `data.rules` as usage boundaries and `data.references[].styleSignals` as the primary evidence. Treat style briefs, query details, and metrics as supporting context.

## Boundaries

- Treat references as evidence, not templates.
- Do not reconstruct source HTML, copy a reference, or preserve its exact element order or copy.
- Follow the local project's architecture, components, and conventions.
- Apply the evidence to hierarchy, layout, spacing, typography, surfaces, color, interaction, and responsive behavior where relevant.
- Validate the result according to the project's instructions.
- Never request, print, log, or save credentials or secret values.

## Finish

Summarize the Evidence Pack request, files changed or reviewed, validation performed, and remaining risks.
