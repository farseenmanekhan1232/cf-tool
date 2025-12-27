# CF-Tool Login Helper Extension

Chrome extension that automatically transfers your Codeforces login session to cf-tool CLI.

## Installation

### From Source (Developer Mode)

1. Open Chrome and go to `chrome://extensions/`
2. Enable **Developer mode** (toggle in top right)
3. Click **Load unpacked**
4. Select the `extension` folder from cf-tool

### Icons (Optional)

The extension needs icon files. You can create simple icons or use any 16x16, 48x48, and 128x128 PNG images named:
- `icon16.png`
- `icon48.png`
- `icon128.png`

Or remove the `icons` section from `manifest.json` to use default icons.

## How It Works

1. When you run `cf config` → login, cf-tool opens Codeforces with a special URL parameter (`?cf_port=XXXXX`)
2. The extension detects this parameter
3. If you're logged in, it automatically sends your cookies to cf-tool
4. Login completes automatically!

## Privacy

- The extension only runs on codeforces.com
- It only communicates with localhost (your own computer)
- No data is ever sent to external servers
