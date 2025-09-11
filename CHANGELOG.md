# Changelog

All notable changes to this project will be documented in this file.

## v0.1.1 — Replace Recharts with Nivo, UX polish

- Charts: migrated UI charts to `@nivo/*` for improved rendering and theming
  - Updated Codecs, Qualities, Usage, and Playback Methods charts
  - Consistent black/gold theme, refined tooltips, and grid styling
- Playback Methods: bars-only click behavior
  - Only the bars are clickable now (container no longer opens details)
  - Clicking a bar opens the detailed view with filters preselected
    - Direct → detailed view with only Direct selected (transcode-only OFF)
    - Video/Audio/Subtitle Transcode → corresponding filter selected (transcode-only ON)
- Legend polish: added left padding inside legend items on Playback Methods chart for better visual separation
- General: minor UI tweaks, code cleanup, and type-safety improvements

 
