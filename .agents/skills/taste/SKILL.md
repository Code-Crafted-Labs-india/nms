---
name: taste
description: "Design and refine NMS web UI with a clear visual hierarchy, responsive layouts, and careful interaction details. Use for frontend changes, UI fixes, and visual polish in this project."
---

# Taste for NMS UI

Use this skill for UI work in the NMS React application. Preserve the user's requested behavior and the existing product identity while making the interface easier to scan, operate, and trust.

## Project conventions

- Inspect the relevant view and shared styles before changing layout. The main app is in `ui/src/App.tsx`; API types and calls are in `ui/src/api.ts`; global styling is in `ui/src/index.css` and `ui/src/App.css`.
- Keep the established dark operations-console palette: deep green-black surfaces, muted slate text, lime brand accents, and restrained green/amber/red status colors. Reuse Tailwind utilities and existing components before adding a new design system or dependency.
- Keep pages information-dense but structured. Give each panel one clear purpose, use consistent spacing and labels, and distinguish primary actions from secondary actions.

## Layout and interaction

- Prefer Grid and Flexbox with shrink-safe children (`min-w-0`, `min-h-0`, `minmax(0, 1fr)`) over absolute positioning for content layout. Keep fixed navigation, scrollable navigation content, and account controls in separate layout regions so they cannot overlap at short viewport heights.
- Check wide and narrow layouts. Let tables and command previews scroll within their own containers; avoid forcing the whole page wider than the viewport. Use responsive stacking when columns no longer have comfortable reading width.
- Make controls understandable without relying on color alone. Provide visible labels, keyboard focus states, disabled/loading feedback, and useful empty and error states. Use semantic buttons and headings; give icon-only controls accessible names.
- For consequential network actions, make the target and intended change clear before submission, show validation and outcome states, and preserve existing authorization and audit behavior. Do not make a UI-only control appear to execute a device change if the backend does not support it.

## Finish

- Build on the existing component and API patterns. Keep changes scoped to the requested UI behavior and avoid adding packages for styling details.
- Run the UI production build (`npm --prefix ui run build`) after UI changes. If the local app is available, inspect the affected view at the rendered viewport and fix visible clipping or overlap before finishing.
