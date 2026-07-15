# Build Directory

This directory contains Wails build resources for both supported desktop platforms.

- `bin/`: local build output; never committed.
- `darwin/`: macOS plist and packaging resources.
- `windows/`: Windows manifest, metadata, icon and installer resources.
- `appicon.png`: shared high-resolution source icon.

Brand sources live in `assets/branding/`. Generated platform icons are copied here only when the packaging task explicitly updates them.
