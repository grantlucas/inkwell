# Installing Inkwell on a Raspberry Pi

This guide walks you from a fresh Raspberry Pi to a running Inkwell
service that drives your e-paper panel. The development flow is to
**cross-compile on your workstation, copy the resulting binary to the
Pi, and run it as a systemd service** — no Go toolchain on the Pi
itself.

> **About the SPI backend.** Inkwell currently ships two production
> backends: `preview` (HTTP + SSE preview that you can hit from your
> network) and `image` (PNG snapshots on disk). The `spi` backend is
> wired up in code but its periph.io `host.Init()` bootstrap is still
> stubbed — building Inkwell with `-tags hardware` and choosing
> `backend: spi` will error at startup until that integration lands
> (see [Known gaps][gaps]). The instructions below give you a working
> end-to-end install with the `preview` backend today, and call out
> the extra steps for the `spi` backend once it's available. Beads
> tracks the SPI integration work via `bd ready`.

[gaps]: ../tech-specs/06-go-implementation-guide.md#known-gaps

## Prerequisites

On your workstation:

- Go 1.25+ (matches [`go.mod`](../../go.mod))
- An SSH-accessible Raspberry Pi (Zero 2 W, 3, 4 — anything that runs
  `linux/arm64` Raspberry Pi OS; `linux/arm` 32-bit is also supported,
  see the cross-compile section)
- The Waveshare 7.5" e-Paper V2 + E-Paper Driver HAT, wired and
  powered. If you've never confirmed the panel works, run the
  [Waveshare vendor test](../tech-specs/02-raspberry-pi-setup.md)
  first.

On the Pi:

- Raspberry Pi OS (64-bit recommended)
- SPI enabled via `sudo raspi-config → Interfacing Options → SPI → Yes`
  (then reboot). Verify with `ls /dev/spi*` — you should see
  `/dev/spidev0.0`.
- The `pi` user (or whatever user you'll run Inkwell as) is in the
  `gpio` and `spi` groups so it can open `/dev/gpiochip0` and
  `/dev/spidev0.0` without root:

  ```bash
  sudo usermod -aG gpio,spi pi
  # Log out and back in for group membership to take effect.
  ```

## 1. Cross-Compile the Binary

From the repo root on your workstation:

```bash
# 64-bit Raspberry Pi OS (Pi Zero 2 W, 3, 4, 5)
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -o build/inkwell-arm64 ./cmd/inkwell

# 32-bit Raspberry Pi OS (older Pis or 32-bit images)
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
  go build -o build/inkwell-armv7 ./cmd/inkwell
```

To compile the SPI backend in (once the real-hardware path is wired —
see [Known gaps][gaps]), add `-tags hardware`:

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -tags hardware -o build/inkwell-arm64 ./cmd/inkwell
```

`CGO_ENABLED=0` keeps the binary statically linked against musl-free
Go — no glibc / libgpiod surprises when you copy it to the Pi.

There's a `make build-pi` target that does the no-tags arm64 build
into the working directory; the explicit form above is more useful
when you want to manage build artifacts in `build/`.

## 2. Copy the Binary to the Pi

```bash
scp build/inkwell-arm64 pi@inkwell.local:~/inkwell
```

Substitute the Pi's hostname / IP for `inkwell.local`. If you're new
to a Pi and haven't set a custom hostname, `raspberrypi.local` is the
default.

## 3. Write a Config File

Inkwell reads its config from `inkwell.yaml` next to the binary (or
from a path passed as the first CLI argument). Start from the bundled
example:

```bash
scp inkwell.example.yaml pi@inkwell.local:~/inkwell.yaml
ssh pi@inkwell.local
```

On the Pi, edit `~/inkwell.yaml` to suit your setup. The example wires
up a full dashboard (date header, clock, separator, weekly calendar +
weather). The most important fields to review:

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
ssh pi@inkwell.local
./inkwell inkwell.yaml
```

If the config parses and the `preview` backend starts, you'll see the
process come up and serve on the port you configured. From your
workstation, point a browser at the Pi:

```text
http://inkwell.local:8080/
```

You should see the rendered dashboard, with the device-view radio
toggle for switching to the source (pre-dither) view. Every render
tick (default: 60 seconds) the SSE stream pushes a refresh and the
browser updates automatically.

Press Ctrl-C to stop the foreground process before moving on.

## 5. Install as a Systemd Service

For unattended operation, run Inkwell as a systemd unit. Create
`/etc/systemd/system/inkwell.service` on the Pi:

```ini
[Unit]
Description=Inkwell e-paper dashboard
After=network.target

[Service]
Type=simple
User=pi
Group=pi
WorkingDirectory=/home/pi
ExecStart=/home/pi/inkwell /home/pi/inkwell.yaml
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
journalctl -u inkwell -f      # Tail logs
sudo systemctl restart inkwell # Apply config edits
sudo systemctl stop inkwell    # Stop the service
```

## 6. Verify on Hardware

With the service running:

1. **Web preview reachable?** Open `http://<pi-host>:8080/` from a
   workstation on the same LAN. The page should render the dashboard
   at 2× scale.
2. **Logs healthy?** `journalctl -u inkwell -f` should show no
   repeated error messages, no crash loops.
3. **Updates flowing?** Watch the browser preview across at least one
   render-loop tick (default 60s) and confirm the SSE stream is
   pushing refreshes.

Once the SPI backend is enabled and you switch `backend: spi`, the
verification step extends to **looking at the panel itself** — the
first full refresh should land within a few seconds of service start,
and `journalctl -u inkwell -f` will surface any `EPD.Init` /
`EPD.Display` errors.

## 7. Updating

Re-running steps 1 and 2 (`go build` → `scp`) replaces the binary on
the Pi. Then:

```bash
sudo systemctl restart inkwell
```

Config-only changes don't need a fresh binary — edit
`~/inkwell.yaml` and restart the service.

## Troubleshooting

**`open inkwell.yaml: no such file or directory`** — Inkwell reads
`inkwell.yaml` from its working directory by default. Either copy a
config file next to the binary, or pass an absolute path as the first
CLI argument (the systemd unit above does this).

**Preview backend never serves traffic** — confirm the service user
can bind the port (no firewall, port not already in use). `ss -lntp`
on the Pi will show what's listening.

**`spi backend requires building with -tags hardware`** — your binary
was cross-compiled without the build tag. Re-run `go build` with
`-tags hardware` and re-deploy.

**`real hardware init not available in test-only builds`** — you
built with `-tags hardware` but Inkwell's real-hardware periph.io
bootstrap is still stubbed. See [Known gaps][gaps]; stay on the
`preview` backend until that work lands.

## Where to Go Next

- [`docs/guides/building-dashboards.md`](building-dashboards.md) —
  design custom screens, build new widgets, configure dashboards.
- [`docs/guides/hardware-grayscale.md`](hardware-grayscale.md) — what
  reads cleanly on the panel vs. what dithers to stipple.
- [`docs/tech-specs/`](../tech-specs/) — hardware overview, SPI
  command reference, Go architecture, and testing strategy.
