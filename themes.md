# DeepBlue Theme Guide

Reference this file whenever making frontend coding changes.

## Color Palette

- `#0B1F33` deep navy for headers and structure
- `#0F5E7A` ocean blue for primary actions
- `#3DB6C6` aqua for highlights
- `#D94B4B` high-risk alerts
- `#E9A93B` warning alerts
- `#2E9E6F` safe/resolved alerts

## Dark Mode Hierarchy

- `Hierarchy 1` `#0B1220`
  Use for the farthest-back application frame and deepest background surfaces.
- `Hierarchy 2` `#162131`
  Use for primary cards and major containers like the header area, map card, upload card, and alerts card.
- `Hierarchy 3` `#223247`
  Use for nested surfaces like stat tiles, alert rows, upload dropzone, and form fields.
- `Hierarchy 4` `#30465F`
  Use for the lightest elevated blue-gray surfaces such as focused controls, inline info strips, and subtle emphasis layers.

## Border And Text

- `#3B5167` for dark-mode borders and dividers
- `#E4EDF5` for primary text
- `#9DB0C2` for secondary text
- `#6FD5E2` for bright aqua highlights on dark surfaces

## Typography

- `IBM Plex Sans` for the UI
- `IBM Plex Mono` only for timestamps, metrics, and confidence values

## Usage Notes

- Keep the overall look calm, civic, data-oriented, and distinctly dark.
- Reserve alert colors for safety state and severity, not general decoration.
- Use the four hierarchy layers consistently so nested UI pieces always appear lighter than the surface behind them.
