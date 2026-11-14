# Configuration and privacy

Preferences use `os.UserConfigDir()/siftpane/config.json`: normally `~/Library/Application Support/siftpane/config.json` on macOS, `$XDG_CONFIG_HOME/siftpane/config.json` or `~/.config/siftpane/config.json` on Linux, and `%AppData%\siftpane\config.json` on Windows. Override with `--config PATH`. See [example configuration](../examples/config.json); do not copy placeholder IDs unchanged.

Inclusion and local-read maps are keyed by **workspace ID then conversation ID** so saved preferences cannot bleed between workspaces. The application still uses one workspace at a time. Writes use a private temporary file and rename; tokens and bodies never enter this file. There are no default application logs. A corrupt configuration is reported rather than overwritten. Use one Siftpane process per config file; concurrent writers are not coordinated.
