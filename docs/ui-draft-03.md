# Space UI draft 03

User review requested a single collapse control below Appearance, no sidebar scrolling, real division marks with customization, styled content scrollbars and custom dropdown/alert/snackbar controls. WhatsApp settings were missing and are now explored in this draft.

The revised prototype has fixed navigation with spacing responsive to window height, no duplicate header controls, and custom selection/date/time/accent components. WhatsApp settings include connection and QR states, message label/style, account type, group mapping, reminders with SOP context, custom commands/stickers and delivery activity. Humas owns the shared Medhum bot route. Role 45 can manage its division bot while members cannot; role/access authority remains with the original kadiv.

Six interactive standalone artifacts and desktop screenshots are in prototypes/space-ui/artifacts/refined-03. See its QA.md and qa-observations.json for manual checks and practical limits. The gallery is http://localhost:5173/refined-gallery.html?draft=03.

This is disposable UI code with in-memory fixture data. It is not approval of a production UI or an implementation of real Baileys/backend behavior. Production remains native MyGo Go components. Previous A–E and draft 02 exports are preserved. User feedback on this iteration is still pending.

## Accepted review refinements — revision 03.1

The user accepted the overall layout and requested cleaner Settings/WhatsApp icons, a header notification dropdown instead of a modal, and logout in the profile ellipsis menu. These are now in the live preview and standalone HTML exports. Notification read state, task/agenda navigation, profile context for the current division/role, local logout/sign-in and keyboard dismissal were manually verified. Production logout must invalidate the real backend session; this prototype resets its in-memory fixture state and records the signed-out review state in the URL.
