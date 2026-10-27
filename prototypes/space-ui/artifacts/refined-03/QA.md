# Draft 03 — UI review observations

The prototype remains isolated, uses in-memory fixture data, and does not connect real WhatsApp accounts or send messages.

## Navigation and controls

- Header has no sidebar-collapse or appearance icon. The sole sidebar collapse control sits below Appearance, with Ctrl B retained.
- Sidebar has no scrolling or clipped profile/navigation at desktop heights 640, 662, 720, 900 and 1000; scrollHeight equals clientHeight. Measured at widths 390 through 1440. Responsive spacing fits the controls rather than merely hiding an overflowing scrollbar.
- Required content scrolling uses thin rounded warm scrollbars, without scrollbar arrow buttons. Light mode uses the corresponding neutral palette.
- No native select, date, time or color popups remain in the revised UI. Status, PJ, category, bot options, time, date and accent controls use custom surfaces. Keyboard navigation and Escape are supported for custom select menus.
- Task status changes create a custom snackbar with Undo. Undo restored the original task state. Opening a task still docks the inspector beside the board without a task modal.
- New-task creation correctly used the custom PJ/category/date pickers. Modal alerts are custom alertdialogs; no window.alert/confirm/prompt calls occurred.
- Division logos come from the HIMASIF profile assets. General settings accepts a local custom logo and restores the default; data stays in memory.

## WhatsApp settings

- Settings includes account connection, QR flow, number type, message label/style, group mapping, reminders/SOP, commands/stickers and delivery activity.
- Disconnect uses a custom confirmation. QR is explicitly an illustration; the simulated connect action does not authenticate a real account.
- Label/style preview changed correctly and Save clears the local dirty indicator.
- Custom command validation rejected protected !jobdesk, and accepted a new !ngopi command. !damn provides a sticker response editor and media picker.
- Default reminders remain H-2, H-1 and deadline day at 06.00 WIB; editable time worked. Follow-up H-4 and design-deadline semantics are presented separately with SOP exceptions requiring review.
- Humas maps to Humas + Medhum groups; Mediatek maps to its own group. A Mediatek manager viewing Medhum cannot edit the Humas bot connection or style.
- Role 45 can manage WhatsApp, with role/access management explicitly reserved for the original kadiv. A member has no bot management controls. These are prototype UI guards, not production authorization.
- Disconnecting the Humas bot did not alter the separate Mediatek bot state.
- Delivery logs distinguish accepted, queued and unknown states; retry requires confirmation and represents one attempt for the same event.

Raw observations: qa-observations.json. Screens are exported at 1440×1000 logical pixels, DPR 2. Initial verification used headless Chromium from the migrated HDD cache; the same frontend is served on port 5173 and embedded by the MyGo preview launcher. Windows runtime and the real Baileys/backend integration remain future implementation work.
