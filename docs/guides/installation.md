# Installing Inkwell on a Raspberry Pi

This guide walks you from a fresh Raspberry Pi to a running Inkwell
service that drives your e-paper panel. The supported flow is to
**download a published release binary, copy it to the Pi, and run it
as a systemd service** — no Go toolchain, no compiler, no source
checkout required on either machine.

> **About the SPI backend.** Inkwell currently ships two production
> backends: `preview` (HTTP + SSE preview that you can hit from your
> network) and `image` (PNG snapshots on disk). The `spi` backend is
> wired up in code but its periph.io `host.Init()` bootstrap is still
> stubbed — choosing `backend: spi` will error at startup until that
> integration lands (see [Known gaps][gaps]). The instructions below
> give you a working end-to-end install with the `preview` backend
> today; once the SPI integration lands, switching is a one-line
> config change. Beads tracks the SPI integration work via
> `bd ready`.

[gaps]: ../tech-specs/06-go-implementation-guide.md#known-gaps

## Prerequisites

On the Pi:

- Raspberry Pi OS (64-bit recommended; 32-bit is also supported).
- SPI enabled via `sudo raspi-config → Interfacing Options → SPI → Yes`
  (then reboot). Verify with `ls /dev/spi*` — you should see
  `/dev/spidev0.0`.
- The user that will run Inkwell is in the `gpio` and `spi` groups so
  it can open `/dev/gpiochip0` and `/dev/spidev0.0` without root:

  ```bash
  sudo usermod -aG gpio,spi pi
  # Log out and back in for group membership to take effect.
  ```

You don't need Go, Python, or any build tools on the Pi.

## 1. Pick Your Architecture

Check what your Pi is running:

```bash
uname -m
```

<!-- markdownlint-disable MD013 -->
| `uname -m` says | Download asset suffix | Typical hardware |
|-----------------|-----------------------|------------------|
| `aarch64`       | `linux-arm64`         | 64-bit Raspberry Pi OS on Pi Zero 2 W, 3, 4, 5 |
| `armv7l`        | `linux-armv7`         | 32-bit Raspberry Pi OS on Pi 2, 3, 4, Zero 2 W |
| `armv6l`        | `linux-armv6`         | 32-bit Raspberry Pi OS on the original Pi Zero / Pi 1 |
<!-- markdownlint-enable MD013 -->

If you're not sure, install the 64-bit Raspberry Pi OS image and use
`linux-arm64` — it's the actively-tested target.

## 2. Download the Latest Release

On the Pi (or on your workstation if you'd rather `scp` the result):

```bash
# Pick your arch from the table above
ARCH=linux-arm64

# Always-latest URL — GitHub redirects to the most recent published
# release. The asset name does not include the version, so this URL
# stays stable across releases.
curl -L -o inkwell.tar.gz \
  "https://github.com/grantlucas/inkwell/releases/latest/download/inkwell-${ARCH}.tar.gz"

tar -xzf inkwell.tar.gz
chmod +x inkwell
./inkwell --help 2>&1 | head -1 || echo "binary is in place"
```

To pin to a specific tagged version instead of `latest`:

```bash
VERSION=v0.5.0
curl -L -o inkwell.tar.gz \
  "https://github.com/grantlucas/inkwell/releases/download/${VERSION}/inkwell-${ARCH}.tar.gz"
```

Find published versions on the
[releases page](https://github.com/grantlucas/inkwell/releases).

## 3. Write a Config File

Inkwell reads its config from `inkwell.yaml` next to the binary (or
from a path passed as the first CLI argument). Start from the bundled
example, which lives in the release archive:

```bash
# inkwell.example.yaml is included in the tarball
cp inkwell.example.yaml inkwell.yaml
```

Edit `inkwell.yaml` to suit your setup. The example wires up a full
dashboard (date header, clock, separator, weekly calendar + weather).
The most important fields to review:

```yaml
display: waveshare_7in5_v2     # Profile name — leave as-is for the V2
backend: preview               # Use 'preview' today; 'spi' when SPI lands
preview:
  port: 8080                   # HTTP preview port

dashboard:
  screens:
    - name: weekly
      widgets:
        - type: weekly-calendar
          bounds: [0, 52, 800, 480]
          config:
            feeds:
              - "https://your-calendar-host.example/calendar.ics"
            latitude: 40.7128
            longitude: -74.0060
            temp_unit: C
```

Replace the feed URL with a real iCal endpoint and set `latitude` /
`longitude` for your location. See
[`docs/guides/building-dashboards.md`](building-dashboards.md) for the
full dashboard / widget configuration model.

For a minimal smoke-test config that doesn't depend on any external
services:

```yaml
display: waveshare_7in5_v2
backend: preview
preview:
  port: 8080
dashboard:
  screens:
    - name: hello
      widgets:
        - type: date
          bounds: [0, 0, 800, 50]
          config: { format: "Monday, January 2" }
        - type: clock
          bounds: [700, 0, 800, 50]
          config: { format: "15:04", align: right }
```

## 4. First Run (Foreground)

```bash
./inkwell inkwell.yaml
```

If the config parses and the `preview` backend starts, you'll see the
process come up and serve on the port you configured. From another
machine on the same network:

```text
http://<pi-host>:8080/
```

You should see the rendered dashboard, with a radio toggle for
switching between the device view (post-dither, what the panel would
show) and the source view (pre-dither grayscale design). Every render
tick (default: 60 seconds) the SSE stream pushes a refresh and the
browser updates automatically.

Press Ctrl-C to stop the foreground process before moving on.

## 5. Install as a Systemd Service

For unattended operation, run Inkwell as a systemd unit. Move the
binary and config to stable paths, then create the unit file. Below
assumes the `pi` user owns the install — adjust `User=` / `Group=` /
paths to match your setup.

```bash
sudo install -m 0755 -o pi -g pi inkwell /usr/local/bin/inkwell
sudo install -d -o pi -g pi /etc/inkwell
sudo install -m 0644 -o pi -g pi inkwell.yaml /etc/inkwell/inkwell.yaml
```

Create `/etc/systemd/system/inkwell.service`:

```ini
[Unit]
Description=Inkwell e-paper dashboard
After=network.target

[Service]
Type=simple
User=pi
Group=pi
ExecStart=/usr/local/bin/inkwell /etc/inkwell/inkwell.yaml
Restart=on-failure
RestartSec=5s

# Once the SPI backend is enabled, allow the service user to access
# the SPI bus and GPIO chip device. The pi user already has these
# through the gpio/spi group membership configured above; these
# directives are belt-and-braces for hardened service users.
SupplementaryGroups=gpio spi

[Install]
WantedBy=multi-user.target
```

Enable and start it:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now inkwell.service
sudo systemctl status inkwell.service
```

Useful commands while iterating:

```bash
journalctl -u inkwell -f       # Tail logs
sudo systemctl restart inkwell # Apply config edits
sudo systemctl stop inkwell    # Stop the service
```

## 6. Verify on Hardware

With the service running:

1. **Web preview reachable?** Open `http://<pi-host>:8080/` from
   another machine on the same LAN. The page should render the
   dashboard at 2× scale.
2. **Logs healthy?** `journalctl -u inkwell -f` should show no
   repeated error messages, no crash loops.
3. **Updates flowing?** Watch the browser preview across at least
   one render-loop tick (default 60s) and confirm the SSE stream is
   pushing refreshes.

Once the SPI backend is enabled and you switch `backend: spi`, the
verification step extends to **looking at the panel itself** — the
first full refresh should land within a few seconds of service start,
and `journalctl -u inkwell -f` will surface any `EPD.Init` /
`EPD.Display` errors.

## 7. Updating

To move to a newer release, download the new tarball (step 2), drop
the new binary in place, and restart the service:

```bash
sudo install -m 0755 -o pi -g pi inkwell /usr/local/bin/inkwell
sudo systemctl restart inkwell
```

Config-only changes don't need a new binary — edit
`/etc/inkwell/inkwell.yaml` and restart the service.

## Troubleshooting

**`open inkwell.yaml: no such file or directory`** — Inkwell reads
`inkwell.yaml` from its working directory by default. Either copy a
config file next to the binary, or pass an absolute path as the first
CLI argument (the systemd unit above does this).

**Preview backend never serves traffic** — confirm the service user
can bind the port (no firewall, port not already in use). `ss -lntp`
on the Pi will show what's listening.

**`spi backend requires building with -tags hardware`** — your binary
was built without the SPI backend compiled in. Release binaries
should include it; if you see this with an official release, file an
issue.

**`real hardware init not available in test-only builds`** — the SPI
backend's real-hardware periph.io bootstrap is still stubbed. See
[Known gaps][gaps]; stay on the `preview` backend until that work
lands.

## Building from Source (Advanced)

If you need to run an unreleased commit or target an architecture
that the official releases don't cover, you can cross-compile from a
workstation that has Go 1.25+ installed:

```bash
git clone https://github.com/grantlucas/inkwell.git
cd inkwell
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -tags hardware -o inkwell ./cmd/inkwell
scp inkwell pi@inkwell.local:~/
```

Then continue from step 3 above. This path is intended for
development, not production installs — official releases are the
supported install method.

## Where to Go Next

- [`docs/guides/building-dashboards.md`](building-dashboards.md) —
  design custom screens, build new widgets, configure dashboards.
- [`docs/guides/hardware-grayscale.md`](hardware-grayscale.md) — what
  reads cleanly on the panel vs. what dithers to stipple.
- [`docs/tech-specs/`](../tech-specs/) — hardware overview, SPI
  command reference, Go architecture, and testing strategy.
