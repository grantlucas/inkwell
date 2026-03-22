# Hardware Overview

## Display: Waveshare 7.5inch e-Paper V2

<!-- markdownlint-disable MD013 -->
| Parameter | Value |
|-----------|-------|
| Resolution | 800 x 480 pixels |
| Colors | Black and White (+ 4 grayscale levels) |
| Full refresh time | ~4 seconds |
| Fast refresh time | ~1.5 seconds |
| Partial refresh time | ~0.4 seconds |
| Interface | SPI |
| Operating voltage | 3.3V / 5V |
| Operating temperature | 0-50 C |
| Display size | 7.5 inches diagonal |
<!-- markdownlint-enable MD013 -->

## Controller: Raspberry Pi Zero 2 W

The Pi Zero 2 W runs a quad-core ARM Cortex-A53 at 1GHz with 512MB RAM.
It exposes a 40-pin GPIO header that the HAT connects to directly.

## Connection: E-Paper Driver HAT

The E-Paper Driver HAT (Rev2.3) sits between the Pi and the display. It is a
universal driver board for Waveshare SPI e-Paper panels.

Key features:

- 40-pin GPIO header (plugs directly onto Pi)
- Onboard voltage converter (supports 3.3V and 5V MCUs)
- GH1.25 9-pin connector to the display (Rev2.3+)
- Dedicated PWR pin for power management (Rev2.3+)
- Board dimensions: 65mm x 30.2mm

### Resistor Selection

The HAT has a resistor selection for display compatibility:

- **0.47 ohm (B):** For newer displays including the 7.5" V2 (this is what we
  use)
- **3 ohm (A):** For older display variants

## GPIO Pin Mapping

<!-- markdownlint-disable MD013 -->
| Signal | Function | BCM GPIO | Board Pin | Description |
|--------|----------|----------|-----------|-------------|
| VCC | Power | - | 3.3V | Power supply |
| GND | Ground | - | GND | Ground |
| DIN | MOSI | 10 | 19 | SPI data in |
| CLK | SCLK | 11 | 23 | SPI clock |
| CS | CE0 | 8 | 24 | SPI chip select |
| DC | Data/Command | 25 | 22 | Low = command, High = data |
| RST | Reset | 17 | 11 | Hardware reset (active low) |
| BUSY | Busy status | 24 | 18 | Low = busy, High = idle |
| PWR | Power control | 18 | 12 | Display power on/off |
<!-- markdownlint-enable MD013 -->

### SPI Configuration

- **Bus:** `/dev/spidev0.0`
- **Mode:** 0 (CPOL=0, CPHA=0)
- **Speed:** 4 MHz (default in Python driver)
- **Bit order:** MSB first
- **Word size:** 8 bits

## Important Operational Notes

1. **Partial refresh safety:** After several partial refreshes, a full refresh
   is required. Failure to do so can cause permanent image retention damage.
2. **Sleep mode:** Always put the display to sleep or power it down when not
   actively updating. Leaving the display powered on with static high voltage
   can damage the panel.
3. **Refresh intervals:** Maintain a minimum of 180 seconds between full
   refreshes. Refresh at least once every 24 hours during regular use.
4. **Temperature:** Do not operate outside 0-50 C range.

## Reference Links

- [7.5inch e-Paper HAT Manual](https://www.waveshare.com/wiki/7.5inch_e-Paper_HAT_Manual)
- [E-Paper Driver HAT Wiki](https://www.waveshare.com/wiki/E-Paper_Driver_HAT)
