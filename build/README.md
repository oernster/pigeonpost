# Build Directory

This is Wails scaffolding, kept close to its generated form so `wails build` finds what it expects.
Two corrections for this repository, because the generated text describes stock Wails rather than
PigeonPost:

- **`windows/installer/` (`project.nsi`, `wails_tools.nsh`) is unused.** Nothing in the repo
  references NSIS. PigeonPost's setup program is a bespoke Wails app under `installer/`, built by
  `build.ps1` into `dist-installer/PigeonPostSetup.exe`. See DEVELOPMENT-README.md.
- **`eml.png` and `windows/eml.ico`** are PigeonPost's own additions, the icon for the `.eml` file
  association; they are not part of the generated scaffold.

The build directory is used to house all the build files and assets for your application.

The structure is:

* bin - Output directory
* darwin - macOS specific files
* windows - Windows specific files

## Mac

The `darwin` directory holds files specific to Mac builds.
These may be customised and used as part of the build. To return these files to the default state, simply delete them
and
build with `wails build`.

The directory contains the following files:

- `Info.plist` - the main plist file used for Mac builds. It is used when building using `wails build`.
- `Info.dev.plist` - same as the main plist file but used when building using `wails dev`.

## Windows

The `windows` directory contains the manifest and rc files used when building with `wails build`.
These may be customised for your application. To return these files to the default state, simply delete them and
build with `wails build`.

- `icon.ico` - The icon used for the application. This is used when building using `wails build`. If you wish to
  use a different icon, simply replace this file with your own. If it is missing, a new `icon.ico` file
  will be created using the `appicon.png` file in the build directory.
- `installer/*` - The files used to create the Windows installer. These are used when building using `wails build`.
- `info.json` - Application details used for Windows builds. The data here will be used by the Windows installer,
  as well as the application itself (right click the exe -> properties -> details)
- `wails.exe.manifest` - The main application manifest file.