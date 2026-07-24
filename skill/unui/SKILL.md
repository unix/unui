---
name: unui
description: Use the unUI CLI to retrieve evidence-backed design guidance and apply it to local interface work. Use when the user asks to design, build, restyle, or improve a web page, application screen, or UI component with unUI evidence. Send a concise retrieval query instead of project contents, code, or the complete task brief.
---

# unUI design evidence

Use unUI design evidence for the user's current interface task.

## Workflow

1. Read the workspace instructions and understand the user's interface goal. Do not inspect or collect project contents for the purpose of constructing the unUI request.
2. Derive one compact retrieval query from the interface goal:

   - Use 6–20 high-information terms, normally 40–160 characters and never more than 256 characters.
   - Include the page or component type, its key composition or layout, the desired visual tone, and only the most important interaction or responsive constraint.
   - Prefer concrete design vocabulary over explanatory prose, business rationale, exhaustive requirements, or repeated synonyms.
   - Express desired characteristics positively. Keep negative constraints and unwanted pattern names in local context because retrieval treats every supplied term as a positive match signal.
   - Keep the user's complete task in local context. Do not send it to unUI.

3. Treat the query as evidence retrieval input, not as the implementation specification. After receiving evidence, combine it with the complete local task when implementing the interface.
4. Make one focused request:

   ```sh
   unui ask "<compact design evidence query>" --json
   ```

   Each request consumes account usage. Do not make exploratory or duplicate requests.

5. Parse stdout as one JSON document and check `ok` before reading `data`.
6. If the error code is `NOT_LOGGED_IN` or `AUTH_REQUIRED`:

   - Run `unui auth login`.
   - Let the user approve the browser authorization. Do not click approval controls for them.
   - Retry the same `unui ask ... --json` request once.

7. For any other error, report its code, message, and recovery hint. Do not bypass access limits or retry indefinitely.
8. Apply `data.rules` as usage boundaries and `data.references[].styleSignals` as the primary evidence. Treat style briefs, query details, and metrics as supporting context.
9. Inspect the relevant local implementation only as needed to apply the evidence. Keep that local material out of the unUI request.

## Boundaries

- Treat the text passed to `unui ask` as a newly authored compact retrieval query, not an extract, inventory, or summary of the workspace.
- Never send project contents to unUI. This includes source code, file contents, directory trees, routes, configuration, manifests, diffs, logs, schemas, API payloads, copied product data, or user data.
- Never send tokens, credentials, secrets, or other authentication material to unUI, whether directly or inside a larger prompt.
- Do not ask unUI to inspect or analyze a repository, file, implementation, or uploaded project material.
- Treat references as evidence, not templates.
- Do not reconstruct source HTML, copy a reference, or preserve its exact element order or copy.
- Follow the local project's architecture, components, and conventions.
- Apply the evidence to hierarchy, layout, spacing, typography, surfaces, color, interaction, and responsive behavior where relevant.
- Validate the result according to the project's instructions.
- Never request, print, log, or save credentials or secret values.

## Finish

Summarize the Evidence Pack request, files changed or reviewed, validation performed, and remaining risks.
