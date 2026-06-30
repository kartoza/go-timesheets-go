# Run `nix run .#fetch-fonts` to populate this directory with DejaVuSans.ttf.
# The file is required for the GUI to build (//go:embed in internal/gui/theme.go).
# Without it, the theme falls back to the Fyne default font, which lacks
# coverage for many BMP Unicode symbols we use as iconography (☰ ▥ ◉ ⚙ ⌂ ✦ …).
