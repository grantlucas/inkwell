# Optional: Waveshare Hardware Bringup (Vendor Test)

> **This is not an Inkwell install guide.** It walks through enabling SPI
> on a Raspberry Pi and running the Waveshare-provided Python test
> script (`epd_7in5_V2_test.py`) to confirm the panel and HAT are wired
> correctly. Inkwell is a Go application and does not use any of the
> Python tools below at runtime.
>
> For installing and running Inkwell on a Pi, see
> [`docs/guides/installation.md`](../guides/installation.md).

Use this checklist when:

- You've just received the panel and HAT and want to confirm they work
  before introducing Inkwell on top.
- You suspect a hardware/wiring fault and want a vendor-provided
  reference that bypasses Inkwell entirely.

If your panel already lights up with Inkwell's preview backend or the
`spi` backend (once available), you can skip this page.

## 1. Enable SPI

```bash
sudo raspi-config
```

Navigate to: **Interfacing Options → SPI → Yes**

Reboot and verify the SPI device node exists:

```bash
sudo reboot
ls /dev/spi*
# Should show: /dev/spidev0.0  /dev/spidev0.1
```

Inkwell itself needs `/dev/spidev0.0` to drive the panel. Enabling SPI
is the only step on this page that is required for Inkwell — everything
below is only needed if you want to run the Python vendor test.

## 2. Install System Dependencies (vendor test only)

```bash
sudo apt-get update
sudo apt-get install python3-pip python3-pil python3-numpy
```

## 3. Install Python Libraries (vendor test only)

```bash
sudo pip3 install spidev
sudo apt install python3-gpiozero
```

### Optional: BCM2835 C Library

Required only if using the C examples or if the Python driver falls
back to native libraries:

```bash
wget http://www.airspayce.com/mikem/bcm2835/bcm2835-1.71.tar.gz
tar zxvf bcm2835-1.71.tar.gz
cd bcm2835-1.71/
sudo ./configure && sudo make && sudo make check && sudo make install
```

## 4. Clone the Waveshare Examples

```bash
git clone https://github.com/waveshare/e-Paper.git
cd e-Paper/RaspberryPi_JetsonNano/python/examples/
```

## 5. Run the Vendor Test

```bash
python3 epd_7in5_V2_test.py
```

This runs through several demo screens: bitmap display, shape drawing,
text rendering, and partial refresh clock updates. If you see all of
them, your panel + HAT + SPI wiring are good and you can proceed to
[installing Inkwell](../guides/installation.md).

## 6. File Structure of the Waveshare Python SDK

```text
e-Paper/RaspberryPi_JetsonNano/python/
├── examples/
│   ├── epd_7in5_V2_test.py          # Main test for our display
│   └── ...                           # ~65 other display tests
├── lib/
│   └── waveshare_epd/
│       ├── __init__.py
│       ├── epdconfig.py              # Hardware abstraction layer
│       ├── epd7in5_V2.py             # Our display driver
│       └── ...                       # Drivers for other displays
└── pic/
    ├── Font.ttc                      # TrueType font
    ├── 7in5_V2.bmp                   # Test bitmap
    └── ...                           # Other test images
```

## Key Python Dependencies (vendor test only)

<!-- markdownlint-disable MD013 -->
| Package | Purpose |
|---------|---------|
| `spidev` | SPI bus access from userspace |
| `gpiozero` | GPIO pin control (RST, DC, BUSY, PWR) |
| `Pillow` (PIL) | Image creation, drawing, font rendering |
| `numpy` | Used by some advanced examples |
<!-- markdownlint-enable MD013 -->

## What Inkwell Actually Needs at Runtime

For reference, Inkwell needs only:

- `/dev/spidev0.0` (SPI enabled via `raspi-config`)
- Access to the GPIO chip device (typically the `gpio` group is enough,
  no extra packages required)
- A `linux/arm64` (or `linux/arm` for 32-bit OS) build of the `inkwell`
  binary, optionally compiled with `-tags hardware` to include the SPI
  backend

None of the Python packages above are used by Inkwell at runtime.
