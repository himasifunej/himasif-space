# UI exploration verification

Scope: five disposable Space desktop UI alternatives; fixture data only. No production backend, WA, Sheets or Trello calls.

Verified in enowX Browser:

- A–E switchers update the URL, title and structural layout.
- All five render at a narrow 365px preview with document width equal to viewport width; board/calendar overflow stays inside its container.
- Desktop 1440×1000 Canvas captures of all five inspected for hierarchy, spacing and overflow; final captures have no horizontal-overflow warning.
- Task detail distinguishes design deadline 28 October from posting date 30 October.
- Ping opens an editable modal and produces explicitly simulated feedback.
- A sample task is added to in-memory jobdesk data.
- Notes expose three templates and editable meeting notes.
- Personal reminder sharing can be enabled and revoked.
- Offline state displays a persistent banner and explicitly says data is on the device, not synced to cloud.
- Ace can enter edge mode and be hidden; hidden dock is removed.
- Standalone artifact C opens with the correct default variant and embedded task detail.

Build:

- MyGo pinned to v0.7.1, using Go 1.27.3.
- Windows amd64 cross-compilation succeeds with CGO disabled.
- Actual Windows runtime, system notifications, tray and backend integration are not verified or implemented by this UI prototype.
- Browser and MyGo use the same embedded frontend. Screenshots and Canvas mockups show that frontend.

These checks validate a selectable design draft, not production task/permission/reminder behavior.
