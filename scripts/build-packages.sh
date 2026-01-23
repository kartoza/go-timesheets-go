#!/usr/bin/env bash
# Build packages for Kartoza Timesheet
# Usage: ./scripts/build-packages.sh [deb|rpm|all] [version]
#
# This script builds packages using nix to get the correct Go version,
# then uses Docker only for the packaging step.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_DIR"

# Get version from git or argument
VERSION="${2:-$(git describe --tags --always 2>/dev/null || echo "0.0.0")}"
VERSION="${VERSION#v}"  # Remove 'v' prefix if present

RELEASE_DIR="$PROJECT_DIR/release"
mkdir -p "$RELEASE_DIR/deb" "$RELEASE_DIR/rpm"

build_binary() {
    echo "Building binary for version $VERSION using nix..."

    # Always use nix develop to ensure all dependencies (GL, X11, etc.) are available
    nix develop --command bash -c "go build -ldflags='-s -w -X main.version=${VERSION}' -o '$RELEASE_DIR/kartoza-timesheet' ."

    echo "Binary built: $RELEASE_DIR/kartoza-timesheet"
    ls -lh "$RELEASE_DIR/kartoza-timesheet"
}

build_deb() {
    echo "Building Debian package for version $VERSION..."

    # First build the binary using nix (to get correct Go version)
    build_binary

    # Then create the deb package structure and package it
    PKG_DIR="$PROJECT_DIR/build/deb/kartoza-timesheet-${VERSION}"

    mkdir -p "${PKG_DIR}/usr/bin"
    mkdir -p "${PKG_DIR}/usr/share/applications"
    mkdir -p "${PKG_DIR}/DEBIAN"

    # Copy binary
    cp "$RELEASE_DIR/kartoza-timesheet" "${PKG_DIR}/usr/bin/"

    # Desktop file
    cat > "${PKG_DIR}/usr/share/applications/kartoza-timesheet.desktop" << 'DESKTOP'
[Desktop Entry]
Name=Kartoza Timesheet
Comment=Time tracking application
Exec=kartoza-timesheet
Terminal=false
Type=Application
Categories=Office;ProjectManagement;
DESKTOP

    # Control file
    cat > "${PKG_DIR}/DEBIAN/control" << CONTROL
Package: kartoza-timesheet
Version: ${VERSION}
Section: utils
Priority: optional
Architecture: amd64
Depends: libgl1, libx11-6, libxcursor1, libxrandr2, libxinerama1, libxi6
Maintainer: Kartoza <info@kartoza.com>
Description: Kartoza Timesheet Application
 A time tracking application with both TUI and GUI modes.
 Features include project/task tracking, favorites, POW mode,
 and integration with ERPNext timesheets.
CONTROL

    # Build deb using docker (for dpkg-deb)
    echo "Creating package..."
    docker run --rm -v "$PROJECT_DIR:/src" -w /src debian:bookworm-slim bash -c "
        dpkg-deb --build build/deb/kartoza-timesheet-${VERSION}
    "

    mv "${PKG_DIR}.deb" "$RELEASE_DIR/deb/kartoza-timesheet_${VERSION}_amd64.deb"
    rm -rf "$PROJECT_DIR/build/deb"

    echo "Done!"
    ls -lh "$RELEASE_DIR/deb/"*.deb
}

build_rpm() {
    echo "Building RPM package for version $VERSION..."

    # First build the binary using nix (to get correct Go version)
    build_binary

    # Create the RPM using docker with the pre-built binary
    docker run --rm -v "$PROJECT_DIR:/src" -w /src fedora:latest bash -c "
        set -e
        dnf install -y rpm-build >/dev/null 2>&1

        VERSION='$VERSION'

        # Setup RPM build dirs
        mkdir -p ~/rpmbuild/{BUILD,RPMS,SOURCES,SPECS,SRPMS}

        # Spec file with %install section that copies files
        cat > ~/rpmbuild/SPECS/kartoza-timesheet.spec << 'SPEC'
Name:           kartoza-timesheet
Version:        VERSION_PLACEHOLDER
Release:        1%{?dist}
Summary:        Kartoza Timesheet Application
License:        MIT
URL:            https://github.com/kartoza/go-timesheets-go
BuildArch:      x86_64

%description
A time tracking application with both TUI and GUI modes.
Features include project/task tracking, favorites, POW mode,
and integration with ERPNext timesheets.

%install
mkdir -p %{buildroot}/usr/bin
mkdir -p %{buildroot}/usr/share/applications
cp /src/release/kartoza-timesheet %{buildroot}/usr/bin/
cat > %{buildroot}/usr/share/applications/kartoza-timesheet.desktop << 'DESKTOP'
[Desktop Entry]
Name=Kartoza Timesheet
Comment=Time tracking application
Exec=kartoza-timesheet
Terminal=false
Type=Application
Categories=Office;ProjectManagement;
DESKTOP

%files
/usr/bin/kartoza-timesheet
/usr/share/applications/kartoza-timesheet.desktop
SPEC

        # Replace version placeholder
        sed -i \"s/VERSION_PLACEHOLDER/\${VERSION}/\" ~/rpmbuild/SPECS/kartoza-timesheet.spec

        echo 'Building package...'
        rpmbuild -bb ~/rpmbuild/SPECS/kartoza-timesheet.spec
        cp ~/rpmbuild/RPMS/x86_64/*.rpm /src/release/rpm/

        echo 'Done!'
        ls -lh /src/release/rpm/*.rpm
    "
}

case "${1:-all}" in
    deb)
        build_deb
        ;;
    rpm)
        build_rpm
        ;;
    all)
        build_deb
        build_rpm
        echo ""
        echo "All packages built in $RELEASE_DIR/"
        ;;
    *)
        echo "Usage: $0 [deb|rpm|all] [version]"
        exit 1
        ;;
esac
