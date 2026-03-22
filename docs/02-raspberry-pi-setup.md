# Raspberry Pi Setup Guide

This documents the setup process for the Waveshare 7.5" e-Paper V2 on a
Raspberry Pi Zero 2 W running Raspberry Pi OS.

## 1. Enable SPI Interface

```bash
sudo raspi-config
```

Navigate to: **Interfacing Options -> SPI -> Yes**

Reboot and verify:

```bash
sudo reboot
ls /dev/spi*
# Should show: /dev/spidev0.0  /dev/spidev0.1
```

## 2. Install System Dependencies

```bash
sudo apt-get update
sudo apt-get install python3-pip python3-pil python3-numpy
```

## 3. Install Python Libraries

```bash
sudo pip3 install spidev
sudo apt install python3-gpiozero
```

### Optional: BCM2835 C Library

Required only if using the C examples or if the Python driver falls back to
native libraries:

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

## 5. Run the Test

```bash
python3 epd_7in5_V2_test.py
```

This runs through several demo screens: bitmap display, shape drawing, text
rendering, and partial refresh clock updates.

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

## Key Python Dependencies

<!-- markdownlint-disable MD013 -->
| Package | Purpose |
|---------|---------|
| `spidev` | SPI bus access from userspace |
| `gpiozero` | GPIO pin control (RST, DC, BUSY, PWR) |
| `Pillow` (PIL) | Image creation, drawing, font rendering |
| `numpy` | Used by some advanced examples |
<!-- markdownlint-enable MD013 -->
